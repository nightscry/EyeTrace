package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SidePanel: pannello laterale sinistro coi dati del nodo selezionato.
type SidePanel struct {
	Root *fyne.Container
	body *fyne.Container // contenuto ricostruito ad ogni selezione
}

func NewSidePanel() *SidePanel {
	bg := canvas.NewRectangle(colPanel)
	bg.SetMinSize(fyne.NewSize(290, 0)) // larghezza fissa

	body := container.NewVBox()
	p := &SidePanel{body: body}
	p.Root = container.NewStack(bg, container.NewPadded(body))
	p.showEmpty()
	return p
}

// ── piccoli helper di stile ──

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
	l.Wrapping = fyne.TextWrapBreak // gli indirizzi lunghi vanno a capo
	l.TextStyle = fyne.TextStyle{Monospace: true}
	return l
}

func (p *SidePanel) set(objs ...fyne.CanvasObject) {
	p.body.Objects = objs
	p.body.Refresh()
}

func (p *SidePanel) showEmpty() {
	hint := widget.NewLabelWithStyle("Clicca un nodo nel grafo\nper vederne i dettagli",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	p.set(
		titleText("EYETRACE"),
		widget.NewSeparator(),
		hint,
	)
}

// Show aggiorna il pannello coi dati del nodo cliccato.
// L'unità (BTC/ETH/...) arriva da n.Asset, quindi è dinamica.
func (p *SidePanel) Show(n NodeInfo) {
	if n.IsTarget {
		p.set(
			titleText("👁  WALLET ANALIZZATO"),
			widget.NewSeparator(),
			sectionHdr("SALDO"),
			bigValue(fmt.Sprintf("%.8f %s", n.Balance, n.Asset)),
			widget.NewSeparator(),
			sectionHdr("INDIRIZZO"),
			addrLabel(n.Address),
			sectionHdr("RUOLO"),
			value("Target dell'indagine"),
		)
		return
	}
	p.set(
		titleText("CONTROPARTE"),
		widget.NewSeparator(),
		sectionHdr("INDIRIZZO"),
		addrLabel(n.Address),
		widget.NewSeparator(),
		sectionHdr("SCAMBIATO COL TARGET"),
		bigValue(fmt.Sprintf("%.8f %s", n.Amount, n.Asset)),
		sectionHdr("N. TRANSAZIONI"),
		value(fmt.Sprintf("%d", n.Count)),
	)
}
