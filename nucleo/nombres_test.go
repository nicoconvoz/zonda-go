package nucleo

import "testing"

// A name learned from someone's card is remembered across restarts, so the lists show it before their card arrives
// again (knownName in the app); someone never seen shows as "Miembro" and the start of their key, as in the app.
func TestNombreAprendidoSobreviveAlReinicio(t *testing.T) {
	dir := t.TempDir()
	alm, err := AbrirAlmacen(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := NuevoNodo(alm)
	n.SoloLocal = true
	n.CrearCuenta("Yo", "m", nil)
	otro := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	n.aprenderNombre(otro, "Capitán")
	n.guardarChats()
	n.Alm.Guardar()

	alm2, err := AbrirAlmacen(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := NuevoNodo(alm2)
	if got := m.NombreDe(otro); got != "Capitán" {
		t.Fatalf("después de reiniciar: %q", got)
	}
	if got := m.NombreDe("0123456789ffffff"); got != "Miembro 012345" {
		t.Fatalf("desconocido: %q", got)
	}
}
