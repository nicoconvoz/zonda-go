package nucleo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/nicoconvoz/zonda-go/diag"
)

/* ---------- the Nostr board (net/nostr.dart, adapters/nostr_relays.dart): first contact only ---------- */

const (
	kindCartel  = 30078
	kindSenal   = 21059
	cartelTtlS  = 3600
	salaTtlS    = 600
	cartelCada  = 2 * 60000
	cartelUnido = 30 * 60000
	salaCada    = 5 * 60000
	tarjetaCada = 15 * 60000
)

// The founder's list (relays.txt) starts with these three, and Zonda without a place keeps the first three: every
// device meets on the same ones.
var relaysComunes = []string{"wss://relay.damus.io", "wss://nos.lol", "wss://relay.primal.net", "wss://relay.snort.social"}

type relay struct {
	url     string
	salida  chan string
	abierto bool
}

type Tablero struct {
	n      *Nodo
	claves *ClavesNostr
	mu     sync.Mutex
	relays map[string]*relay
	activo bool

	ruta       map[string]string // member key → its Nostr key
	visto      map[string]bool
	visita     int
	sala       string
	buscando   string
	ultCartel  int64
	ultSala    int64
	salaCartel string
	tarjetaA   map[string]int64
	// Stats: what came and went over the board, for the Red screen.
	Stats map[string]int
}

func nuevoTablero(n *Nodo) *Tablero {
	return &Tablero{n: n, relays: map[string]*relay{}, ruta: map[string]string{}, visto: map[string]bool{}, tarjetaA: map[string]int64{}, Stats: map[string]int{}}
}

func (t *Tablero) contar(k string) { t.Stats[k]++ }

func segs() int64 { return time.Now().Unix() }

// Abierto: at least one relay is connected.
func (t *Tablero) Abierto() bool { return t.Abiertos() > 0 }

func (t *Tablero) Abiertos() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := 0
	for _, r := range t.relays {
		if r.abierto {
			c++
		}
	}
	return c
}

func (t *Tablero) arrancar() {
	if t.activo || t.n.Yo == nil || t.n.SoloLocal {
		return
	}
	t.activo = true
	t.claves = ClavesNostrDe(t.n.Yo)
	for _, u := range relaysComunes[:3] {
		r := &relay{url: u, salida: make(chan string, 256)}
		t.relays[u] = r
		go t.mantener(r)
	}
}

// mantener keeps one relay connected, waiting longer after each failure (5 s, 10 s… 10 min).
func (t *Tablero) mantener(r *relay) {
	diag.Nombrar("tablero")
	espera := 5 * time.Second
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		c, _, err := websocket.Dial(ctx, r.url, nil)
		cancel()
		if err == nil {
			c.SetReadLimit(4 << 20)
			espera = 5 * time.Second
			t.mu.Lock()
			r.abierto = true
			t.mu.Unlock()
			t.n.Hacer(t.alAbrir)
			ctx, stop := context.WithCancel(context.Background())
			go func() {
				diag.Nombrar("tablero-escribir")
				for {
					select {
					case s := <-r.salida:
						if c.Write(ctx, websocket.MessageText, []byte(s)) != nil {
							stop()
							return
						}
					case <-ctx.Done():
						return
					}
				}
			}()
			for {
				_, b, err := c.Read(ctx)
				if err != nil {
					break
				}
				s := string(b)
				t.n.Hacer(func() { t.recibir(s) })
			}
			stop()
			c.CloseNow()
			t.mu.Lock()
			r.abierto = false
			t.mu.Unlock()
			t.n.Hacer(t.n.cambio)
		}
		time.Sleep(espera)
		espera = min(espera*2, 10*time.Minute)
	}
}

func (t *Tablero) enviar(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, r := range t.relays {
		if r.abierto {
			select {
			case r.salida <- s:
			default: // a stuck relay drops what it cannot take; it will reconnect
			}
		}
	}
}

func (t *Tablero) alAbrir() {
	t.suscribir()
	t.ultCartel = 0
	t.latido()
	t.n.cambio()
}

// suscribir: the signals for this device, the beacons of its contacts, and the room on screen (after a reconnection
// the relays forgot every subscription).
func (t *Tablero) suscribir() {
	t.enviar(Codificar([]any{"REQ", "zd-sig", Msg{"kinds": []int{kindSenal}, "#p": []string{t.claves.Pub}, "since": segs() - 120}}))
	t.seguir()
	if r := t.sala; r != "" {
		t.sala = ""
		t.entrarSala(r)
	}
}

// seguir subscribes to the contacts' beacons.
func (t *Tablero) seguir() {
	if t.claves == nil {
		return
	}
	var tags []string
	for k := range t.n.Contactos {
		tags = append(tags, KeyTag(k))
	}
	if len(tags) > 0 {
		t.enviar(Codificar([]any{"REQ", "zd-watch", Msg{"kinds": []int{kindCartel}, "#t": tags, "since": segs() - cartelTtlS}}))
	}
}

// Buscar looks for the member called nick: their beacon arrives as a found member.
func (t *Tablero) Buscar(nick string) {
	if t.claves == nil {
		return
	}
	t.buscando = NickTag(nick)
	t.n.Encontrados = map[string]string{}
	t.enviar(Codificar([]any{"REQ", "zd-find", Msg{"kinds": []int{kindCartel}, "#t": []string{t.buscando}, "since": segs() - cartelTtlS}}))
}

// latido leaves this device's beacon often while it has no link, rarely once linked.
func (t *Tablero) latido() {
	if !t.Abierto() || !t.n.TieneCuenta() {
		return
	}
	cada := int64(cartelCada)
	if t.n.Red.Abiertos() > 0 {
		cada = cartelUnido
	}
	if ahora()-t.ultCartel < cada {
		return
	}
	t.ultCartel = ahora()
	n := t.n
	entry := NuevaEntrada(n.Yo, ahora())
	t.enviar(Codificar([]any{"EVENT", EventoNostr(t.claves, kindCartel, [][]string{
		{"d", "zonda"}, {"t", KeyTag(n.Clave())}, {"t", NickTag(n.Nombre)}, {"expiration", fmt.Sprint(segs() + cartelTtlS)},
	}, Codificar(Msg{"entry": entry.JSON(), "chain": n.Cadena, "nick": n.Nombre, "pv": Protocolo}), segs())}))
}

// sincronizarSala keeps this device announced in its room (or the one it knocks on), so members find it.
func (t *Tablero) sincronizarSala() {
	if !t.Abierto() || t.claves == nil {
		return
	}
	room := t.n.SalaActual
	if g := t.n.salas.Golpe; room == "" && g != nil {
		if t.n.salas.EsMiembro(g.ID, t.n.Clave()) {
			t.n.salas.Golpe = nil
		} else {
			room = g.ID
		}
	}
	if room != t.salaCartel {
		t.salaCartel = room
		t.ultSala = ahora()
		t.entrarSala(room)
	} else if room != "" && ahora()-t.ultSala > salaCada {
		t.ultSala = ahora()
		t.cartelSala()
	}
}

func (t *Tablero) entrarSala(room string) {
	if room != t.sala {
		t.sala = room
		t.visita++
		if room == "" {
			t.enviar(Codificar([]any{"CLOSE", "zd-room"}))
			return
		}
		t.enviar(Codificar([]any{"REQ", "zd-room", Msg{"kinds": []int{kindCartel}, "#t": []string{RoomTag(room)}, "since": segs() - salaTtlS}}))
	}
	if room != "" {
		t.cartelSala()
	}
}

func (t *Tablero) cartelSala() {
	n, r := t.n, t.sala
	if n.Yo == nil || r == "" {
		return
	}
	entry := NuevaEntrada(n.Yo, ahora())
	t.enviar(Codificar([]any{"EVENT", EventoNostr(t.claves, kindCartel, [][]string{
		{"d", "zonda-room"}, {"t", RoomTag(r)}, {"expiration", fmt.Sprint(segs() + salaTtlS)},
	}, Codificar(Msg{"entry": entry.JSON(), "chain": n.Cadena, "room": r, "pv": Protocolo}), segs())}))
}

// EnviarSenal sends a sealed envelope to a member found through a beacon; false when its route is unknown.
func (t *Tablero) EnviarSenal(key string, env Msg) bool {
	to, ok := t.ruta[key]
	if !ok || !t.Abierto() {
		t.contar("señales sin camino")
		return false
	}
	t.contar("señales enviadas")
	c := Copiar(env)
	c["pv"] = Protocolo
	t.enviar(Codificar([]any{"EVENT", EventoNostr(t.claves, kindSenal, [][]string{{"p", to}}, Codificar(c), segs())}))
	return true
}

func tieneTag(tags []any, k, v string) bool {
	for _, x := range tags {
		if l, ok := x.([]any); ok && len(l) > 1 && l[0] == k && (v == "" || l[1] == v) {
			return true
		}
	}
	return false
}

func (t *Tablero) recibir(text string) {
	v, ok := DecodificarValor([]byte(text))
	l, _ := v.([]any)
	if !ok || len(l) < 3 || l[0] != "EVENT" {
		return
	}
	e, ok := l[2].(map[string]any)
	if !ok || e["pubkey"] == t.claves.Pub {
		return
	}
	id, ok := Texto(e, "id")
	if !ok {
		return
	}
	tags, _ := Lista(e, "tags")
	esSala := tieneTag(tags, "d", "zonda-room")
	clave := id
	if esSala {
		clave = fmt.Sprintf("%s@%d", id, t.visita)
	}
	if t.visto[clave] {
		return
	}
	if len(t.visto) > 5000 {
		t.visto = map[string]bool{}
	}
	t.visto[clave] = true
	content, ok := Decodificar(Interpolar(e["content"]))
	if !ok {
		return
	}
	kind, _ := Entero(e, "kind")
	if pv, ok := Entero(content, "pv"); !ok || pv < Protocolo {
		return
	}
	pub := Interpolar(e["pubkey"])
	switch kind {
	case kindCartel:
		ej, ok := Objeto(content, "entry")
		if !ok {
			return
		}
		entry, ok := EntradaDe(ej)
		if !ok || !entry.Verificada() || entry.Key == t.n.Clave() {
			return
		}
		t.ruta[entry.Key] = pub
		n := t.n
		if room, ok := Texto(content, "room"); ok {
			if room == t.sala {
				t.contar("carteles de sala")
				if n.EsMiembro(entry.Key, content["chain"]) {
					n.EnviarTarjeta(entry.Key, true)
					n.Red.MarcarYa(entry.Key)
				}
			}
			return
		}
		if t.buscando != "" && tieneTag(tags, "t", t.buscando) {
			nick, _ := Texto(content, "nick")
			n.Encontrados[entry.Key] = nick
			n.EnviarTarjeta(entry.Key, true)
			n.Red.MarcarYa(entry.Key)
			n.cambio()
			return
		}
		n.OidoDe(entry.Key, entry.T)
		if ahora()-t.tarjetaA[entry.Key] >= tarjetaCada {
			t.tarjetaA[entry.Key] = ahora()
			n.EnviarTarjeta(entry.Key, true)
		}
		if n.Contactos[entry.Key] {
			n.Red.MarcarYa(entry.Key)
		}
		n.cambio()
	case kindSenal:
		sf, _ := Texto(content, "sf")
		st, _ := Texto(content, "st")
		at, okat := Entero(content, "at")
		box, _ := Texto(content, "box")
		sig, _ := Texto(content, "sig")
		if sf != "" && okat && Verificar(sf, fmt.Sprintf("SG2|%s|%s|%d|%s", sf, st, at, box), sig) {
			t.ruta[sf] = pub
		}
		t.contar("señales recibidas")
		t.n.RecibirSobre(content)
	}
}

// Conectado: whether at least one board relay is open (for the Red screen, which names none of them).
func (t *Tablero) Conectado() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, r := range t.relays {
		if r.abierto {
			return true
		}
	}
	return false
}
