package gui

import (
	"eyetracker/src"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

const appID = "com.nightscry.eyetrace"

func RunGUI(initialAddress string) {
	// Set in code so it also holds for plain `go run`/`go build` (FyneApp.toml is only
	// read by the fyne packaging tool). The ID backs the Preferences API used by the
	// file dialogs; fyneDo declares that every UI update already goes through fyne.Do.
	app.SetMetadata(fyne.AppMetadata{
		ID:         appID,
		Name:       "EyeTrace",
		Version:    src.Version,
		Build:      1,
		Migrations: map[string]bool{"fyneDo": true},
	})
	a := app.NewWithID(appID)
	a.Settings().SetTheme(&nightScryTheme{})

	showSplash(a, func() {
		showDashboard(a, initialAddress)
	})

	a.Run() // blocks until the last window closes
}
