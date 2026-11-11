package gui

import (
	"fmt"
	"net/url"
	"strings"

	"eyetracker/src"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type SidePanel struct {
	Root     *fyne.Container
	body     *fyne.Container // rebuilt on every selection
	OnExpand func(addr string)
}

func NewSidePanel() *SidePanel {
	bg := canvas.NewRectangle(colPanel)
	bg.SetMinSize(fyne.NewSize(290, 0)) // fixed width

	body := container.NewVBox()
	p := &SidePanel{body: body}
	// the opaque background also hides graph nodes panned or dragged under the panel
	p.Root = container.NewStack(bg, container.NewVScroll(container.NewPadded(body)))
	p.showEmpty()
	return p
}

func titleText(s string) *canvas.Text {
	t := canvas.NewText(s, colPurpleHi)
	t.TextSize = 15
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func sectionHdr(s string) *canvas.Text {
	t := canvas.NewText(s, colTextDim)
	t.TextSize = 10
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func bigValue(s string) *canvas.Text {
	t := canvas.NewText(s, colText)
	t.TextSize = 17
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func value(s string) *canvas.Text {
	t := canvas.NewText(s, colSilver)
	t.TextSize = 13
	return t
}

func addrLabel(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Wrapping = fyne.TextWrapBreak // long addresses wrap instead of overflowing
	l.TextStyle = fyne.TextStyle{Monospace: true}
	return l
}

func wrapped(s string, imp widget.Importance) *widget.Label {
	l := widget.NewLabel(s)
	l.Wrapping = fyne.TextWrapWord
	l.Importance = imp
	return l
}

// %.8f is right for BTC and unreadable for 600M USDT: keep 8 decimals only for small values
func amount(v float64, unit string) string {
	switch {
	case v >= 1e6:
		return fmt.Sprintf("%s %s", groupThousands(fmt.Sprintf("%.0f", v)), unit)
	case v >= 1:
		return fmt.Sprintf("%s %s", groupThousands(fmt.Sprintf("%.4f", v)), unit)
	}
	return fmt.Sprintf("%.8f %s", v, unit)
}

func groupThousands(s string) string {
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String() + frac
	if neg {
		out = "-" + out
	}
	return out
}

func (p *SidePanel) set(objs ...fyne.CanvasObject) {
	p.body.Objects = objs
	p.body.Refresh()
}

func (p *SidePanel) showEmpty() {
	hint := widget.NewLabelWithStyle("Click a node in the graph\nto see its details.\n\nDouble-click a wallet to expand it,\nscroll to zoom, drag the background to pan.",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	p.set(
		titleText("EYETRACE"),
		widget.NewSeparator(),
		hint,
	)
}

func (p *SidePanel) ShowLoading(addr string) {
	p.set(
		titleText("EXPANDING"),
		widget.NewSeparator(),
		addrLabel(addr),
		widget.NewProgressBarInfinite(),
	)
}

func (p *SidePanel) actions(n NodeInfo) fyne.CanvasObject {
	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		fyne.CurrentApp().Clipboard().SetContent(n.Address)
	})
	var row []fyne.CanvasObject
	row = append(row, copyBtn)
	if link := src.ExplorerURL(n.Chain, n.Testnet, n.Address); link != "" {
		if u, err := url.Parse(link); err == nil {
			row = append(row, widget.NewButtonWithIcon("Explorer", theme.ComputerIcon(), func() {
				fyne.CurrentApp().OpenURL(u)
			}))
		}
	}
	box := container.NewVBox(container.NewGridWithColumns(len(row), row...))
	if !n.Expanded && p.OnExpand != nil {
		exp := widget.NewButtonWithIcon("Expand: fetch its transactions", theme.ZoomInIcon(), func() {
			p.OnExpand(n.Address)
		})
		exp.Importance = widget.HighImportance
		box.Add(exp)
	}
	return box
}

func entityRows(e *src.Entity) []fyne.CanvasObject {
	if e == nil {
		return nil
	}
	t := canvas.NewText(e.Name, entityColor(e.Category))
	t.TextSize = 14
	t.TextStyle = fyne.TextStyle{Bold: true}
	rows := []fyne.CanvasObject{sectionHdr("KNOWN ENTITY"), t, value(strings.ToUpper(e.Category[:1]) + e.Category[1:])}
	if e.Risky() {
		rows = append(rows, wrapped("⚠  Mixer: funds through it lose their on-chain trail.", widget.DangerImportance))
	}
	return rows
}

// Balance and tx count are the wallet's own; In/Out cover only the edges drawn,
// in the asset the graph is filtered on. Every recognised wallet (the target and
// each expanded one) gets the same identity block.
func (p *SidePanel) Show(n NodeInfo) {
	var objs []fyne.CanvasObject
	switch {
	case n.IsTarget:
		objs = append(objs, titleText("👁  ANALYZED WALLET"))
	case n.Expanded:
		objs = append(objs, titleText("EXPANDED WALLET"))
	default:
		objs = append(objs, titleText("COUNTERPARTY"))
	}
	objs = append(objs, widget.NewSeparator())

	if n.Recognized {
		kind := n.Chain
		if n.Testnet {
			kind += " testnet"
		}
		if n.Subtype != "" {
			kind += "  ·  " + n.Subtype
		}
		objs = append(objs, sectionHdr("TYPE"), value(kind), sectionHdr("BALANCE"))
		switch {
		case n.HasBalance:
			objs = append(objs, bigValue(amount(n.Balance, n.BalanceUnit)))
		case n.Identifying:
			objs = append(objs, value("retrying…"))
		default:
			objs = append(objs, value("unavailable"))
			if n.BalanceErr != "" {
				objs = append(objs, wrapped(n.BalanceErr, widget.LowImportance))
			}
		}
		if n.TxCount > 0 {
			objs = append(objs, sectionHdr("TOTAL TRANSACTIONS"), value(groupThousands(fmt.Sprint(n.TxCount))))
		}
	}

	objs = append(objs, sectionHdr("ADDRESS"), addrLabel(n.Address), p.actions(n))
	objs = append(objs, entityRows(n.Entity)...)

	if n.OnChainName != "" || n.IsContract {
		objs = append(objs, sectionHdr("ON-CHAIN IDENTITY"))
		if n.OnChainName != "" {
			t := canvas.NewText(n.OnChainName, colContract)
			t.TextStyle = fyne.TextStyle{Bold: true}
			objs = append(objs, t)
		}
		if n.IsContract {
			objs = append(objs, value("Smart contract"))
		}
	}
	if len(n.Profile.Hints) > 0 {
		objs = append(objs, sectionHdr("PROFILE  ·  HEURISTIC"))
		for _, h := range n.Profile.Hints {
			imp := widget.MediumImportance
			if n.Profile.ServiceLike {
				imp = widget.WarningImportance
			}
			objs = append(objs, wrapped(h, imp))
		}
	}
	switch {
	case !n.Recognized && n.Identifying:
		objs = append(objs, wrapped("Identifying…  balance, tx count, on-chain name", widget.LowImportance))
	case !n.Expanded && !n.IsTarget:
		objs = append(objs, wrapped("Expand this wallet to add its own counterparties to the graph.", widget.LowImportance))
	}

	objs = append(objs,
		widget.NewSeparator(),
		sectionHdr("IN THIS GRAPH  ·  "+n.Asset),
		sectionHdr("RECEIVED"),
		value(amount(n.In, n.Asset)),
		sectionHdr("SENT"),
		value(amount(n.Out, n.Asset)),
		sectionHdr("TRANSFERS  ·  LINKS"),
		value(fmt.Sprintf("%d  ·  %d", n.Count, n.Links)),
	)
	if isUnverified(n.Asset) {
		objs = append(objs, wrapped("⚠  Unverified token: unknown contract, possibly spam or a fake copy of a real token.", widget.WarningImportance))
	}

	if n.IsTarget {
		objs = append(objs, widget.NewSeparator(), sectionHdr("RISK  ·  MIXER EXPOSURE"))
		if len(n.Risk) == 0 {
			objs = append(objs, wrapped("No known mixer in this graph. Only built-in labels are checked, so this is not a clean bill of health.", widget.MediumImportance))
		} else {
			for _, r := range n.Risk {
				objs = append(objs, wrapped("⚠  "+r, widget.DangerImportance))
			}
		}
	}
	p.set(objs...)
}
