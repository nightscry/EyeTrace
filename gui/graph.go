package gui

import (
	"image/color"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"eyetracker/src"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	colEdge   = color.NRGBA{0x8b, 0x5c, 0xf6, 0x99}
	colNode   = color.NRGBA{0x2a, 0x20, 0x40, 0xff}
	colNodeHi = color.NRGBA{0xa8, 0x55, 0xf7, 0xff}
)

// force simulation parameters, tuned by eye
var (
	simRepulsion = float32(9000)
	simSpring    = float32(0.01)
	simRestLen   = float32(150)
	simCenter    = float32(0.01)
	simDamping   = float32(0.85)
	simMaxV      = float32(30)
)

type NodeInfo struct {
	Address  string
	IsTarget bool
	Balance  float64 // only set for the target
	Amount   float64
	Count    int
	Asset    string
}

func short(a string) string {
	if len(a) <= 13 {
		return a
	}
	return a[:6] + "…" + a[len(a)-4:]
}

type counterparty struct {
	addr   string
	amount float64
	count  int
}

func aggregate(target string, transfers []src.Transfer, maxNodes int) []counterparty {
	if maxNodes <= 0 {
		maxNodes = 12
	}
	m := map[string]*counterparty{}
	for _, t := range transfers {
		var cp string
		switch {
		// EqualFold is required: ETH addresses come back lowercase from the API
		// while the target is mixed-case (checksummed).
		case strings.EqualFold(t.From, target) && !strings.EqualFold(t.To, target):
			cp = t.To
		case strings.EqualFold(t.To, target) && !strings.EqualFold(t.From, target):
			cp = t.From
		default:
			continue
		}
		// drops coinbase and addressless inputs, so no ghost nodes
		if cp == "" {
			continue
		}
		e := m[cp]
		if e == nil {
			e = &counterparty{addr: cp}
			m[cp] = e
		}
		e.amount += t.Amount
		e.count++
	}
	list := make([]counterparty, 0, len(m))
	for _, v := range m {
		list = append(list, *v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].amount > list[j].amount })
	if len(list) > maxNodes {
		list = list[:maxNodes]
	}
	return list
}

type gnode struct {
	addr     string
	label    string
	balance  float64
	amount   float64
	count    int
	x, y     float32
	vx, vy   float32
	isTarget bool
}

type graphWidget struct {
	widget.BaseWidget
	nodes    []*gnode
	edges    [][2]int
	size     fyne.Size
	placed   bool
	dragging int
	selected int // -1 = nothing selected
	asset    string
	anim     *fyne.Animation
	OnSelect func(NodeInfo)
}

var (
	_ fyne.Draggable = (*graphWidget)(nil)
	_ fyne.Tappable  = (*graphWidget)(nil)
)

// One hop only: target at the centre, counterparties around it.
func newGraphWidget(target string, asset string, balance float64, transfers []src.Transfer, maxNodes int) *graphWidget {
	parties := aggregate(target, transfers, maxNodes)

	g := &graphWidget{dragging: -1, selected: -1, asset: asset}
	g.nodes = append(g.nodes, &gnode{addr: target, label: short(target), balance: balance, isTarget: true})
	for i, p := range parties {
		g.nodes = append(g.nodes, &gnode{addr: p.addr, label: short(p.addr), amount: p.amount, count: p.count})
		g.edges = append(g.edges, [2]int{0, i + 1})
	}
	g.ExtendBaseWidget(g)
	return g
}

func (g *graphWidget) infoOf(i int) NodeInfo {
	n := g.nodes[i]
	return NodeInfo{Address: n.addr, IsTarget: n.isTarget, Balance: n.balance, Amount: n.amount, Count: n.count, Asset: g.asset}
}

func (g *graphWidget) placeInitial() {
	cx, cy := g.size.Width/2, g.size.Height/2
	ring := float32(math.Min(float64(g.size.Width), float64(g.size.Height))) * 0.3
	n := len(g.nodes)
	for i, node := range g.nodes {
		if node.isTarget {
			node.x, node.y = cx, cy
			continue
		}
		ang := 2 * math.Pi * float64(i) / float64(n)
		jit := func() float32 { return (rand.Float32() - 0.5) * 20 }
		node.x = cx + ring*float32(math.Cos(ang)) + jit()
		node.y = cy + ring*float32(math.Sin(ang)) + jit()
	}
	g.placed = true
}

func (g *graphWidget) step() {
	w, h := g.size.Width, g.size.Height
	if w < 1 || h < 1 {
		return
	}
	if !g.placed {
		g.placeInitial()
	}
	cx, cy := w/2, h/2

	// repulsion between every pair of nodes
	for i := 0; i < len(g.nodes); i++ {
		for j := i + 1; j < len(g.nodes); j++ {
			a, b := g.nodes[i], g.nodes[j]
			dx, dy := a.x-b.x, a.y-b.y
			d2 := dx*dx + dy*dy
			if d2 < 1 {
				d2 = 1
			}
			d := float32(math.Sqrt(float64(d2)))
			f := simRepulsion / d2
			fx, fy := f*dx/d, f*dy/d
			a.vx += fx
			a.vy += fy
			b.vx -= fx
			b.vy -= fy
		}
	}
	// edges act as springs pulling back to simRestLen
	for _, e := range g.edges {
		a, b := g.nodes[e[0]], g.nodes[e[1]]
		dx, dy := b.x-a.x, b.y-a.y
		d := float32(math.Sqrt(float64(dx*dx+dy*dy))) + 0.01
		f := simSpring * (d - simRestLen)
		fx, fy := f*dx/d, f*dy/d
		a.vx += fx
		a.vy += fy
		b.vx -= fx
		b.vy -= fy
	}
	for i, node := range g.nodes {
		node.vx += (cx - node.x) * simCenter
		node.vy += (cy - node.y) * simCenter
		node.vx *= simDamping
		node.vy *= simDamping
		node.vx = clamp(node.vx, -simMaxV, simMaxV)
		node.vy = clamp(node.vy, -simMaxV, simMaxV)

		if i == g.dragging {
			continue
		}
		// the target stays anchored to the centre
		if node.isTarget {
			node.x, node.y = cx, cy
			continue
		}
		node.x += node.vx
		node.y += node.vy
	}
}

func clamp(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (g *graphWidget) startSim() {
	g.anim = fyne.NewAnimation(time.Second, func(float32) {
		g.step()
		g.Refresh()
	})
	g.anim.RepeatCount = fyne.AnimationRepeatForever
	g.anim.Start()
}

func (g *graphWidget) Tapped(e *fyne.PointEvent) {
	i := g.pick(e.Position)
	if i < 0 {
		return
	}
	g.selected = i
	if g.OnSelect != nil {
		g.OnSelect(g.infoOf(i))
	}
	g.Refresh()
}

func (g *graphWidget) Dragged(e *fyne.DragEvent) {
	if g.dragging < 0 {
		g.dragging = g.pick(e.Position)
	}
	if g.dragging >= 0 {
		n := g.nodes[g.dragging]
		n.x, n.y = e.Position.X, e.Position.Y
		n.vx, n.vy = 0, 0
	}
}

func (g *graphWidget) DragEnd() { g.dragging = -1 }

// nearest node within 22px, or -1
func (g *graphWidget) pick(pos fyne.Position) int {
	best, bestD := -1, float32(22)
	for i, n := range g.nodes {
		dx, dy := n.x-pos.X, n.y-pos.Y
		d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

func (g *graphWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &graphRenderer{w: g}
	for range g.edges {
		line := canvas.NewLine(colEdge)
		line.StrokeWidth = 1.5
		r.edges = append(r.edges, line)
	}
	for _, n := range g.nodes {
		radius := float32(13)
		fill, stroke := colNode, colNodeHi
		if n.isTarget {
			radius, fill, stroke = 20, colNodeHi, colText
		}
		c := canvas.NewCircle(fill)
		c.StrokeColor = stroke
		c.StrokeWidth = 2
		r.circles = append(r.circles, c)
		r.radii = append(r.radii, radius)

		lbl := canvas.NewText(n.label, colTextDim)
		lbl.TextSize = 11
		lbl.Alignment = fyne.TextAlignCenter
		lbl.TextStyle = fyne.TextStyle{Monospace: true}
		if n.isTarget {
			lbl.Color = colText
			lbl.TextStyle.Bold = true
		}
		r.labels = append(r.labels, lbl)
	}
	// draw order: edges, then circles, then labels on top
	for _, e := range r.edges {
		r.objects = append(r.objects, e)
	}
	for _, c := range r.circles {
		r.objects = append(r.objects, c)
	}
	for _, l := range r.labels {
		r.objects = append(r.objects, l)
	}
	g.startSim()
	return r
}

type graphRenderer struct {
	w       *graphWidget
	edges   []*canvas.Line
	circles []*canvas.Circle
	radii   []float32
	labels  []*canvas.Text
	objects []fyne.CanvasObject
}

func (r *graphRenderer) relayout() {
	g := r.w
	for i, e := range g.edges {
		a, b := g.nodes[e[0]], g.nodes[e[1]]
		r.edges[i].Position1 = fyne.NewPos(a.x, a.y)
		r.edges[i].Position2 = fyne.NewPos(b.x, b.y)
	}
	for i, n := range g.nodes {
		rad := r.radii[i]
		c := r.circles[i]
		c.Resize(fyne.NewSize(rad*2, rad*2))
		c.Move(fyne.NewPos(n.x-rad, n.y-rad))

		if i == g.selected {
			c.StrokeColor = colText
			c.StrokeWidth = 4
		} else {
			c.StrokeWidth = 2
			if n.isTarget {
				c.StrokeColor = colText
			} else {
				c.StrokeColor = colNodeHi
			}
		}

		const lw = 150
		r.labels[i].Resize(fyne.NewSize(lw, 16))
		r.labels[i].Move(fyne.NewPos(n.x-lw/2, n.y+rad+2))
	}
}

func (r *graphRenderer) Layout(size fyne.Size) {
	r.w.size = size
	r.relayout()
}
func (r *graphRenderer) MinSize() fyne.Size          { return fyne.NewSize(400, 300) }
func (r *graphRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *graphRenderer) Refresh()                     { r.relayout() }
func (r *graphRenderer) Destroy() {
	if r.w.anim != nil {
		r.w.anim.Stop()
	}
}

// GraphView is the graph area, watermark included.
type GraphView struct {
	Root     *fyne.Container
	host     *fyne.Container
	OnSelect func(NodeInfo) // set by the dashboard, passed on to each new graph
}

func NewGraphView() *GraphView {
	bg := canvas.NewRectangle(colBg2)

	eye := canvas.NewImageFromResource(eyeResource)
	eye.FillMode = canvas.ImageFillContain
	eye.Translucency = 0.9
	eye.SetMinSize(fyne.NewSize(360, 300))

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = colBorder
	border.StrokeWidth = 1.5

	host := container.NewStack()
	gv := &GraphView{
		Root: container.NewStack(bg, container.NewCenter(eye), border, host),
		host: host,
	}
	gv.Message("enter a wallet and press Analyze")
	return gv
}

func (gv *GraphView) Message(msg string) {
	t := canvas.NewText(msg, colTextDim)
	t.Alignment = fyne.TextAlignCenter
	t.TextStyle = fyne.TextStyle{Italic: true}
	gv.host.Objects = []fyne.CanvasObject{container.NewCenter(t)}
	gv.host.Refresh()
}

func (gv *GraphView) Render(target string, asset string, balance float64, transfers []src.Transfer, maxNodes int) {
	g := newGraphWidget(target, asset, balance, transfers, maxNodes)
	g.OnSelect = gv.OnSelect
	gv.host.Objects = []fyne.CanvasObject{g}
	gv.host.Refresh()

	// preselect the target so the panel shows the analysed wallet right away
	g.selected = 0
	if g.OnSelect != nil {
		g.OnSelect(g.infoOf(0))
	}
}
