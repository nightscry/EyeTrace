package gui

import (
	"strings"

	"eyetracker/src"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showDashboard(a fyne.App) {
	w := a.NewWindow("EyeTrace — NightScry")

	input := widget.NewEntry()
	input.SetPlaceHolder("Enter a wallet address (BTC, ETH, ...)")

	panel := NewSidePanel()

	graph := NewGraphView()
	graph.OnSelect = func(n NodeInfo) { panel.Show(n) }

	// search depth, driven by the Low/Medium/High buttons below:
	// txLimit = how many txs to download, nodeCap = how many counterparties to draw.
	txLimit := 100
	nodeCap := 25

	analyzeFn := func() {
		addr := strings.TrimSpace(input.Text)
		if addr == "" {
			return
		}
		limit, nodes := txLimit, nodeCap // snapshot on the UI thread, no race with the goroutine
		graph.Message("· · ·  loading  · · ·")

		// network work off the UI thread, otherwise the window freezes
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
			case "DOGE":
				connector = src.DOGEConnector{}

			default:
				fyne.Do(func() {
					graph.Message("Graph available for BTC, DOGE and ETH only (for now).  Detected type: " + wl.Type)
				})
				return
			}





			balance, err := connector.GetBalance(wl.Address)
			if(err!=nil){
				fyne.Do(func() { graph.Message("❌ network error: " + err.Error()) })
				return
			}
			wl.Balance = balance.Balance

			transfers := wl.History
			if len(transfers) == 0 {
				t, err := connector.GetTransactions(addr,limit)
				if err != nil {
					fyne.Do(func() { graph.Message("❌ network error: " + err.Error()) })
					return
				}
				transfers = t
			}

			// every UI update goes inside fyne.Do
			fyne.Do(func() { graph.Render(addr, wl.Type, wl.Balance, transfers, nodes) })
		}()
	}

	analyze := widget.NewButton("🔍  Analyze", analyzeFn)
	analyze.Importance = widget.HighImportance
	input.OnSubmitted = func(string) { analyzeFn() }

	label := canvas.NewText("Wallet", colSilver)
	label.TextStyle = fyne.TextStyle{Bold: true}

	topBar := container.NewBorder(nil, nil, label, analyze, input)

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

	// presets: {txs to download, counterparties to draw}
	const (
		lowTx, lowNodes   = 25, 12
		medTx, medNodes   = 100, 25
		highTx, highNodes = 500, 50
	)
	var btnLow, btnMed, btnHigh *widget.Button
	setDepth := func(limit, nodes int, active *widget.Button) {
		txLimit, nodeCap = limit, nodes
		for _, b := range []*widget.Button{btnLow, btnMed, btnHigh} {
			if b == active {
				b.Importance = widget.HighImportance
			} else {
				b.Importance = widget.MediumImportance
			}
			b.Refresh()
		}
	}
	btnLow = widget.NewButton("Low", func() { setDepth(lowTx, lowNodes, btnLow) })
	btnMed = widget.NewButton("Medium", func() { setDepth(medTx, medNodes, btnMed) })
	btnHigh = widget.NewButton("High", func() { setDepth(highTx, highNodes, btnHigh) })
	setDepth(medTx, medNodes, btnMed) // default

	depthLabel := canvas.NewText("Depth", colSilver)
	depthLabel.TextStyle = fyne.TextStyle{Bold: true}

	depthRow := container.NewBorder(nil, nil, depthLabel, nil,
		container.NewGridWithColumns(3, btnLow, btnMed, btnHigh))

	// The opaque rectangle behind this strip is required: the graph widget does not
	// clip to its own bounds, so a dragged node would show through the header.
	topContent := container.NewVBox(header, container.NewPadded(topBar), container.NewPadded(depthRow))
	top := container.NewStack(canvas.NewRectangle(colBg), topContent)

	content := container.NewBorder(
		top,
		nil,
		panel.Root,
		nil,
		graph.Root,
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(1000, 700))
	w.CenterOnScreen()
	w.Show()
}
