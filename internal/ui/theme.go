package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// accentColor is the brand primary used for buttons/headers.
var accentColor = color.NRGBA{R: 0x0f, G: 0xb5, B: 0xae, A: 0xff}

type vepeenTheme struct {
	fyne.Theme
}

// NewTheme returns the Vepeen custom theme (dark base + teal accent).
func NewTheme() fyne.Theme {
	return &vepeenTheme{Theme: theme.DarkTheme()}
}

func (t *vepeenTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return accentColor
	}
	return t.Theme.Color(name, variant)
}
