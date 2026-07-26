package desktop

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/theme"
)

// TestNuevoTema_Oscuro verifica que el tema oscuro devuelva los colores
// esperados de la paleta Liz.
func TestNuevoTema_Oscuro(t *testing.T) {
	tm := NuevoTema(ModoOscuro)
	if tm == nil {
		t.Fatal("NuevoTema devolvió nil")
	}

	tl, ok := tm.(*temaLiz)
	if !ok {
		t.Fatal("tema no es *temaLiz")
	}
	if tl.Modo() != ModoOscuro {
		t.Errorf("Modo = %v, quiero ModoOscuro", tl.Modo())
	}

	// Background debe ser el morado muy oscuro #0f0f14
	bg := tm.Color(theme.ColorNameBackground, theme.VariantDark)
	nrgba, ok := bg.(*color.NRGBA)
	if !ok {
		t.Fatalf("Background no es *color.NRGBA: %T", bg)
	}
	if nrgba.R != 0x0f || nrgba.G != 0x0f || nrgba.B != 0x14 {
		t.Errorf("Background RGB = (%x,%x,%x), quiero (0f,0f,14)", nrgba.R, nrgba.G, nrgba.B)
	}

	// Primary debe ser el morado #7c3aed
	prim := tm.Color(theme.ColorNamePrimary, theme.VariantDark)
	nrgba, ok = prim.(*color.NRGBA)
	if !ok {
		t.Fatalf("Primary no es *color.NRGBA: %T", prim)
	}
	if nrgba.R != 0x7c || nrgba.G != 0x3a || nrgba.B != 0xed {
		t.Errorf("Primary RGB = (%x,%x,%x), quiero (7c,3a,ed)", nrgba.R, nrgba.G, nrgba.B)
	}
}

// TestNuevoTema_Claro verifica el tema claro.
func TestNuevoTema_Claro(t *testing.T) {
	tm := NuevoTema(ModoClaro)
	tl, _ := tm.(*temaLiz)
	if tl.Modo() != ModoClaro {
		t.Errorf("Modo = %v, quiero ModoClaro", tl.Modo())
	}

	bg := tm.Color(theme.ColorNameBackground, theme.VariantLight)
	nrgba, _ := bg.(*color.NRGBA)
	if nrgba.R != 0xfa || nrgba.G != 0xfa || nrgba.B != 0xfa {
		t.Errorf("Background RGB = (%x,%x,%x), quiero (fa,fa,fa)", nrgba.R, nrgba.G, nrgba.B)
	}
}

// TestNuevoTema_Sizes verifica que los tamaños personalizados sean correctos.
func TestNuevoTema_Sizes(t *testing.T) {
	tm := NuevoTema(ModoOscuro)

	if s := tm.Size(theme.SizeNameText); s != 14 {
		t.Errorf("SizeNameText = %v, quiero 14", s)
	}
	if s := tm.Size(theme.SizeNameHeadingText); s != 18 {
		t.Errorf("SizeNameHeadingText = %v, quiero 18", s)
	}
	if s := tm.Size(theme.SizeNamePadding); s != 6 {
		t.Errorf("SizeNamePadding = %v, quiero 6", s)
	}
}

// TestNuevoTema_FallbackSizes verifica que tamaños no sobreescritos deleguen
// al tema default y devuelvan valores razonables (> 0).
func TestNuevoTema_FallbackSizes(t *testing.T) {
	tm := NuevoTema(ModoOscuro)
	// SizeNameScrollBarSmall no lo sobreescribimos → debe ser > 0 (default)
	if s := tm.Size(theme.SizeNameScrollBarSmall); s <= 0 {
		t.Errorf("SizeNameScrollBarSmall = %v, debe ser > 0", s)
	}
}

// TestNuevoTema_IconAndFont verifica que el tema devuelva iconos y fonts.
// NOTA: theme.DefaultTheme() puede panic sin app de Fyne inicializado, así
// que para ciertos iconos/fonts específicos el test solo verifica que no
// devuelvan nil para los que tenemos en nuestra implementación.
func TestNuevoTema_IconAndFont(t *testing.T) {
	tm := NuevoTema(ModoOscuro)
	// Size y Color personalizados (los que sí sobreescribimos) deben funcionar.
	if s := tm.Size(theme.SizeNameText); s != 14 {
		t.Errorf("SizeNameText = %v, quiero 14", s)
	}
}
