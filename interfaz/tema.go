package interfaz

import (
	"embed"
	"image/color"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
)

// The app's skins (app/packages/zonda_ui/lib/src/tokens/ib_skins.dart), with their own fonts. The "Zonda" skin of
// colored shapes is not brought over (Captain's decision): these five are.

//go:embed fuentes/*.ttf
var fuentesTTF embed.FS

type Piel struct {
	ID, Nombre, Origen                         string
	AppBg, Surface, Surface2, Ink, Muted, Line color.NRGBA
	Bar, BarInk, BarActive                     color.NRGBA
	Btn, BtnLine, BtnInk, On, OnInk            color.NRGBA
	Dark, DarkInk                              color.NRGBA
	Radio, RadioChico                          float32
	Fuente                                     font.Typeface
	Borde                                      *color.NRGBA
	Sombra                                     color.NRGBA // the drop under cards (offset, no blur)
	SombraDY                                   float32
	Muestra                                    [4]color.NRGBA
	Light, LightInk                            color.NRGBA // the light card (women's rows), as Dark is the men's
	Brillo                                     bool        // the shadow is a glow around the box (neon), not a drop
}

func hx(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: uint8(v >> 24)}
}

func borde(v uint32) *color.NRGBA { c := hx(v); return &c }

var pieles = []Piel{
	{
		ID: "tinta", Light: hx(0xFFFFFFFF), LightInk: hx(0xFF121212), Nombre: "Tinta", Origen: "Base · blanco y negro",
		AppBg: hx(0xFFE8E8E5), Surface: hx(0xFFFFFFFF), Surface2: hx(0xFFF1F1EE), Ink: hx(0xFF121212), Muted: hx(0xFF6B6B67), Line: hx(0xFFD8D8D3),
		Bar: hx(0xFF8D8D89), BarInk: hx(0xFFFFFFFF), BarActive: hx(0xFF111111),
		Btn: hx(0xFFFFFFFF), BtnLine: hx(0xFFCFCFCA), BtnInk: hx(0xFF1A1A1A), On: hx(0xFF121212), OnInk: hx(0xFFFFFFFF),
		Dark: hx(0xFF121212), DarkInk: hx(0xFFFFFFFF),
		Radio: 14, RadioChico: 10, Fuente: "Figtree", Sombra: hx(0x2E000000), SombraDY: 3,
		Muestra: [4]color.NRGBA{hx(0xFF121212), hx(0xFFFFFFFF), hx(0xFF8D8D89), hx(0xFFE8E8E5)},
	},
	{
		ID: "glaciar", Light: hx(0xFFFFFFFF), LightInk: hx(0xFF173B55), Nombre: "Glaciar", Origen: "Tesoro",
		AppBg: hx(0xFFDDE8EF), Surface: hx(0xFFF7FBFD), Surface2: hx(0xFFE9F1F6), Ink: hx(0xFF14324A), Muted: hx(0xFF5B7489), Line: hx(0xFFC5D6E2),
		Bar: hx(0xFF24506E), BarInk: hx(0xFF9EC3DC), BarActive: hx(0xFFFFFFFF),
		Btn: hx(0xFFF7FBFD), BtnLine: hx(0xFFBCD0DE), BtnInk: hx(0xFF24506E), On: hx(0xFF1B4562), OnInk: hx(0xFFEAF6FF),
		Dark: hx(0xFF173B55), DarkInk: hx(0xFFEAF5FB),
		Radio: 18, RadioChico: 13, Fuente: "Sora", Sombra: hx(0x2E000000), SombraDY: 3,
		Muestra: [4]color.NRGBA{hx(0xFF173B55), hx(0xFFF7FBFD), hx(0xFF24506E), hx(0xFFDDE8EF)},
	},
	{
		ID: "arrecife", Light: hx(0xFFFFFAF6), LightInk: hx(0xFF1D4F4F), Nombre: "Arrecife", Origen: "Skin de tesoro",
		AppBg: hx(0xFFFDE8DC), Surface: hx(0xFFFFFAF6), Surface2: hx(0xFFFDEEE5), Ink: hx(0xFF2B2320), Muted: hx(0xFF8B6F63), Line: hx(0xFFF1CDBB),
		Bar: hx(0xFFE2654D), BarInk: hx(0xFFFFD9CC), BarActive: hx(0xFF1D4F4F),
		Btn: hx(0xFFFFFAF6), BtnLine: hx(0xFFF0C4B1), BtnInk: hx(0xFFC4503B), On: hx(0xFF1D4F4F), OnInk: hx(0xFFFFF4EC),
		Dark: hx(0xFF1D4F4F), DarkInk: hx(0xFFFFF4EC),
		Radio: 24, RadioChico: 16, Fuente: "Fredoka", Sombra: hx(0x45C4503B), SombraDY: 4,
		Muestra: [4]color.NRGBA{hx(0xFF1D4F4F), hx(0xFFFFFAF6), hx(0xFFE2654D), hx(0xFFFDE8DC)},
	},
	{
		ID: "pergamino", Light: hx(0xFFF7EED8), LightInk: hx(0xFF3D2B1A), Nombre: "Pergamino", Origen: "Skin base · papel antiguo",
		AppBg: hx(0xFFE9DCBF), Surface: hx(0xFFF5ECD5), Surface2: hx(0xFFECE0C2), Ink: hx(0xFF3A2A19), Muted: hx(0xFF7B6447), Line: hx(0xFFCDB88F),
		Bar: hx(0xFF4B3520), BarInk: hx(0xFFCDB184), BarActive: hx(0xFFF6E7C3),
		Btn: hx(0xFFF5ECD5), BtnLine: hx(0xFFBEA475), BtnInk: hx(0xFF4B3520), On: hx(0xFF4B3520), OnInk: hx(0xFFF6E7C3),
		Dark: hx(0xFF3D2B1A), DarkInk: hx(0xFFF4E5C3),
		Radio: 5, RadioChico: 4, Fuente: "IM Fell English", Borde: borde(0x734B3520), Sombra: hx(0x404B3520), SombraDY: 3,
		Muestra: [4]color.NRGBA{hx(0xFF3D2B1A), hx(0xFFF5ECD5), hx(0xFF4B3520), hx(0xFFE9DCBF)},
	},
	{
		ID: "neon", Light: hx(0xFF2A2346), LightInk: hx(0xFFF3EDFF), Nombre: "Neón 86", Origen: "Skin base · neón",
		AppBg: hx(0xFF120F1F), Surface: hx(0xFF1C1830), Surface2: hx(0xFF231E3B), Ink: hx(0xFFEFE8FF), Muted: hx(0xFF9A8FC9), Line: hx(0xFF3A3263),
		Bar: hx(0xFF0B0916), BarInk: hx(0xFF6656A8), BarActive: hx(0xFFFF4FD8),
		Btn: hx(0xFF1C1830), BtnLine: hx(0xFF3A3263), BtnInk: hx(0xFFB9ADF0), On: hx(0xFFFF4FD8), OnInk: hx(0xFF140B1F),
		Dark: hx(0xFF0B0916), DarkInk: hx(0xFFF3EDFF),
		Radio: 6, RadioChico: 5, Fuente: "Chakra Petch", Borde: borde(0x55FF4FD8), Sombra: hx(0xFFFF4FD8), SombraDY: 2, Brillo: true,
		Muestra: [4]color.NRGBA{hx(0xFF0B0916), hx(0xFFFF4FD8), hx(0xFF2A2346), hx(0xFF120F1F)},
	},
}

// pielDe: the skin chosen on this device; Neón 86 when none was chosen yet (the Captain's).
func pielDe(id string) Piel {
	if id == "" {
		id = "neon"
	}
	for _, p := range pieles {
		if p.ID == id {
			return p
		}
	}
	return pieles[0]
}

// colecciónFuentes loads the skins' fonts; each face is named after its family, so a skin picks it by name.
func coleccionFuentes() []font.FontFace {
	var out []font.FontFace
	dir, _ := fuentesTTF.ReadDir("fuentes")
	for _, e := range dir {
		b, err := fuentesTTF.ReadFile("fuentes/" + e.Name())
		if err != nil {
			continue
		}
		faces, err := opentype.ParseCollection(b)
		if err != nil {
			continue
		}
		out = append(out, faces...)
	}
	return out
}

var _ = text.Start
