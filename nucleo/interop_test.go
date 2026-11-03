package nucleo

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"testing"
)

// The fixtures come from the Dart core (herramientas/fixturas.dart): what Dart wrote, Go must read and recompute.
func fixturas(t *testing.T) Msg {
	b, err := os.ReadFile("testdata/fixturas.json")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := Decodificar(string(b))
	if !ok {
		t.Fatal("fixturas ilegibles")
	}
	return m
}

func s(m Msg, k string) string { v, _ := Texto(m, k); return v }

func TestIdentityMatchesDart(t *testing.T) {
	f := fixturas(t)
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	a := DesdeSemilla(seed)
	if a.Publica() != s(f, "a_pub") || a.Exportar() != s(f, "a_secret") {
		t.Fatal("la clave no coincide con Dart")
	}
	if a.Firmar("hola") != s(f, "a_sign_hola") {
		t.Fatal("la firma no coincide con Dart (Ed25519 es determinista)")
	}
	if Hash("x") != s(f, "hash_x") {
		t.Fatal("hash distinto")
	}
	b, _ := Restaurar(s(f, "b_secret"))
	sealed, _ := hex.DecodeString(s(f, "seal_a_to_b"))
	got, ok := b.Abrir(a.Publica(), sealed)
	if !ok || string(got) != "secreto ñ" {
		t.Fatal("no abre el sobre sellado por Dart")
	}
	// And what Go seals, the other side opens.
	mine, _ := a.Sellar(b.Publica(), []byte("de vuelta"))
	if back, ok := b.Abrir(a.Publica(), mine); !ok || string(back) != "de vuelta" {
		t.Fatal("ida y vuelta")
	}
	large, _ := base64.StdEncoding.DecodeString(s(f, "seal_large_a_to_b_b64"))
	big, ok := b.AbrirGrande(a.Publica(), large)
	if !ok || len(big) != Pieza+1000 || big[300] != byte(300%251) || HashBytes(big) != s(f, "photo_hash_big") {
		t.Fatal("no abre el sobre grande de Dart")
	}
}

func TestRoomKeyAndLinesMatchDart(t *testing.T) {
	f := fixturas(t)
	key, _ := base64.StdEncoding.DecodeString(s(f, "room_key_b64"))
	sealed, _ := base64.StdEncoding.DecodeString(s(f, "room_sealed_b64"))
	if p, ok := AbrirSala(key, sealed); !ok || string(p) != "en la sala" {
		t.Fatal("no abre lo sellado con la llave de sala")
	}
	c, ok := LeerCarta(s(f, "charter"))
	if !ok || c.ID != s(f, "charter_id") || c.Name != "Los Pibes ñ" {
		t.Fatal("carta de sala de Dart no válida en Go")
	}
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	if mine := NuevaCarta(DesdeSemilla(seed), "Los Pibes ñ", 1800000000001); mine.Line != c.Line {
		t.Fatalf("la carta de Go difiere:\n%s\n%s", mine.Line, c.Line)
	}
}

func TestCardsEntriesAndChainsMatchDart(t *testing.T) {
	f := fixturas(t)
	cj, _ := Objeto(f, "card")
	c, ok := TarjetaDe(cj)
	if !ok || !c.Verificada() || c.Contenido() != s(f, "card_content") {
		t.Fatal("tarjeta de Dart")
	}
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	if NuevaTarjeta(DesdeSemilla(seed), "Ana", "f", c.Photo, c.At).Sig != c.Sig {
		t.Fatal("la tarjeta de Go firma otro cuerpo")
	}
	ej, _ := Objeto(f, "dir_entry")
	e, ok := EntradaDe(ej)
	if !ok || !e.Verificada() {
		t.Fatal("entrada de Dart")
	}
	if NuevaEntrada(DesdeSemilla(seed), e.T).Sig != e.Sig {
		t.Fatal("la entrada de Go firma otro cuerpo")
	}
	chain := []string{PuertaAbierta, s(f, "door_entry")}
	if !VerificarCadena(chain, s(f, "b_pub"), Fundador) {
		t.Fatal("la entrada por la puerta abierta de Dart no vale en Go")
	}
	b, _ := Restaurar(s(f, "b_secret"))
	n := Peaje(PuertaAbierta, b.Publica())
	if !VerificarCadena([]string{PuertaAbierta, Entrar(b, PuertaAbierta, n, 1800000000003, "pc")}, b.Publica(), Fundador) {
		t.Fatal("la entrada de Go no vale")
	}
	if NickTag(" Ana ") != s(f, "nick_tag_ana") || KeyTag(s(f, "a_pub")) != s(f, "key_tag_a") {
		t.Fatal("etiquetas del tablero")
	}
}

func TestNostrMatchesDart(t *testing.T) {
	f := fixturas(t)
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	k := ClavesNostrDe(DesdeSemilla(seed))
	if k.Priv != s(f, "nostr_priv") || k.Pub != s(f, "nostr_pub") {
		t.Fatal("clave nostr")
	}
	ev, _ := Objeto(f, "nostr_event")
	if idEvento(ev) != s(ev, "id") {
		t.Fatal("el id del evento no se recalcula igual (escapes de JSON)")
	}
	mine := EventoNostr(k, 30078, [][]string{{"d", "zonda"}, {"t", "zd-k-abc"}}, s(ev, "content"), 1800000000)
	if mine["id"] != s(ev, "id") {
		t.Fatal("el evento de Go tiene otro id")
	}
}
