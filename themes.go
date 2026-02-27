package main

import (
	"image/color"

	"gioui.org/widget/material"
)

type themeVariant int

const (
	themeLight themeVariant = iota
	themeDark
	themeSepia
)

// applyTheme switches the active Gio palette.
func (a *App) applyTheme(t themeVariant) {
	switch t {
	case themeDark:
		a.th.Palette = darkPalette()
	case themeSepia:
		a.th.Palette = sepiaPalette()
	default:
		a.th.Palette = material.NewTheme().Palette
	}
	a.window.Invalidate()
}

func darkPalette() material.Palette {
	return material.Palette{
		Bg:         color.NRGBA{R: 30, G: 30, B: 34, A: 255},
		Fg:         color.NRGBA{R: 220, G: 220, B: 220, A: 255},
		ContrastBg: color.NRGBA{R: 70, G: 120, B: 200, A: 255},
		ContrastFg: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	}
}

func sepiaPalette() material.Palette {
	return material.Palette{
		Bg:         color.NRGBA{R: 247, G: 238, B: 218, A: 255},
		Fg:         color.NRGBA{R: 55, G: 38, B: 20, A: 255},
		ContrastBg: color.NRGBA{R: 140, G: 95, B: 45, A: 255},
		ContrastFg: color.NRGBA{R: 255, G: 248, B: 235, A: 255},
	}
}
