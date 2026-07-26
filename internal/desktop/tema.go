package desktop

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// temaLiz es el tema nativo de Liz. Hereda del tema default de Fyne
// y pinta con la paleta morada característica del proyecto.
//
// La paleta está alineada con la del frontend web original (web/tailwind.config.js):
//   - primario: morado #7c3aed (violet-600)
//   - acento:   cyan   #06b6d4 (cyan-500)
//   - fondo dark:  #0f0f14 (morado muy oscuro)
//   - fondo light: #fafafa
type temaLiz struct {
	fallback fyne.Theme
	modo     ModoTema
	vars     paleta
}

// ModoTema selecciona el esquema de color.
type ModoTema int

const (
	// ModoOscuro tema por defecto (estilo ChatGPT).
	ModoOscuro ModoTema = iota
	// ModoClaro tema alternativo.
	ModoClaro
)

// paleta contiene los colores base para un modo.
type paleta struct {
	fondo          color.Color
	fondoPanel     color.Color
	fondoInput     color.Color
	primario       color.Color
	primarioHover  color.Color
	accent         color.Color
	texto          color.Color
	textoSec       color.Color
	borde          color.Color
	exito          color.Color
	error          color.Color
}

var paletaOscuro = paleta{
	fondo:         &color.NRGBA{R: 0x0f, G: 0x0f, B: 0x14, A: 0xff},
	fondoPanel:    &color.NRGBA{R: 0x1a, G: 0x1a, B: 0x22, A: 0xff},
	fondoInput:    &color.NRGBA{R: 0x22, G: 0x22, B: 0x2c, A: 0xff},
	primario:      &color.NRGBA{R: 0x7c, G: 0x3a, B: 0xed, A: 0xff},
	primarioHover: &color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff},
	accent:        &color.NRGBA{R: 0x06, G: 0xb6, B: 0xd4, A: 0xff},
	texto:         &color.NRGBA{R: 0xf3, G: 0xf4, B: 0xf6, A: 0xff},
	textoSec:      &color.NRGBA{R: 0x9c, G: 0xa3, B: 0xaf, A: 0xff},
	borde:         &color.NRGBA{R: 0x37, G: 0x41, B: 0x51, A: 0xff},
	exito:         &color.NRGBA{R: 0x10, G: 0xb9, B: 0x81, A: 0xff},
	error:         &color.NRGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff},
}

var paletaClaro = paleta{
	fondo:         &color.NRGBA{R: 0xfa, G: 0xfa, B: 0xfa, A: 0xff},
	fondoPanel:    &color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	fondoInput:    &color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	primario:      &color.NRGBA{R: 0x7c, G: 0x3a, B: 0xed, A: 0xff},
	primarioHover: &color.NRGBA{R: 0x6d, G: 0x28, B: 0xd9, A: 0xff},
	accent:        &color.NRGBA{R: 0x06, G: 0xb6, B: 0xd4, A: 0xff},
	texto:         &color.NRGBA{R: 0x1f, G: 0x29, B: 0x37, A: 0xff},
	textoSec:      &color.NRGBA{R: 0x6b, G: 0x72, B: 0x80, A: 0xff},
	borde:         &color.NRGBA{R: 0xe5, G: 0xe7, B: 0xeb, A: 0xff},
	exito:         &color.NRGBA{R: 0x05, G: 0x96, B: 0x69, A: 0xff},
	error:         &color.NRGBA{R: 0xdc, G: 0x26, B: 0x26, A: 0xff},
}

// NuevoTema crea un tema Liz con el modo especificado.
func NuevoTema(modo ModoTema) fyne.Theme {
	t := &temaLiz{fallback: theme.DefaultTheme(), modo: modo}
	if modo == ModoOscuro {
		t.vars = paletaOscuro
	} else {
		t.vars = paletaClaro
	}
	return t
}

// Modo devuelve el modo actual.
func (t *temaLiz) Modo() ModoTema { return t.modo }

// Color implementa fyne.Theme. Devuelve colores Liz para los nombres canónicos.
func (t *temaLiz) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return t.vars.fondo
	case theme.ColorNameButton:
		return t.vars.fondoPanel
	case theme.ColorNameInputBackground:
		return t.vars.fondoInput
	case theme.ColorNameForeground:
		return t.vars.texto
	case theme.ColorNameDisabled:
		return t.vars.textoSec
	case theme.ColorNamePlaceHolder:
		return t.vars.textoSec
	case theme.ColorNamePrimary:
		return t.vars.primario
	case theme.ColorNameHover:
		return t.vars.primarioHover
	case theme.ColorNamePressed:
		return t.vars.primarioHover
	case theme.ColorNameSelection:
		return t.vars.primario
	case theme.ColorNameFocus:
		return t.vars.primario
	case theme.ColorNameSeparator:
		return t.vars.borde
	case theme.ColorNameInputBorder:
		return t.vars.borde
	case theme.ColorNameInnerWindowBorder:
		return t.vars.borde
	case theme.ColorNameError:
		return t.vars.error
	case theme.ColorNameSuccess:
		return t.vars.exito
	case theme.ColorNameHeaderBackground:
		return t.vars.fondoPanel
	case theme.ColorNameOverlayBackground:
		return t.vars.fondoPanel
	case theme.ColorNameMenuBackground:
		return t.vars.fondoPanel
	case theme.ColorNameForegroundOnPrimary:
		return color.White
	case theme.ColorNameForegroundOnSuccess:
		return color.White
	case theme.ColorNameForegroundOnError:
		return color.White
	case theme.ColorNameScrollBar:
		return t.vars.textoSec
	case theme.ColorNameScrollBarBackground:
		return t.vars.fondoPanel
	}
	return t.fallback.Color(name, v)
}

// Size implementa fyne.Theme. Usa tamaños un poco más compactos que el default.
func (t *temaLiz) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return t.fallback.Size(name)
}

// Font implementa fyne.Theme. Usa la fuente default de Fyne.
func (t *temaLiz) Font(style fyne.TextStyle) fyne.Resource {
	return t.fallback.Font(style)
}

// Icon implementa fyne.Theme. Usa los iconos del tema default.
func (t *temaLiz) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.fallback.Icon(name)
}
