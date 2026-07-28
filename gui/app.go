package gui

import (
	"fyne.io/fyne/v2/app"
)

func RunGUI() {
	a := app.New()
	a.Settings().SetTheme(&nightScryTheme{})

	showSplash(a, func() {
		showDashboard(a)
	})

	a.Run() // blocks until the last window closes
}
