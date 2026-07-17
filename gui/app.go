package gui

import (
	"fyne.io/fyne/v2/app"
)

// RunGUI è il punto d'ingresso dell'interfaccia grafica.
// Flusso: crea l'app → applica il tema NightScry → mostra la splash →
// (dopo qualche secondo) la splash apre la dashboard.
func RunGUI() {
	a := app.New()
	a.Settings().SetTheme(&nightScryTheme{})

	showSplash(a, func() {
		showDashboard(a)
	})

	a.Run() // blocca finché non chiudi l'ultima finestra
}
