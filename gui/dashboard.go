package gui

import (
	"strings"

	"eyetracker/src"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// showDashboard apre la finestra principale: barra di input in alto + area grafo sotto.
func showDashboard(a fyne.App) {
	w := a.NewWindow("EyeTrace — NightScry")

	// --- barra input ---
	input := widget.NewEntry()
	input.SetPlaceHolder("Inserisci un indirizzo wallet (BTC, ETH, ...)")

	// pannello laterale coi dati del nodo selezionato
	panel := NewSidePanel()

	// area del grafo (con watermark occhio); la popoliamo dopo l'analisi
	graph := NewGraphView()
	// quando clicchi un nodo nel grafo, aggiorna il pannello
	graph.OnSelect = func(n NodeInfo) { panel.Show(n) }

	// funzione di analisi (riusata dal bottone e dall'invio nell'input)
	analyzeFn := func() {
		addr := strings.TrimSpace(input.Text)
		if addr == "" {
			return
		}
		graph.Message("· · ·  caricamento  · · ·")

		// lavoro pesante (rete) in una goroutine → la UI non si blocca
		go func() {
			wl := src.Wallet{Address: addr}
			var connector src.Connector

			if err := wl.Indentification(); err != nil {
				fyne.Do(func() { graph.Message("❌ " + err.Error()) })
				return
			}
	
			switch wl.Type{
			case "BTC":
				connector = src.BTCconnector{}
			case "ETH":
				connector = src.ETHConnector{}
			
			default:
				fyne.Do(func() {
					graph.Message("Grafo disponibile solo per BTC (per ora).  Tipo rilevato: " + wl.Type)
				})
				return
			}
			




			balance, err := connector.GetBalance(wl.Address)
			if(err!=nil){
				fyne.Do(func() { graph.Message("❌ errore di rete: " + err.Error()) })
				return
			}
			wl.Balance = balance 
			// riusa lo storico se già scaricato dall'identificazione, altrimenti scaricalo
			transfers := wl.History
			if len(transfers) == 0 {
				t, err := connector.GetTransactions(addr)
				if err != nil {
					fyne.Do(func() { graph.Message("❌ errore di rete: " + err.Error()) })
					return
				}
				transfers = t
			}

			// aggiornamento UI → SEMPRE dentro fyne.Do
			// wl.Type ("BTC"/"ETH") diventa l'unità mostrata nel pannello
			fyne.Do(func() { graph.Render(addr, wl.Type, wl.Balance, transfers) })
		}()
	}

	analyze := widget.NewButton("🔍  Analizza", analyzeFn)
	analyze.Importance = widget.HighImportance // colore "primary" (viola)
	input.OnSubmitted = func(string) { analyzeFn() } // premere Invio = Analizza

	label := canvas.NewText("Wallet", colSilver)
	label.TextStyle = fyne.TextStyle{Bold: true}

	// input che si allarga, label a sinistra, bottone a destra
	topBar := container.NewBorder(nil, nil, label, analyze, input)

	// --- header con logo (occhio) + titolo ---
	eyeIcon := canvas.NewImageFromResource(eyeResource)
	eyeIcon.FillMode = canvas.ImageFillContain
	eyeIcon.SetMinSize(fyne.NewSize(32, 32))
	appTitle := canvas.NewText("EYETRACE", colSilver)
	appTitle.TextSize = 20
	appTitle.TextStyle = fyne.TextStyle{Bold: true}
	headerLine := canvas.NewRectangle(colBorder)
	headerLine.SetMinSize(fyne.NewSize(0, 1))
	header := container.NewVBox(
		container.NewPadded(container.NewHBox(eyeIcon, appTitle)),
		headerLine,
	)

	// zona superiore: header + barra di input
	top := container.NewVBox(header, container.NewPadded(topBar))

	// layout: header/input in alto, pannello a sinistra, grafo che riempie il resto
	content := container.NewBorder(
		top,        // top
		nil,        // bottom
		panel.Root, // left
		nil,        // right
		graph.Root, // centro (riempie)
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(1000, 700))
	w.CenterOnScreen()
	w.Show()
}
