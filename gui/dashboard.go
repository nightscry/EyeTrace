package gui

import (
	"fmt"
	"image/color"
	"strings"

	"eyetracker/src"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func showDashboard(a fyne.App, initialAddress string) {
	w := a.NewWindow("EyeTrace " + src.Version + " — NightScry")

	input := widget.NewEntry()
	input.SetPlaceHolder("Enter a wallet address (BTC, ETH, DOGE, TRON, XRP, SOL)")

	panel := NewSidePanel()

	graph := NewGraphView()

	// search depth, driven by the Low/Medium/High buttons below:
	// txLimit = how many txs to download per wallet, nodeCap = counterparties drawn per wallet.
	txLimit := 100
	nodeCap := 25

	// current investigation; all of it is only touched on the UI thread
	var (
		inv          *src.Investigation
		conn         src.Connector
		currentAsset string
		assetByLabel = map[string]string{}
		expanding    = map[string]bool{}
	)
	// bumped on every Analyze/Open: a slow earlier request must not overwrite a newer result
	analysisID := 0

	render := func() { graph.Render(inv, currentAsset, nodeCap) }

	assetSelect := widget.NewSelect(nil, func(label string) {
		if a, ok := assetByLabel[label]; ok && a != currentAsset {
			currentAsset = a
			render()
		}
	})
	assetSelect.PlaceHolder = "—"
	assetSelect.Disable()

	var exportBtn *widget.Button

	// Rebuilds the selector from every transfer of the investigation (an expansion can
	// bring new assets), keeps the current pick when it still exists, and redraws.
	refreshAssets := func() {
		native := src.NativeAsset(inv.Chain)
		assets := assetsOf(inv.AllTransfers(), native)

		assetByLabel = map[string]string{}
		labels := make([]string, 0, len(assets))
		selected := ""
		for _, as := range assets {
			l := fmt.Sprintf("%s  (%d)", as.Asset, as.Count)
			assetByLabel[l] = as.Asset
			labels = append(labels, l)
			if as.Asset == currentAsset {
				selected = l
			}
		}
		exportBtn.Enable()

		// no history at all: still draw the target, in the native coin
		if len(labels) == 0 {
			currentAsset = native
			assetSelect.Options = nil
			assetSelect.ClearSelected()
			assetSelect.Disable()
			render()
			return
		}
		if selected == "" {
			selected = labels[0]
		}
		currentAsset = assetByLabel[selected]
		assetSelect.Options = labels
		assetSelect.Enable()
		assetSelect.SetSelected(selected) // no-op in the callback: currentAsset already matches
		render()
	}

	setInvestigation := func(i *src.Investigation, c src.Connector) {
		inv, conn = i, c
		currentAsset = ""
		expanding = map[string]bool{}
		refreshAssets()
	}

	analyzeFn := func() {
		addr := strings.TrimSpace(input.Text)
		if addr == "" {
			return
		}
		limit := txLimit // snapshot on the UI thread, no race with the goroutine
		analysisID++
		id := analysisID
		graph.Message("· · ·  loading  · · ·")
		assetSelect.Disable()

		// network work off the UI thread, otherwise the window freezes
		go func() {
			fail := func(msg string) {
				fyne.Do(func() {
					if id == analysisID {
						graph.Message("❌ " + msg)
					}
				})
			}

			wl := src.Wallet{Address: addr}
			if err := wl.Indentification(); err != nil {
				fail(err.Error())
				return
			}
			c, err := src.ConnectorFor(wl)
			if err != nil {
				fail(err.Error())
				return
			}
			balance, err := c.GetBalance(wl.Address)
			if err != nil {
				fail("network error: " + err.Error())
				return
			}
			transfers, err := c.GetTransactions(addr, limit)
			if err != nil {
				fail("network error: " + err.Error())
				return
			}

			// every UI update goes inside fyne.Do
			fyne.Do(func() {
				if id != analysisID {
					return // the user started another analysis meanwhile
				}
				setInvestigation(src.NewInvestigation(wl, balance, transfers), c)
			})
		}()
	}

	// Expanding = fetching a counterparty's own history and adding its counterparties,
	// which is how the graph grows past one hop.
	expand := func(addr string) {
		if inv == nil || conn == nil || inv.IsExpanded(addr) || expanding[addr] {
			return
		}
		myInv, c, limit := inv, conn, txLimit
		expanding[addr] = true
		panel.ShowLoading(addr)

		go func() {
			// same recognition the target gets: detector subtype, balance, tx count and
			// the explorer's own identity of the address. The balance is a nice-to-have:
			// if it fails the history still makes it into the graph.
			info := src.Recognize(c, addr)

			transfers, err := c.GetTransactions(addr, limit)
			fyne.Do(func() {
				delete(expanding, addr)
				if inv != myInv {
					return // another investigation was opened meanwhile
				}
				if err != nil {
					dialog.ShowError(fmt.Errorf("expanding %s: %w", short(addr), err), w)
					if sel, ok := graph.Selected(); ok {
						panel.Show(sel)
					}
					return
				}
				inv.AddHistory(addr, transfers)
				inv.SetWalletInfo(addr, info)
				refreshAssets()
			})
		}()
	}
	graph.OnExpand = expand
	panel.OnExpand = expand

	// Selecting a counterparty identifies it on the spot (balance, tx count, on-chain
	// name), without downloading its history: that is what Expand is for. A failed
	// lookup is retried on the next click.
	identifying := map[string]bool{}
	graph.OnSelect = func(n NodeInfo) {
		need := inv != nil && conn != nil && !n.IsTarget && !expanding[n.Address]
		if need {
			info, known := inv.WalletInfo(n.Address)
			need = !known || info.BalanceErr != ""
		}
		if !need || identifying[n.Address] {
			n.Identifying = identifying[n.Address]
			panel.Show(n)
			return
		}
		n.Identifying = true
		panel.Show(n)

		myInv, c, addr := inv, conn, n.Address
		identifying[addr] = true
		go func() {
			info := src.Recognize(c, addr)
			fyne.Do(func() {
				delete(identifying, addr)
				if inv != myInv || inv.IsExpanded(addr) {
					return // the expansion brought its own, fresher info
				}
				inv.SetWalletInfo(addr, info)
				graph.UpdateWallet(addr)
				if sel, ok := graph.Selected(); ok && src.SameAddress(inv.Chain, sel.Address, addr) {
					panel.Show(sel)
				}
			})
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
	versionText := canvas.NewText("v"+src.Version, colTextDim)
	versionText.TextSize = 11
	headerLine := canvas.NewRectangle(colBorder)
	headerLine.SetMinSize(fyne.NewSize(0, 1))
	header := container.NewVBox(
		container.NewPadded(container.NewHBox(eyeIcon, appTitle, container.NewCenter(versionText))),
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
		// the node cap applies right away; the tx limit to the next fetch
		if inv != nil {
			render()
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

	// Save: .json keeps the whole investigation (reopenable offline), .csv is a flat
	// table of transfers for spreadsheets and reports.
	exportBtn = widget.NewButtonWithIcon("Export", theme.DocumentSaveIcon(), func() {
		if inv == nil {
			return
		}
		d := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
			if err != nil || wc == nil {
				return
			}
			defer wc.Close()
			if strings.HasSuffix(strings.ToLower(wc.URI().Name()), ".csv") {
				err = inv.WriteCSV(wc)
			} else {
				err = inv.SaveJSON(wc)
			}
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			dialog.ShowInformation("Export", "Saved "+wc.URI().Name(), w)
		}, w)
		d.SetFileName("eyetrace-" + strings.ToLower(inv.Chain) + "-" + inv.Target[:min(8, len(inv.Target))] + ".json")
		d.SetFilter(storage.NewExtensionFileFilter([]string{".json", ".csv"}))
		d.Show()
	})
	exportBtn.Disable()

	openBtn := widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			loaded, err := src.LoadInvestigation(rc)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			// the connector is rebuilt so a reopened case can keep being expanded
			c, err := src.ConnectorFor(src.Wallet{Address: loaded.Target, Type: loaded.Chain, Testnet: loaded.Testnet})
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			analysisID++ // drop any analysis still in flight
			input.SetText(loaded.Target)
			setInvestigation(loaded, c)
		}, w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		d.Show()
	})

	resetBtn := widget.NewButtonWithIcon("", theme.ViewRestoreIcon(), graph.ResetView)

	assetLabel := canvas.NewText("Asset", colSilver)
	assetLabel.TextStyle = fyne.TextStyle{Bold: true}
	assetRow := container.NewBorder(nil, nil, assetLabel,
		container.NewHBox(openBtn, exportBtn, resetBtn), assetSelect)

	// The opaque rectangles behind the top and bottom strips are required: the graph
	// widget does not clip to its own bounds, so a dragged node would show through.
	topContent := container.NewVBox(header, container.NewPadded(topBar), container.NewPadded(depthRow), container.NewPadded(assetRow))
	top := container.NewStack(canvas.NewRectangle(colBg), topContent)
	bottom := container.NewStack(canvas.NewRectangle(colBg), container.NewPadded(legend()))

	content := container.NewBorder(
		top,
		bottom,
		panel.Root,
		nil,
		graph.Root,
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(1100, 760))
	w.CenterOnScreen()
	w.Show()

	if initialAddress != "" {
		input.SetText(initialAddress)
		analyzeFn()
	}
}

// Two groups, since the same palette idea is used twice: arrows say which way the
// money went, node rims say what a wallet is.
func legend() fyne.CanvasObject {
	item := func(c color.Color, s string) fyne.CanvasObject {
		dot := canvas.NewText("●", c)
		t := canvas.NewText(s, colTextDim)
		t.TextSize = 11
		return container.NewHBox(dot, t)
	}
	group := func(s string) fyne.CanvasObject {
		t := canvas.NewText(s, colSilver)
		t.TextSize = 11
		t.TextStyle = fyne.TextStyle{Bold: true}
		return t
	}
	help := canvas.NewText("click: focus  ·  double-click: expand  ·  hover: name  ·  scroll: zoom  ·  drag: pan", colTextDim)
	help.TextSize = 11
	return container.NewVBox(
		container.NewHBox(
			group("Arrows"),
			item(colOut, "sent"),
			item(colIn, "received"),
			item(colEdge, "both ways"),
			layout.NewSpacer(),
			container.NewCenter(help),
		),
		container.NewHBox(
			group("Nodes"),
			item(colExchange, "exchange"),
			item(colMixer, "mixer"),
			item(colContract, "contract / token"),
			item(colBurn, "burn"),
			item(colOther, "other known"),
			item(colService, "service-like (heuristic)"),
		),
	)
}
