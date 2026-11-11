package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
)

func showSplash(a fyne.App, onDone func()) {
	// borderless splash window when the driver supports it, plain window otherwise
	var w fyne.Window
	if drv, ok := a.Driver().(desktop.Driver); ok {
		w = drv.CreateSplashWindow()
	} else {
		w = a.NewWindow("EyeTrace")
	}

	logo := canvas.NewImageFromResource(logoResource)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(180, 180))

	title := canvas.NewText("EyeTrace", colPurpleHi)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 24
	title.Alignment = fyne.TextAlignCenter

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

	bg := canvas.NewRectangle(colBg)
	w.SetContent(container.NewStack(bg, container.NewPadded(content)))
	w.Resize(fyne.NewSize(380, 320))
	w.CenterOnScreen()
	w.Show()

	// UI work must run on the main thread, hence fyne.Do
	time.AfterFunc(2000*time.Millisecond, func() {
		fyne.Do(func() {
			onDone() // open the dashboard first, then close the splash
			w.Close()
		})
	})
}
