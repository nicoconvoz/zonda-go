//go:build windows

package voz

import "testing"

func TestCapturaDeLaPantalla(t *testing.T) {
	img, err := capturar()
	if err != nil {
		t.Fatal(err)
	}
	var suma int
	for _, v := range img.Pix[:4000] {
		suma += int(v)
	}
	t.Logf("%dx%d, suma de los primeros píxeles %d", img.Bounds().Dx(), img.Bounds().Dy(), suma)
}
