package interfaz

import (
	"image/color"
	"testing"
)

// An icon drawn once as an image: ink on the line, nothing far from it, and the same picture every time it is asked
// for (the cost was redoing the strokes on every frame).
func TestIconoRasterizado(t *testing.T) {
	tinta := color.NRGBA{0xff, 0x40, 0xd0, 0xff}
	img := rasterIcono("check", 48, tinta)
	if b := img.Bounds(); b.Dx() != 48 || b.Dy() != 48 {
		t.Fatalf("tamaño %v", b)
	}
	// "check" is m5 12.5 4.5 4.5L19 7: the corner at (9.5,17) of 24 is on the line; (2,2) is far from it.
	if a := img.RGBAAt(19, 34).A; a < 0xc0 {
		t.Fatalf("sobre la línea casi no hay tinta: alfa %d", a)
	}
	if a := img.RGBAAt(4, 4).A; a != 0 {
		t.Fatalf("lejos de la línea hay tinta: alfa %d", a)
	}
	r := img.RGBAAt(19, 34)
	if r.R < r.G || r.B < r.G {
		t.Fatalf("color %v no es la tinta", r)
	}

	antes := rasterizados
	iconoImagen("check", 48, tinta)
	iconoImagen("check", 48, tinta)
	if rasterizados-antes > 1 {
		t.Fatalf("se dibujó %d veces", rasterizados-antes)
	}
}
