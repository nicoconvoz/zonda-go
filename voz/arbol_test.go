package voz

import (
	"fmt"
	"testing"
)

func TestTreeReachesEveryoneOnceWithBoundedUpload(t *testing.T) {
	var ms []string
	for i := 0; i < 13; i++ {
		ms = append(ms, fmt.Sprintf("%064x", i+1))
	}
	src := ms[0]
	primeros, hijos := Arbol(src, ms)
	if len(primeros) != Ramas {
		t.Fatalf("primer nivel: %d", len(primeros))
	}
	oyen := map[string]int{}
	for _, p := range primeros {
		oyen[p]++
		if len(hijos[p]) > Ramas {
			t.Fatalf("%s reenvía %d copias: más de %d", p, len(hijos[p]), Ramas)
		}
		for _, h := range hijos[p] {
			oyen[h]++
			if len(hijos[h]) != 0 {
				t.Fatal("un solo salto extra: los del segundo nivel no reenvían")
			}
		}
	}
	if len(oyen) != len(ms)-1 {
		t.Fatalf("oyen %d de %d", len(oyen), len(ms)-1)
	}
	for k, n := range oyen {
		if n != 1 || k == src {
			t.Fatalf("%s oye %d veces", k, n)
		}
	}
	// Every device computes the same tree, whatever order its list is in.
	rev := append([]string(nil), ms...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	p2, _ := Arbol(src, rev)
	if fmt.Sprint(p2) != fmt.Sprint(primeros) {
		t.Fatal("el árbol depende del orden de la lista")
	}
}

func TestVoiceFrameRoundTrip(t *testing.T) {
	src := fmt.Sprintf("%064x", 42)
	b := armarCuadro("sala/General", src, 7, []byte{1, 2, 3})
	c, s, seq, mu, ok := leerCuadro(b)
	if !ok || c != huellaCanal("sala/General") || s != src || seq != 7 || len(mu) != 3 {
		t.Fatal("cuadro")
	}
}
