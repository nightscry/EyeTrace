package gui

import (
	"fmt"
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
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

var (
	colEdge   = color.NRGBA{0x8b, 0x5c, 0xf6, 0x99}
	colOut    = color.NRGBA{0xf4, 0x72, 0xb6, 0xcc} // analysed wallet -> counterparty
	colIn     = color.NRGBA{0x34, 0xd3, 0x99, 0xcc} // counterparty -> analysed wallet
	colNode   = color.NRGBA{0x2a, 0x20, 0x40, 0xff}
	colNodeHi = color.NRGBA{0xa8, 0x55, 0xf7, 0xff}
	colNodeEx = color.NRGBA{0x5b, 0x3d, 0x9a, 0xff} // expanded wallet

	// entity categories
	colExchange = color.NRGBA{0xf5, 0xb7, 0x00, 0xff}
	colMixer    = color.NRGBA{0xef, 0x44, 0x44, 0xff}
	colContract = color.NRGBA{0x38, 0xbd, 0xf8, 0xff}
	colBurn     = color.NRGBA{0x6b, 0x72, 0x80, 0xff}
	colOther    = color.NRGBA{0x2d, 0xd4, 0xbf, 0xff}
	colService  = color.NRGBA{0xfb, 0x92, 0x3c, 0xff} // heuristic: exchange-like volume
)

func entityColor(cat string) color.NRGBA {
	switch cat {
	case src.CatExchange:
		return colExchange
	case src.CatMixer:
		return colMixer
	case src.CatContract, src.CatToken:
		return colContract
	case src.CatBurn:
		return colBurn
	}
	return colOther
}

// force simulation parameters, tuned by eye
var (
	simRepulsion = float32(9000)
	simSpring    = float32(0.01)
	simRestLen   = float32(150)
	simHubLen    = float32(420)   // rest length between two expanded wallets
	simCenter    = float32(0.002) // weak: only keeps stray nodes from drifting off
	simDamping   = float32(0.85)
	simMaxV      = float32(30)
)

const (
	minZoom = 0.2
	maxZoom = 5
	// frames the view eases towards fitting the whole graph after it changes
	fitFrames = 90
)

// NodeInfo is what the side panel shows for the selected node.
type NodeInfo struct {
	Address     string
	Chain       string
	Testnet     bool
	IsTarget    bool
	Expanded    bool
	Recognized  bool // target or expanded: detector, balance and profile below are set
	Subtype     string
	Balance     float64
	HasBalance  bool
	BalanceErr  string
	Identifying bool   // a lookup of this wallet is running
	BalanceUnit string // native coin of the chain, the balance is always in it
	TxCount     int    // 0 = unknown
	OnChainName string // what the explorer knows, e.g. a verified contract name
	IsContract  bool
	Profile     src.Profile
	Asset       string  // asset the graph is filtered on, In/Out are in it
	In, Out     float64 // flows into / out of this node, over the edges drawn
	Count       int     // transfers on those edges
	Links       int
	Entity      *src.Entity
	Risk        []string // only set for the target: mixers reachable in the graph
}

func short(a string) string {
	if len(a) <= 13 {
		return a
	}
	return a[:6] + "…" + a[len(a)-4:]
}

// AssetCount is one entry of the asset selector.
type AssetCount struct {
	Asset string
	Count int
}

// Assets found in transfers, most used first. Unverified tokens always go last,
// so the default pick is never a spam token while a real asset is available.
// On a tie the chain's native coin wins.
func assetsOf(transfers []src.Transfer, native string) []AssetCount {
	m := map[string]int{}
	for _, t := range transfers {
		m[t.Asset]++
	}
	list := make([]AssetCount, 0, len(m))
	for a, c := range m {
		list = append(list, AssetCount{a, c})
	}
	sort.Slice(list, func(i, j int) bool {
		ui, uj := isUnverified(list[i].Asset), isUnverified(list[j].Asset)
		if ui != uj {
			return uj
		}
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		if (list[i].Asset == native) != (list[j].Asset == native) {
			return list[i].Asset == native
		}
		return list[i].Asset < list[j].Asset
	})
	return list
}

func isUnverified(asset string) bool { return strings.HasSuffix(asset, src.UnverifiedMark) }

type counterparty struct {
	addr  string
	sent  float64 // center -> counterparty
	recv  float64 // counterparty -> center
	count int
}

// Only transfers of one asset are summed: adding TRX to USDT would be meaningless.
// Counterparties are ranked by total volume, both directions.
func aggregate(center string, asset string, transfers []src.Transfer, maxNodes int) []counterparty {
	if maxNodes <= 0 {
		maxNodes = 12
	}
	m := map[string]*counterparty{}
	for _, t := range transfers {
		if t.Asset != asset {
			continue
		}
		var cp string
		outgoing := false
		switch {
		// EqualFold is required: ETH addresses come back lowercase from the API
		// while the target is mixed-case (checksummed).
		case strings.EqualFold(t.From, center) && !strings.EqualFold(t.To, center):
			cp, outgoing = t.To, true
		case strings.EqualFold(t.To, center) && !strings.EqualFold(t.From, center):
			cp = t.From
		default:
			continue
		}
		// drops coinbase and addressless inputs, so no ghost nodes
		if cp == "" {
			continue
		}
		key := strings.ToLower(cp)
		e := m[key]
		if e == nil {
			e = &counterparty{addr: cp}
			m[key] = e
		}
		if outgoing {
			e.sent += t.Amount
		} else {
			e.recv += t.Amount
		}
		e.count++
	}
	list := make([]counterparty, 0, len(m))
	for _, v := range m {
		list = append(list, *v)
	}
	sort.Slice(list, func(i, j int) bool {
		vi, vj := list[i].sent+list[i].recv, list[j].sent+list[j].recv
		if vi != vj {
			return vi > vj
		}
		return list[i].addr < list[j].addr
	})
	if len(list) > maxNodes {
		list = list[:maxNodes]
	}
	return list
}

type gnode struct {
	addr     string
	label    string
	x, y     float32 // world coordinates, target at the origin
	vx, vy   float32
	isTarget bool
	expanded bool
	entity   *src.Entity
	service  bool // heuristic: volume typical of an exchange or service
	contract bool
	named    bool    // label is a name (entity or on-chain), not an address
	keyLabel bool    // among the top counterparties of a wallet: labelled by default
	radius   float32 // leaves only: grows with volume
}

type gedge struct {
	a, b  int
	ab    float64 // a -> b
	ba    float64 // b -> a
	count int
	rest  float32 // spring rest length
}

// Every expanded wallet is a centre: its top counterparties are linked to it.
// An edge between two expanded wallets shows up in both histories, so it is
// built once, from whichever side comes first.
// Recognition: built-in label first, then what the explorer said about the wallet
// (analysed, expanded or looked up on click), then the activity heuristic.
func decorate(n *gnode, inv *src.Investigation) {
	n.label, n.named, n.entity = short(n.addr), false, nil
	n.expanded = inv.IsExpanded(n.addr)
	if info, ok := inv.WalletInfo(n.addr); ok {
		n.contract = info.IsContract
		if info.Name != "" {
			n.label, n.named = info.Name, true
		}
		n.service = src.ProfileOf(info, inv.HistoryOf(n.addr)).ServiceLike
	}
	if e, ok := src.LookupEntity(inv.Chain, n.addr); ok {
		n.entity = &e
		n.label, n.named = e.Name, true
	}
}

func buildGraph(inv *src.Investigation, asset string, maxNodes int) ([]*gnode, []*gedge) {
	var nodes []*gnode
	var edges []*gedge
	index := map[string]int{}
	edgeIndex := map[[2]int]bool{}

	ensure := func(addr string) int {
		key := strings.ToLower(addr)
		if i, ok := index[key]; ok {
			return i
		}
		n := &gnode{addr: addr}
		n.isTarget = len(nodes) == 0
		decorate(n, inv)
		nodes = append(nodes, n)
		index[key] = len(nodes) - 1
		return len(nodes) - 1
	}

	for _, center := range inv.Expanded {
		ci := ensure(center)
		for rank, p := range aggregate(center, asset, inv.History[center], maxNodes) {
			pi := ensure(p.addr)
			// the biggest counterparties keep a visible label, the rest show on hover
			if rank < 3 {
				nodes[pi].keyLabel = true
			}
			k := [2]int{min(ci, pi), max(ci, pi)}
			if edgeIndex[k] {
				continue
			}
			edgeIndex[k] = true
			edges = append(edges, &gedge{a: ci, b: pi, ab: p.sent, ba: p.recv, count: p.count, rest: simRestLen})
		}
	}
	// two expanded wallets are two stars: a long link keeps their counterparties apart
	vol := make([]float64, len(nodes))
	for _, e := range edges {
		if nodes[e.a].expanded && nodes[e.b].expanded {
			e.rest = simHubLen
		}
		vol[e.a] += e.ab + e.ba
		vol[e.b] += e.ab + e.ba
	}
	// leaf size follows volume on a log scale, so the main counterparties stand out
	maxLeaf := 0.0
	for i, n := range nodes {
		if !n.expanded {
			maxLeaf = math.Max(maxLeaf, vol[i])
		}
	}
	for i, n := range nodes {
		n.radius = 7
		if maxLeaf > 0 {
			n.radius += 6 * float32(math.Log1p(vol[i])/math.Log1p(maxLeaf))
		}
	}
	return nodes, edges
}

type graphWidget struct {
	widget.BaseWidget
	inv      *src.Investigation
	nodes    []*gnode
	edges    []*gedge
	size     fyne.Size
	placed   bool
	prev     map[string]fyne.Position // world positions kept from the graph this one replaces
	prevExp  map[string]bool          // wallets already expanded in that graph
	fitting  int                      // frames left of easing the view onto the whole graph
	dragging int
	panning  bool
	selected int // -1 = nothing selected
	hover    int // node under the pointer, -1 = none
	asset    string
	scale    float32
	off      fyne.Position // pan, in screen pixels
	anim     *fyne.Animation
	OnSelect func(NodeInfo)
	OnExpand func(addr string)
}

var (
	_ fyne.Draggable      = (*graphWidget)(nil)
	_ fyne.Tappable       = (*graphWidget)(nil)
	_ fyne.DoubleTappable = (*graphWidget)(nil)
	_ fyne.Scrollable     = (*graphWidget)(nil)
	_ desktop.Hoverable   = (*graphWidget)(nil)
)

// hovering a node reveals its label, so the graph can stay quiet by default
func (g *graphWidget) MouseIn(e *desktop.MouseEvent)    { g.setHover(g.pick(e.Position)) }
func (g *graphWidget) MouseMoved(e *desktop.MouseEvent) { g.setHover(g.pick(e.Position)) }
func (g *graphWidget) MouseOut()                        { g.setHover(-1) }

func (g *graphWidget) setHover(i int) {
	if i != g.hover {
		g.hover = i
		g.Refresh()
	}
}

func newGraphWidget(inv *src.Investigation, asset string, maxNodes int) *graphWidget {
	g := &graphWidget{inv: inv, dragging: -1, selected: -1, hover: -1, asset: asset, scale: 1}
	g.nodes, g.edges = buildGraph(inv, asset, maxNodes)
	g.ExtendBaseWidget(g)
	return g
}

func (g *graphWidget) indexOf(addr string) int {
	for i, n := range g.nodes {
		if strings.EqualFold(n.addr, addr) {
			return i
		}
	}
	return -1
}

// hops from the target over the drawn edges, -1 when unreachable
func (g *graphWidget) distances() []int {
	adj := make([][]int, len(g.nodes))
	for _, e := range g.edges {
		adj[e.a] = append(adj[e.a], e.b)
		adj[e.b] = append(adj[e.b], e.a)
	}
	dist := make([]int, len(g.nodes))
	for i := range dist {
		dist[i] = -1
	}
	if len(dist) == 0 {
		return dist
	}
	dist[0] = 0
	queue := []int{0}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range adj[cur] {
			if dist[nb] < 0 {
				dist[nb] = dist[cur] + 1
				queue = append(queue, nb)
			}
		}
	}
	return dist
}

func (g *graphWidget) infoOf(i int) NodeInfo {
	n := g.nodes[i]
	info := NodeInfo{
		Address: n.addr, Chain: g.inv.Chain, Testnet: g.inv.Testnet,
		IsTarget: n.isTarget, Expanded: n.expanded,
		BalanceUnit: src.NativeAsset(g.inv.Chain), Asset: g.asset, Entity: n.entity,
	}
	if w, ok := g.inv.WalletInfo(n.addr); ok {
		info.Recognized = true
		info.Subtype, info.Balance, info.HasBalance, info.TxCount = w.Subtype, w.Balance, w.HasBalance, w.TxCount
		info.BalanceErr = w.BalanceErr
		info.OnChainName, info.IsContract = w.Name, w.IsContract
		info.Profile = src.ProfileOf(w, g.inv.HistoryOf(n.addr))
	}
	for _, e := range g.edges {
		switch i {
		case e.a:
			info.Out += e.ab
			info.In += e.ba
		case e.b:
			info.Out += e.ba
			info.In += e.ab
		default:
			continue
		}
		info.Count += e.count
		info.Links++
	}
	if n.isTarget {
		dist := g.distances()
		for j, m := range g.nodes {
			if m.entity == nil || !m.entity.Risky() || dist[j] < 0 {
				continue
			}
			if dist[j] == 1 {
				info.Risk = append(info.Risk, "Direct: "+m.entity.Name)
			} else {
				info.Risk = append(info.Risk, fmt.Sprintf("%d hops: %s", dist[j], m.entity.Name))
			}
		}
	}
	return info
}

// screen <-> world
func (g *graphWidget) toScreen(x, y float32) fyne.Position {
	return fyne.NewPos(g.size.Width/2+x*g.scale+g.off.X, g.size.Height/2+y*g.scale+g.off.Y)
}

func (g *graphWidget) toWorld(p fyne.Position) (float32, float32) {
	return (p.X - g.size.Width/2 - g.off.X) / g.scale, (p.Y - g.size.Height/2 - g.off.Y) / g.scale
}

// Nodes kept from the previous graph stay where they were, so expanding a wallet
// does not reshuffle the whole picture. A freshly expanded wallet is pushed out to
// hub distance, and its new counterparties fan out on the far side, away from the
// target, so the new star does not land on top of the old one.
func (g *graphWidget) placeInitial() {
	ring := float32(math.Min(float64(g.size.Width), float64(g.size.Height))) * 0.3
	jit := func() float32 { return (rand.Float32() - 0.5) * 20 }
	placed := make([]bool, len(g.nodes))

	for i, node := range g.nodes {
		if node.isTarget {
			node.x, node.y = 0, 0
			placed[i] = true
			continue
		}
		key := strings.ToLower(node.addr)
		p, ok := g.prev[key]
		if !ok {
			continue
		}
		node.x, node.y = p.X, p.Y
		placed[i] = true
		if node.expanded && !g.prevExp[key] {
			d := float32(math.Hypot(float64(p.X), float64(p.Y)))
			if d < 1 {
				ang := rand.Float64() * 2 * math.Pi
				node.x, node.y, d = float32(math.Cos(ang)), float32(math.Sin(ang)), 1
			}
			node.x, node.y = node.x/d*simHubLen, node.y/d*simHubLen
		}
	}
	for _, e := range g.edges {
		for _, pair := range [][2]int{{e.a, e.b}, {e.b, e.a}} {
			from, to := pair[0], pair[1]
			if !placed[from] || placed[to] {
				continue
			}
			f := g.nodes[from]
			ang := rand.Float64() * 2 * math.Pi
			if !f.isTarget {
				out := math.Atan2(float64(f.y), float64(f.x)) // pointing away from the target
				ang = out + (rand.Float64()-0.5)*math.Pi
			}
			g.nodes[to].x = f.x + e.rest*float32(math.Cos(ang)) + jit()
			g.nodes[to].y = f.y + e.rest*float32(math.Sin(ang)) + jit()
			placed[to] = true
		}
	}
	n := len(g.nodes)
	for i, node := range g.nodes {
		if placed[i] {
			continue
		}
		ang := 2 * math.Pi * float64(i) / float64(n)
		node.x = ring*float32(math.Cos(ang)) + jit()
		node.y = ring*float32(math.Sin(ang)) + jit()
	}
	g.placed = true
}

func (g *graphWidget) step() {
	if g.size.Width < 1 || g.size.Height < 1 {
		return
	}
	if !g.placed {
		g.placeInitial()
	}

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
	// edges act as springs pulling back to their rest length
	for _, e := range g.edges {
		a, b := g.nodes[e.a], g.nodes[e.b]
		dx, dy := b.x-a.x, b.y-a.y
		d := float32(math.Sqrt(float64(dx*dx+dy*dy))) + 0.01
		f := simSpring * (d - e.rest)
		fx, fy := f*dx/d, f*dy/d
		a.vx += fx
		a.vy += fy
		b.vx -= fx
		b.vy -= fy
	}
	for i, node := range g.nodes {
		node.vx -= node.x * simCenter
		node.vy -= node.y * simCenter
		node.vx *= simDamping
		node.vy *= simDamping
		node.vx = clamp(node.vx, -simMaxV, simMaxV)
		node.vy = clamp(node.vy, -simMaxV, simMaxV)

		if i == g.dragging {
			continue
		}
		// the target stays anchored to the origin
		if node.isTarget {
			node.x, node.y = 0, 0
			continue
		}
		node.x += node.vx
		node.y += node.vy
	}

	if g.fitting > 0 {
		g.fitting--
		g.easeToFit(0.12)
	}
}

// Moves zoom and pan a step towards showing every node, labels included.
func (g *graphWidget) easeToFit(k float32) {
	if len(g.nodes) == 0 {
		return
	}
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := -minX, -minY
	for _, n := range g.nodes {
		minX, maxX = min(minX, n.x), max(maxX, n.x)
		minY, maxY = min(minY, n.y), max(maxY, n.y)
	}
	const margin = 90 // node radius + label
	w, h := maxX-minX+2*margin, maxY-minY+2*margin
	target := clamp(min(g.size.Width/w, g.size.Height/h), minZoom, 1.2)
	cx, cy := (minX+maxX)/2, (minY+maxY)/2

	g.scale += (target - g.scale) * k
	g.off.X += (-cx*g.scale - g.off.X) * k
	g.off.Y += (-cy*g.scale - g.off.Y) * k
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

func (g *graphWidget) selectNode(i int) {
	g.selected = i
	if g.OnSelect != nil && i >= 0 {
		g.OnSelect(g.infoOf(i))
	}
	g.Refresh()
}

func (g *graphWidget) Tapped(e *fyne.PointEvent) {
	if i := g.pick(e.Position); i >= 0 {
		g.selectNode(i)
	}
}

// double click on a wallet expands it; on empty space it resets the view
func (g *graphWidget) DoubleTapped(e *fyne.PointEvent) {
	i := g.pick(e.Position)
	if i < 0 {
		g.resetView()
		return
	}
	g.selectNode(i)
	if !g.nodes[i].expanded && g.OnExpand != nil {
		g.OnExpand(g.nodes[i].addr)
	}
}

// eases onto the whole graph rather than jumping back to 1:1, which after a few
// expansions would leave most of it off screen
func (g *graphWidget) resetView() {
	g.fitting = fitFrames / 2
}

// dragging a node moves it, dragging empty space pans the view
func (g *graphWidget) Dragged(e *fyne.DragEvent) {
	if g.dragging < 0 && !g.panning {
		start := e.Position.Subtract(e.Dragged)
		if g.dragging = g.pick(start); g.dragging < 0 {
			g.panning = true
		}
	}
	if g.panning {
		g.fitting = 0 // the user takes over the view
		g.off = g.off.Add(e.Dragged)
		return
	}
	n := g.nodes[g.dragging]
	n.x, n.y = g.toWorld(e.Position)
	n.vx, n.vy = 0, 0
}

func (g *graphWidget) DragEnd() {
	g.dragging = -1
	g.panning = false
}

// wheel / two-finger scroll zooms around the pointer
func (g *graphWidget) Scrolled(e *fyne.ScrollEvent) {
	g.fitting = 0 // the user takes over the view
	f := clamp(1+e.Scrolled.DY/200, 0.8, 1.25)
	newScale := clamp(g.scale*f, minZoom, maxZoom)
	wx, wy := g.toWorld(e.Position)
	g.scale = newScale
	g.off = fyne.NewPos(
		e.Position.X-g.size.Width/2-wx*g.scale,
		e.Position.Y-g.size.Height/2-wy*g.scale,
	)
	g.Refresh()
}

func nodeRadius(n *gnode) float32 {
	switch {
	case n.isTarget:
		return 20
	case n.expanded:
		return 16
	case n.radius > 0:
		return n.radius
	}
	return 10
}

// nearest node within reach of the pointer, or -1
func (g *graphWidget) pick(pos fyne.Position) int {
	best, bestD := -1, float32(math.MaxFloat32)
	for i, n := range g.nodes {
		p := g.toScreen(n.x, n.y)
		dx, dy := p.X-pos.X, p.Y-pos.Y
		d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if d < max(nodeRadius(n)*g.scale, 9)+4 && d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// The selected node and its direct neighbours; nil when nothing is selected.
// Everything else is drawn faded, so one star can be read inside a busy graph.
func (g *graphWidget) focus() []bool {
	if g.selected < 0 || g.selected >= len(g.nodes) {
		return nil
	}
	f := make([]bool, len(g.nodes))
	f[g.selected] = true
	for _, e := range g.edges {
		if e.a == g.selected {
			f[e.b] = true
		}
		if e.b == g.selected {
			f[e.a] = true
		}
	}
	return f
}

// Labels are the main source of clutter: names (target, expanded wallets, known
// entities) always show, the top counterparties of each wallet too, the rest only
// on hover, on selection, or once zoomed in.
func (g *graphWidget) labelVisible(i int, lit bool) bool {
	n := g.nodes[i]
	important := n.isTarget || n.expanded || n.named || n.service
	switch {
	case i == g.selected || i == g.hover:
		return true
	case g.scale < 0.45:
		return n.isTarget
	case important:
		return true
	case !lit:
		return false
	case n.keyLabel:
		return true
	}
	return g.scale >= 1.4
}

type edgeObj struct {
	shaft *canvas.Line
	headA [2]*canvas.Line // arrow pointing at node a
	headB [2]*canvas.Line // arrow pointing at node b
	width float32
	base  color.NRGBA
	alpha uint8 // grows with volume
}

// Arrow colour reads from the wallet the edge was fetched from (the target, or
// the expanded wallet): pink = it sent, green = it received, purple = both.
func edgeColor(e *gedge) color.NRGBA {
	switch {
	case e.ab > 0 && e.ba > 0:
		return colEdge
	case e.ab > 0:
		return colOut
	}
	return colIn
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func (g *graphWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &graphRenderer{w: g}

	maxVol := 0.0
	for _, e := range g.edges {
		maxVol = math.Max(maxVol, e.ab+e.ba)
	}
	newLine := func(w float32) *canvas.Line {
		l := canvas.NewLine(colEdge)
		l.StrokeWidth = w
		return l
	}
	for _, e := range g.edges {
		// width and opacity grow with volume, on a log scale so one whale does not
		// flatten the rest: small flows recede, big ones stand out
		norm := float32(0.5)
		if maxVol > 0 {
			norm = float32(math.Log1p(e.ab+e.ba) / math.Log1p(maxVol))
		}
		o := &edgeObj{
			shaft: newLine(1 + 2.5*norm),
			width: 1 + 2.5*norm,
			base:  edgeColor(e),
			alpha: uint8(0x50 + (0xee-0x50)*norm),
		}
		for k := 0; k < 2; k++ {
			o.headA[k] = newLine(1.8)
			o.headB[k] = newLine(1.8)
		}
		r.edges = append(r.edges, o)
	}
	for _, n := range g.nodes {
		fill, stroke := colNode, colNodeHi
		switch {
		case n.isTarget:
			fill, stroke = colNodeHi, colText
		case n.expanded:
			fill = colNodeEx
		}
		switch {
		case n.entity != nil:
			stroke = entityColor(n.entity.Category)
		case n.service:
			stroke = colService
		case n.contract:
			stroke = colContract
		}
		r.fills = append(r.fills, fill)
		r.strokes = append(r.strokes, stroke)
		r.circles = append(r.circles, canvas.NewCircle(fill))

		lblCol := colTextDim
		lbl := canvas.NewText(n.label, lblCol)
		lbl.TextSize = 11
		lbl.Alignment = fyne.TextAlignCenter
		lbl.TextStyle = fyne.TextStyle{Monospace: !n.named}
		switch {
		case n.isTarget || n.expanded:
			lblCol = colText
			lbl.TextStyle.Bold = true
		case n.entity != nil:
			lblCol = entityColor(n.entity.Category)
		case n.service:
			lblCol = colService
		case n.contract:
			lblCol = colContract
		}
		r.labels = append(r.labels, lbl)
		r.lblCols = append(r.lblCols, lblCol)
	}
	// draw order: edges, then circles, then labels on top
	for _, e := range r.edges {
		r.objects = append(r.objects, e.shaft, e.headA[0], e.headA[1], e.headB[0], e.headB[1])
	}
	for _, c := range r.circles {
		r.objects = append(r.objects, c)
	}
	for _, l := range r.labels {
		r.objects = append(r.objects, l)
	}
	return r
}

type graphRenderer struct {
	w       *graphWidget
	edges   []*edgeObj
	circles []*canvas.Circle
	fills   []color.NRGBA
	strokes []color.NRGBA
	labels  []*canvas.Text
	lblCols []color.NRGBA
	objects []fyne.CanvasObject
}

// arrow head with its tip at (tx,ty), pointing along the unit vector (ux,uy)
func placeHead(head [2]*canvas.Line, tx, ty, ux, uy float32, col color.Color, show bool) {
	for _, l := range head {
		l.Hidden = !show
		l.StrokeColor = col
	}
	if !show {
		return
	}
	const size, spread = 8, 0.5
	for k, sgn := range []float32{1, -1} {
		cos, sin := float32(math.Cos(spread)), sgn*float32(math.Sin(spread))
		bx := -(ux*cos - uy*sin)
		by := -(ux*sin + uy*cos)
		head[k].Position1 = fyne.NewPos(tx, ty)
		head[k].Position2 = fyne.NewPos(tx+bx*size, ty+by*size)
	}
}

func (r *graphRenderer) relayout() {
	g := r.w
	if !g.placed {
		return
	}
	focus := g.focus()

	for i, e := range g.edges {
		a, b := g.nodes[e.a], g.nodes[e.b]
		pa, pb := g.toScreen(a.x, a.y), g.toScreen(b.x, b.y)
		o := r.edges[i]

		alpha := o.alpha
		if focus != nil && e.a != g.selected && e.b != g.selected {
			alpha /= 4
		}
		col := withAlpha(o.base, alpha)
		o.shaft.Position1, o.shaft.Position2 = pa, pb
		o.shaft.StrokeColor = col
		o.shaft.StrokeWidth = o.width * min(g.scale, 1.5)

		// heads sit along the edge, not on the node rim: a busy wallet would
		// otherwise wear a crown of overlapping arrow heads
		dx, dy := pb.X-pa.X, pb.Y-pa.Y
		d := float32(math.Sqrt(float64(dx*dx+dy*dy))) + 0.01
		ux, uy := dx/d, dy/d
		room := d > (nodeRadius(a)+nodeRadius(b))*g.scale+24
		tAB, tBA := float32(0.55), float32(0.45)
		if e.ab > 0 && e.ba > 0 {
			tAB, tBA = 0.62, 0.38
		}
		placeHead(o.headB, pa.X+dx*tAB, pa.Y+dy*tAB, ux, uy, col, room && e.ab > 0)
		placeHead(o.headA, pa.X+dx*tBA, pa.Y+dy*tBA, -ux, -uy, col, room && e.ba > 0)
	}

	for i, n := range g.nodes {
		lit := focus == nil || focus[i]
		rad := nodeRadius(n) * g.scale
		p := g.toScreen(n.x, n.y)
		c := r.circles[i]
		c.Resize(fyne.NewSize(rad*2, rad*2))
		c.Move(fyne.NewPos(p.X-rad, p.Y-rad))

		fill, stroke := r.fills[i], r.strokes[i]
		if !lit {
			fill, stroke = withAlpha(fill, 0x50), withAlpha(stroke, 0x50)
		}
		c.FillColor = fill
		switch i {
		case g.selected:
			c.StrokeColor, c.StrokeWidth = colText, 3.5
		case g.hover:
			c.StrokeColor, c.StrokeWidth = colText, 2.5
		default:
			c.StrokeColor, c.StrokeWidth = stroke, 2
		}

		const lw = 170
		lbl := r.labels[i]
		lbl.Hidden = !g.labelVisible(i, lit)
		lbl.Color = r.lblCols[i]
		if !lit && i != g.hover {
			lbl.Color = withAlpha(r.lblCols[i], 0x70)
		}
		lbl.Resize(fyne.NewSize(lw, 16))
		lbl.Move(fyne.NewPos(p.X-lw/2, p.Y+rad+2))
	}
}

func (r *graphRenderer) Layout(size fyne.Size) {
	r.w.size = size
	r.relayout()
}
func (r *graphRenderer) MinSize() fyne.Size           { return fyne.NewSize(400, 300) }
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
	eye      *canvas.Image // watermark, hidden behind a graph: it only adds noise there
	current  *graphWidget
	OnSelect func(NodeInfo)    // set by the dashboard, passed on to each new graph
	OnExpand func(addr string) // idem
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
		eye:  eye,
	}
	gv.Message("enter a wallet and press Analyze")
	return gv
}

// Fyne does not reliably Destroy a renderer that leaves the tree, so without this
// every replaced graph would keep its endless animation ticking in the background.
func (gv *GraphView) stop() {
	if gv.current != nil && gv.current.anim != nil {
		gv.current.anim.Stop()
	}
	gv.current = nil
}

func (gv *GraphView) Message(msg string) {
	gv.stop()
	gv.showEye(true)
	t := canvas.NewText(msg, colTextDim)
	t.Alignment = fyne.TextAlignCenter
	t.TextStyle = fyne.TextStyle{Italic: true}
	gv.host.Objects = []fyne.CanvasObject{container.NewCenter(t)}
	gv.host.Refresh()
}

func (gv *GraphView) showEye(show bool) {
	if gv.eye != nil {
		gv.eye.Hidden = !show
		gv.eye.Refresh()
	}
}

func (gv *GraphView) ResetView() {
	if gv.current != nil {
		gv.current.resetView()
	}
}

// Render draws the investigation for one asset. When the previous graph shows the
// same investigation (asset switch, expansion), node positions, zoom and selection
// carry over.
func (gv *GraphView) Render(inv *src.Investigation, asset string, maxNodes int) {
	old := gv.current
	gv.stop()

	g := newGraphWidget(inv, asset, maxNodes)
	g.OnSelect = gv.OnSelect
	g.OnExpand = gv.OnExpand

	selectAddr := inv.Target
	g.fitting = fitFrames // a fresh graph starts framed
	if old != nil && old.inv == inv {
		g.fitting = 0
		g.prev = map[string]fyne.Position{}
		g.prevExp = map[string]bool{}
		for _, n := range old.nodes {
			key := strings.ToLower(n.addr)
			g.prev[key] = fyne.NewPos(n.x, n.y)
			g.prevExp[key] = n.expanded
		}
		g.scale, g.off = old.scale, old.off
		// a new wallet was expanded: bring the grown graph into view
		for _, n := range g.nodes {
			if n.expanded && !g.prevExp[strings.ToLower(n.addr)] {
				g.fitting = fitFrames
				break
			}
		}
		if old.selected >= 0 && old.selected < len(old.nodes) {
			selectAddr = old.nodes[old.selected].addr
		}
	}

	gv.current = g
	gv.showEye(false)
	gv.host.Objects = []fyne.CanvasObject{g}
	gv.host.Refresh()
	// started only once the renderer exists: a tick refreshes the widget, and
	// refreshing from inside CreateRenderer would recreate the renderer endlessly
	g.startSim()

	// keep the selection if the node is still drawn, otherwise fall back to the target
	i := g.indexOf(selectAddr)
	if i < 0 {
		i = 0
	}
	g.selectNode(i)
}

// New info about one wallet (a click lookup): redraw its node without rebuilding.
func (gv *GraphView) UpdateWallet(addr string) {
	g := gv.current
	if g == nil {
		return
	}
	if i := g.indexOf(addr); i >= 0 {
		decorate(g.nodes[i], g.inv)
		g.Refresh()
	}
}

// Info of the selected node, refreshed after the graph changed under it.
func (gv *GraphView) Selected() (NodeInfo, bool) {
	if gv.current == nil || gv.current.selected < 0 {
		return NodeInfo{}, false
	}
	return gv.current.infoOf(gv.current.selected), true
}
