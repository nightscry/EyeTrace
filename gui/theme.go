package gui

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Logo NightScry embeddato nel binario (nessuna dipendenza da path esterni).
//
//go:embed assets/logo.png
var logoPNG []byte

var logoResource = fyne.NewStaticResource("logo.png", logoPNG)

// Solo l'occhio (ritagliato, sfondo trasparente) → usato come watermark.
//
//go:embed assets/eye.png
var eyePNG []byte

var eyeResource = fyne.NewStaticResource("eye.png", eyePNG)

// Palette NightScry (estratta dal styles.css del sito).
var (
	colBg        = color.NRGBA{0x04, 0x03, 0x09, 0xff} // sfondo nero-viola
	colBg2       = color.NRGBA{0x0a, 0x07, 0x12, 0xff} // sfondo secondario / input
	colPanel     = color.NRGBA{0x12, 0x0d, 0x1e, 0xff} // pannelli / bottoni
	colText      = color.NRGBA{0xe7, 0xe3, 0xf2, 0xff} // testo bianco-lavanda
	colTextDim   = color.NRGBA{0xa3, 0x9d, 0xb8, 0xff} // testo attenuato
	colSilver    = color.NRGBA{0xd7, 0xda, 0xe2, 0xff} // argento (titoli)
	colPurple    = color.NRGBA{0x8b, 0x5c, 0xf6, 0xff} // viola
	colPurpleHi  = color.NRGBA{0xa8, 0x55, 0xf7, 0xff} // viola brillante (accento)
	colBorder    = color.NRGBA{0x7c, 0x3a, 0xed, 0x60} // bordo viola tenue
)

// nightScryTheme implementa fyne.Theme: sovrascrive i colori con la palette
// NightScry e delega tutto il resto (font, icone, dimensioni) al tema di default.
type nightScryTheme struct{}

var _ fyne.Theme = (*nightScryTheme)(nil)

func (t *nightScryTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBg
	case theme.ColorNameForeground:
		return colText
	case theme.ColorNamePrimary:
		return colPurpleHi
	case theme.ColorNameButton:
		return colPanel
	case theme.ColorNameInputBackground:
		return colBg2
	case theme.ColorNamePlaceHolder:
		return colTextDim
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return colBorder
	case theme.ColorNameHover:
		return color.NRGBA{0x8b, 0x5c, 0xf6, 0x33}
	case theme.ColorNameFocus:
		return color.NRGBA{0x8b, 0x5c, 0xf6, 0x66}
	case theme.ColorNameSelection:
		return color.NRGBA{0x8b, 0x5c, 0xf6, 0x40}
	}
	// tutto il resto: tema scuro di default
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (t *nightScryTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }
func (t *nightScryTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}
func (t *nightScryTheme) Size(n fyne.ThemeSizeName) float32 { return theme.DefaultTheme().Size(n) }
