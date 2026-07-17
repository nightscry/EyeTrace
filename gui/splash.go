package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
)

// showSplash mostra la finestrella di caricamento col logo NightScry.
// Dopo ~2.5s la chiude e chiama onDone (che apre la dashboard).
func showSplash(a fyne.App, onDone func()) {
	// se il driver supporta le splash window (senza bordi), usiamola;
	// altrimenti ripieghiamo su una finestra normale.
	var w fyne.Window
	if drv, ok := a.Driver().(desktop.Driver); ok {
		w = drv.CreateSplashWindow()
	} else {
		w = a.NewWindow("EyeTracking")
	}

	// logo
	logo := canvas.NewImageFromResource(logoResource)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(180, 180))

	// scritta "EyeTracking" in viola accento
	title := canvas.NewText("EyeTracking", colPurpleHi)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 24
	title.Alignment = fyne.TextAlignCenter

	// riga "loading"
	loading := canvas.NewText("· · ·  loading  · · ·", colTextDim)
	loading.TextStyle = fyne.TextStyle{Monospace: true}
	loading.Alignment = fyne.TextAlignCenter

	content := container.NewVBox(
		layout.NewSpacer(),
		logo,
		title,
		loading,
		layout.NewSpacer(),
	)

	// sfondo scuro dietro tutto
	bg := canvas.NewRectangle(colBg)
	w.SetContent(container.NewStack(bg, container.NewPadded(content)))
	w.Resize(fyne.NewSize(380, 320))
	w.CenterOnScreen()
	w.Show()

	// timer: dopo 2.5s apri la dashboard e chiudi la splash.
	// le operazioni UI vanno eseguite sul thread principale → fyne.Do.
	time.AfterFunc(5000*time.Millisecond, func() {
		fyne.Do(func() {
			onDone()  // apri la dashboard PRIMA
			w.Close() // poi chiudi la splash
		})
	})
}
