package ui

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Design tokens from the 2026 redesign brief. Kept together so palette tweaks
// live in one place.
var (
	accentColor   = color.NRGBA{R: 0x2d, G: 0xd4, B: 0xbf, A: 0xff} // teal #2dd4bf
	accentDim     = color.NRGBA{R: 0x4f, G: 0xb3, B: 0xab, A: 0xff} // section label #4fb3ab
	warnColor     = color.NRGBA{R: 0xf5, G: 0xb9, B: 0x4a, A: 0xff} // amber #f5b94a
	dangerColor   = color.NRGBA{R: 0xff, G: 0x8a, B: 0x84, A: 0xff} // #ff8a84
	bgDeep        = color.NRGBA{R: 0x0a, G: 0x0f, B: 0x11, A: 0xff} // #0a0f11
	bgMid         = color.NRGBA{R: 0x0c, G: 0x12, B: 0x14, A: 0xff} // #0c1214
	cardFill      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x08} // rgba(255,255,255,.03)
	cardBorder    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x10} // rgba(255,255,255,.06)
	inputFill     = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x47} // rgba(0,0,0,.28)
	inputBorder   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x14} // rgba(255,255,255,.08)
	textPrimary   = color.NRGBA{R: 0xe8, G: 0xee, B: 0xf0, A: 0xff} // #e8eef0
	textSecondary = color.NRGBA{R: 0xb9, G: 0xc6, B: 0xc9, A: 0xff} // #b9c6c9
	textMuted     = color.NRGBA{R: 0x7d, G: 0x8f, B: 0x94, A: 0xff} // #7d8f94
	textFaint     = color.NRGBA{R: 0x63, G: 0x75, B: 0x7a, A: 0xff} // #63757a
	monoFaint     = color.NRGBA{R: 0x4a, G: 0x5a, B: 0x5e, A: 0xff} // #4a5a5e
	ringIdle      = color.NRGBA{R: 0x4a, G: 0x5a, B: 0x5e, A: 0xff} // disconnected ring
	darkOnAccent  = color.NRGBA{R: 0x05, G: 0x20, B: 0x1d, A: 0xff} // dark text on teal CTA
)

// Custom theme color names for activity-log rows. RichText segments only accept
// a ColorName (not an arbitrary color.Color), so the exact log tokens are exposed
// here and resolved in vepeenTheme.Color.
const (
	colorNameLogTs    fyne.ThemeColorName = "logTs"
	colorNameLogInfo  fyne.ThemeColorName = "logInfo"
	colorNameLogOK    fyne.ThemeColorName = "logOK"
	colorNameLogWarn  fyne.ThemeColorName = "logWarn"
	colorNameLogMuted fyne.ThemeColorName = "logMuted"
)

// sizeNameLog is the 11px font size used by activity-log rows.
const sizeNameLog fyne.ThemeSizeName = "log"

//go:embed fonts/JetBrainsMono-Regular.ttf
var monoRegular []byte

//go:embed fonts/JetBrainsMono-Bold.ttf
var monoBold []byte

var (
	monoRegularRes = &fyne.StaticResource{StaticName: "JetBrainsMono-Regular.ttf", StaticContent: monoRegular}
	monoBoldRes    = &fyne.StaticResource{StaticName: "JetBrainsMono-Bold.ttf", StaticContent: monoBold}
)

type vepeenTheme struct {
	fyne.Theme
}

// NewTheme returns the Vepeen custom theme (dark base + teal accent + design tokens).
func NewTheme() fyne.Theme {
	return &vepeenTheme{Theme: theme.DarkTheme()}
}

func (t *vepeenTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return accentColor
	case theme.ColorNameBackground:
		return bgMid
	case theme.ColorNameForeground:
		return textPrimary
	case theme.ColorNamePlaceHolder:
		return textMuted
	case theme.ColorNameInputBackground:
		return inputFill
	case theme.ColorNameInputBorder:
		// Kill Fyne's heavy pill outline on every entry/select. Inputs read via
		// their dark fill; cards draw their own 1px stroke in card().
		return color.Transparent
	case theme.ColorNameButton:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0d} // rgba(255,255,255,.05)
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x10}
	case theme.ColorNameSeparator:
		return cardBorder
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return bgDeep
	case colorNameLogTs:
		return monoFaint
	case colorNameLogInfo:
		return textSecondary
	case colorNameLogOK:
		return accentColor
	case colorNameLogWarn:
		return warnColor
	case colorNameLogMuted:
		return monoFaint
	}
	return t.Theme.Color(name, variant)
}

// Font serves JetBrains Mono for monospace styles; sans falls through to Fyne default.
func (t *vepeenTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		if style.Bold {
			return monoBoldRes
		}
		return monoRegularRes
	}
	return t.Theme.Font(style)
}

func (t *vepeenTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputBorder:
		return 0 // no pill stroke (see ColorNameInputBorder)
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 11
	case sizeNameLog:
		return 11
	}
	return t.Theme.Size(name)
}
