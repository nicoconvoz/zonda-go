package nucleo

import "testing"

// Two members with the same nickname keep one order, whatever order they come in (the list is built from a map).
func TestOrdenarMiembrosEsTotal(t *testing.T) {
	a := Miembro{Clave: "aaaa", Nombre: "Capitán", EnLinea: true}
	b := Miembro{Clave: "bbbb", Nombre: "capitán", EnLinea: true}
	c := Miembro{Clave: "cccc", Nombre: "Ana", EnLinea: false}
	for _, entrada := range [][]Miembro{{a, b, c}, {b, a, c}, {c, b, a}, {c, a, b}} {
		l := append([]Miembro(nil), entrada...)
		ordenarMiembros(l)
		if l[0].Clave != "aaaa" || l[1].Clave != "bbbb" || l[2].Clave != "cccc" {
			t.Fatalf("orden %v desde %v", []string{l[0].Clave, l[1].Clave, l[2].Clave}, entrada)
		}
	}
}
