package nucleo

import (
	"os"
	"testing"
	"time"
)

// Over the real public relays (opt-in: ZONDA_RED=1): two devices that never met find each other only through the
// board, the newcomer knocks with the room code, is let in, and a sealed line reaches it over a WebRTC link.
func TestRoomOverTheRealBoard(t *testing.T) {
	if os.Getenv("ZONDA_RED") != "1" {
		t.Skip("prueba contra relays públicos: ZONDA_RED=1")
	}
	mk := func(nombre string) *Nodo {
		alm, _ := AbrirAlmacen(t.TempDir())
		n := NuevoNodo(alm)
		n.CrearCuenta(nombre, "m", nil)
		go n.Correr()
		return n
	}
	a, b := mk("PruebaA"), mk("PruebaB")
	lento := func(n *Nodo, que string, ok func() bool) {
		t.Helper()
		for i := 0; i < 600; i++ {
			var listo bool
			n.Consultar(func() { listo = ok() })
			if listo {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		var st, tb map[string]int
		n.Consultar(func() { st, tb = n.Red.Stats, n.Tablero.Stats })
		t.Fatalf("nunca pasó: %s\nred %v\ntablero %v", que, st, tb)
	}
	lento(a, "A conectado al tablero", func() bool { return a.Tablero.Abierto() })
	lento(b, "B conectado al tablero", func() bool { return b.Tablero.Abierto() })
	var c Carta
	a.Consultar(func() { c = a.Salas().Crear("Prueba de red"); a.EntrarASala(c.ID) })
	var code string
	a.Consultar(func() { code = a.Salas().Codigo(c.ID) })
	carta, _ := CartaDeCodigo(code)
	b.Consultar(func() { b.Salas().Golpear(carta) })
	lento(a, "a A le llega el pedido", func() bool { return len(a.Salas().Pedidos(c.ID)) == 1 })
	a.Consultar(func() { a.Salas().Aprobar(c.ID, b.Clave()) })
	lento(b, "B queda adentro", func() bool { return b.Salas().EsMiembro(c.ID, b.Clave()) })
	b.Consultar(func() { b.EntrarASala(c.ID) })
	lento(a, "A ve a B en la sala", func() bool {
		for _, k := range a.EnSala(c.ID) {
			if k == b.Clave() {
				return true
			}
		}
		return false
	})
	a.Consultar(func() { a.DecirEnSala("hola por internet", nil, nil) })
	lento(b, "la línea llega", func() bool {
		for _, l := range b.LineasSala {
			if l.Texto == "hola por internet" {
				return true
			}
		}
		return false
	})
	var st map[string]int
	a.Consultar(func() { st = a.Red.Stats })
	t.Logf("red de A: %v", st)
}
