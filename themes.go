package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// sepiaTheme is a warm parchment-coloured Fyne theme.
type sepiaTheme struct{}

func (sepiaTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 247, G: 238, B: 218, A: 255}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 237, G: 224, B: 200, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 232, G: 215, B: 185, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 210, G: 198, B: 175, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 241, G: 229, B: 205, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 55, G: 38, B: 20, A: 255}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 160, G: 140, B: 110, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 205, G: 185, B: 148, A: 180}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 140, G: 95, B: 45, A: 200}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 140, G: 95, B: 45, A: 255}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 185, G: 163, B: 128, A: 220}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 235, G: 222, B: 198, A: 100}
	}
	// Delegate everything else to the built-in light theme.
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

func (sepiaTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (sepiaTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (sepiaTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// applyTheme switches the app to the named theme ("Light", "Dark", or "Sepia").
func (a *App) applyTheme(name string) {
	switch name {
	case "Dark":
		a.fyneApp.Settings().SetTheme(theme.DarkTheme())
	case "Sepia":
		a.fyneApp.Settings().SetTheme(sepiaTheme{})
	default: // "Light"
		a.fyneApp.Settings().SetTheme(theme.LightTheme())
	}
}
