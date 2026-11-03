package voz

import "testing"

// The track keeps exactly the last samples it can hold, in order, and writing a frame once it is full costs no new
// memory: copying the whole minute on every 20 ms frame is what made a call climb to 38 % of a core.
func TestPistaGuardaElUltimoTramoSinCopiar(t *testing.T) {
	p := nuevaPista(5)
	p.anotar([]int16{1, 2, 3})
	p.anotar([]int16{4, 5, 6, 7})
	if got := p.enOrden(); !iguales(got, []int16{3, 4, 5, 6, 7}) {
		t.Fatalf("enOrden = %v", got)
	}
	p.anotar([]int16{8, 9, 10, 11, 12, 13})
	if got := p.enOrden(); !iguales(got, []int16{9, 10, 11, 12, 13}) {
		t.Fatalf("más largo que la pista: %v", got)
	}

	larga := nuevaPista(48000 * 60)
	cuadro := make([]int16, 960)
	for i := 0; i < 48000*60/960+10; i++ {
		larga.anotar(cuadro)
	}
	if a := testing.AllocsPerRun(100, func() { larga.anotar(cuadro) }); a != 0 {
		t.Fatalf("anotar con la pista llena pide memoria: %v por cuadro", a)
	}
}

func iguales(a, b []int16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
