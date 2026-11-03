//go:build windows

package voz

import (
	"testing"
	"time"
)

// The real screen, through libvpx and back: the size, the colors near the original, and what one frame costs.
func TestVP8RoundTripOfTheScreen(t *testing.T) {
	img, err := capturar()
	if err != nil {
		t.Fatal(err)
	}
	c, err := NuevoCodificadorVP8(img.Bounds().Dx(), img.Bounds().Dy(), 5, 1500)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Cerrar()
	d, err := NuevoDecodificadorVP8()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Cerrar()
	var total time.Duration
	var bytes int
	for i := 0; i < 10; i++ {
		ini := time.Now()
		vp8, clave, err := c.Cuadro(img, i == 0, 200)
		total += time.Since(ini)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && !clave {
			t.Fatal("el primer cuadro no es clave")
		}
		bytes += len(vp8)
		out, err := d.Cuadro(vp8)
		if err != nil || (len(vp8) > 0 && out == nil) {
			t.Fatalf("cuadro %d: %v", i, err)
		}
		if out != nil && i == 9 {
			if out.Bounds() != img.Bounds() {
				t.Fatalf("tamaño %v", out.Bounds())
			}
			var dif int
			for k := 0; k < len(img.Pix); k += 4 * 97 {
				x := int(img.Pix[k]) - int(out.Pix[k])
				if x < 0 {
					x = -x
				}
				dif += x
			}
			prom := float64(dif) / float64(len(img.Pix)/(4*97))
			t.Logf("diferencia media de color %.1f de 255", prom)
			if prom > 12 {
				t.Fatalf("la imagen vuelve muy distinta: %.1f", prom)
			}
		}
	}
	t.Logf("%dx%d · %.1f ms por cuadro · %d KB los 10 cuadros (el primero clave)", img.Bounds().Dx(), img.Bounds().Dy(), float64(total.Microseconds())/10000, bytes/1024)
}
