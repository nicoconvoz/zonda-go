package nucleo

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func ahora() int64 { return time.Now().UnixMilli() }

// Persona is someone this device heard of: their signed card, when, and which room they say they are in.
type Persona struct {
	Tarjeta Tarjeta
	Visto   int64
	Sala    string
	Nick    string
}

// Nodo is one Zonda device: presence, sealed signals, photos, rooms with an owner, private chats, Internet links and
// the Nostr board. Every change runs on one goroutine (Hacer), so its state needs no locks; network callbacks post
// their work there.
type Nodo struct {
	Alm    *Almacen
	Router *Router
	Yo     *Identidad
	Cadena []string
	Nombre string
	Genero string
	MiFoto string

	ops      chan func()
	OnCambio func()

	personas       map[string]*Persona
	via            map[string]string
	beaconAt       map[string]int64
	confiables     map[string]bool
	dispositivos   map[string]string
	helloDicho     string
	tarjetaCache   *Tarjeta
	cardSentTo     map[string]int64
	sigVisto       map[string]bool
	fotoRespondida map[string]int64
	fotoEspera     map[string]map[string]bool

	// SalaActual is the room on screen (its id); its lines live in LineasSala while inside.
	SalaActual string
	LineasSala []LineaSala
	aperturas  map[string][]byte

	salas *Salas

	Chats     map[string][]LineaChat
	Contactos map[string]bool

	Red     *Red
	Tablero *Tablero
	// Encontrados: members found by the magnifier (key → nickname).
	Encontrados map[string]string
	nombres     map[string]string
	// OnSenal hands the signals the core does not handle (voice "v-…") to the app.
	OnSenal func(de string, d Msg)
	// SoloLocal keeps the board off (tests: no public relays).
	SoloLocal bool
}

// LineaSala is one line of a room's chat.
type LineaSala struct {
	Sala, Por, Nick, Genero, Texto string
	At                             int64
	Sistema                        bool
	Foto                           string
	Archivo                        Msg
	Epoca                          int64 // -1: not sealed
}

// LineaChat is one line of a private chat.
type LineaChat struct {
	Texto   string
	Mia     bool
	At      int64
	Foto    string
	Archivo Msg
	ID      string
	Estado  string // sending | ok | fail ("" for received)
}

func NuevoNodo(alm *Almacen) *Nodo {
	n := &Nodo{
		Alm: alm, ops: make(chan func(), 1024),
		personas: map[string]*Persona{}, via: map[string]string{}, beaconAt: map[string]int64{},
		confiables: map[string]bool{}, dispositivos: map[string]string{}, cardSentTo: map[string]int64{},
		sigVisto: map[string]bool{}, fotoRespondida: map[string]int64{}, fotoEspera: map[string]map[string]bool{},
		aperturas: map[string][]byte{}, Chats: map[string][]LineaChat{}, Contactos: map[string]bool{}, Encontrados: map[string]string{}, nombres: map[string]string{},
	}
	n.cargarCuenta()
	self := "pc-" + hex.EncodeToString(aleatorio(4))
	n.Router = NuevoRouter(self, func(to, js string) { n.Red.Enviar(to, js) })
	n.salas = nuevasSalas(n)
	n.Red = nuevaRed(n)
	n.Tablero = nuevoTablero(n)
	r := n.Router
	r.On("hello", n.alHola)
	r.On("bye", n.alChau)
	r.On("gone", n.alIdos)
	r.On("room", n.alSala)
	r.On("photoreq", n.alPedidoFoto)
	r.On("photo", n.alFoto)
	r.On("sig", func(m Msg, _ string) { n.alSobre(m) })
	r.On("dm", func(m Msg, _ string) { n.alDm(m) })
	r.AlAgregar(func(string, bool) { n.helloDicho = ""; n.Latido() })
	r.AlQuitar(n.alPerderEnlace)
	r.OnLegacy = func(v string) { n.Red.legacyVecino(v) }
	n.cargarChats()
	return n
}

func aleatorio(k int) []byte {
	b := make([]byte, k)
	rand.Read(b)
	return b
}

// Correr is the node's loop: it runs every posted change, plus the beat every 20 s (cheap when nothing changed:
// a hello goes out only when what it says changed).
func (n *Nodo) Correr() {
	n.Tablero.arrancar()
	tic := time.NewTicker(3 * time.Second)
	beat := time.NewTicker(20 * time.Second)
	n.latidoLargo()
	for {
		select {
		case f := <-n.ops:
			f()
		case <-tic.C:
			n.Red.vencidos()
			n.Tablero.sincronizarSala()
		case <-beat.C:
			n.latidoLargo()
		}
	}
}

func (n *Nodo) latidoLargo() {
	n.Latido()
	n.Red.Tick()
	n.Tablero.latido()
	n.cambio()
}

// Hacer runs f on the node's loop.
func (n *Nodo) Hacer(f func()) { n.ops <- f }

// Consultar runs f on the node's loop and waits for it (the screen reads state this way).
func (n *Nodo) Consultar(f func()) {
	done := make(chan struct{})
	n.ops <- func() { f(); close(done) }
	<-done
}

func (n *Nodo) cambio() {
	if n.OnCambio != nil {
		n.OnCambio()
	}
}

/* ---------- account ---------- */

func (n *Nodo) cargarCuenta() {
	if s := n.Alm.Leer("secreto"); s != "" {
		if id, err := Restaurar(s); err == nil {
			n.Yo = id
			n.Cadena = Textos(func() any { v, _ := n.Alm.LeerJSON("cadena"); return v }())
		}
	}
	n.Nombre, n.Genero, n.MiFoto = n.Alm.Leer("nombre"), n.Alm.Leer("genero"), n.Alm.Leer("mifoto")
}

func (n *Nodo) TieneCuenta() bool { return n.Yo != nil && n.Nombre != "" }

// CrearCuenta joins as the founder's direct invitee through the open door (a new key, the toll paid here), with
// only a nickname, man/woman and an optional photo.
func (n *Nodo) CrearCuenta(nombre, genero string, foto []byte) {
	if n.Yo == nil {
		id := NuevaIdentidad()
		dev := n.Alm.Leer("dispositivo")
		if dev == "" {
			dev = hex.EncodeToString(aleatorio(16))
			n.Alm.Escribir("dispositivo", dev)
		}
		nonce := Peaje(PuertaAbierta, id.Publica())
		n.Cadena = []string{PuertaAbierta, Entrar(id, PuertaAbierta, nonce, ahora(), dev)}
		n.Yo = id
		n.Alm.Escribir("secreto", id.Exportar())
		n.Alm.EscribirJSON("cadena", n.Cadena)
	}
	n.Nombre, n.Genero = strings.TrimSpace(nombre), genero
	if foto != nil {
		n.MiFoto = n.guardarFoto(foto)
	}
	n.Alm.Escribir("nombre", n.Nombre)
	n.Alm.Escribir("genero", n.Genero)
	n.Alm.Escribir("mifoto", n.MiFoto)
	n.Alm.Guardar()
	n.helloDicho = ""
	n.Latido()
	n.Tablero.arrancar()
	n.cambio()
}

func (n *Nodo) Clave() string {
	if n.Yo == nil {
		return ""
	}
	return n.Yo.Publica()
}

/* ---------- membership ---------- */

// EsMiembro: key proves a chain back to the founder; through the door, one account per device (the first seen wins).
func (n *Nodo) EsMiembro(key string, chain any) bool {
	if key == Fundador || n.confiables[key] {
		return true
	}
	c := Textos(chain)
	if !VerificarCadena(c, key, Fundador) {
		return false
	}
	if len(c) == 2 && strings.HasPrefix(c[0], "OD1 ") {
		dev := DispositivoDe(c[1])
		if had, ok := n.dispositivos[dev]; dev == "" || (ok && had != key) {
			return false
		}
		n.dispositivos[dev] = key
	}
	n.confiables[key] = true
	return true
}

/* ---------- presence ---------- */

func (n *Nodo) miTarjeta() Tarjeta {
	if c := n.tarjetaCache; c != nil && c.Name == n.Nombre && c.Gender == n.Genero && c.Photo == n.MiFoto {
		return *c
	}
	c := NuevaTarjeta(n.Yo, n.Nombre, n.Genero, n.MiFoto, ahora())
	n.tarjetaCache = &c
	return c
}

func (n *Nodo) nickSala() any { return nil }

// Latido floods this device's card, only when what it says changed (never a "still here" on a timer).
func (n *Nodo) Latido() {
	if !n.TieneCuenta() {
		return
	}
	c := n.miTarjeta()
	dicho := c.Contenido() + "|" + n.SalaActual
	if dicho == n.helloDicho {
		return
	}
	n.helloDicho = dicho
	n.Router.Inundar(Msg{"t": "hello", "card": c.JSON(), "chain": n.Cadena, "room": salaONulo(n.SalaActual)}, 3)
}

func salaONulo(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// EnviarTarjeta sends this card sealed to one member at once (first contact over Nostr), at most every 30 s.
func (n *Nodo) EnviarTarjeta(to string, reply bool) {
	if !n.TieneCuenta() || to == n.Clave() {
		return
	}
	t := ahora()
	if t-n.cardSentTo[to] < 30000 {
		return
	}
	n.cardSentTo[to] = t
	n.EnviarSenal(to, Msg{"kind": "pc", "card": n.miTarjeta().JSON(), "chain": n.Cadena, "room": salaONulo(n.SalaActual), "re": reply})
}

func (n *Nodo) alHola(m Msg, de string) {
	j, ok := Objeto(m, "card")
	if !ok {
		return
	}
	c, ok := TarjetaDe(j)
	if !ok {
		return
	}
	prev := n.personas[c.Key]
	if !(prev != nil && prev.Tarjeta.Contenido() == c.Contenido()) && !c.Verificada() {
		return
	}
	if c.Key == n.Clave() || !n.EsMiembro(c.Key, m["chain"]) {
		return
	}
	sala, _ := Texto(m, "room")
	estaba := prev != nil && prev.Sala == n.SalaActual && n.SalaActual != "" && n.presente(c.Key)
	p := &Persona{Tarjeta: c, Visto: ahora(), Sala: sala}
	if prev != nil {
		p.Nick = prev.Nick
	}
	if nk, ok := Texto(m, "nick"); ok {
		p.Nick = nk
	}
	n.personas[c.Key] = p
	n.aprenderNombre(c.Key, c.Name)
	if de != "" {
		n.via[c.Key] = de
	}
	if n.SalaActual != "" {
		quien := n.NombreDe(c.Key)
		if sala == n.SalaActual && !estaba {
			n.lineaSistema(quien + " entró a la sala")
		}
		if sala != n.SalaActual && estaba {
			n.lineaSistema(quien + " salió de la sala")
		}
	}
	if c.Photo != "" && n.Alm.Foto(c.Photo) == nil {
		n.PedirFoto(c.Photo, c.Key)
	}
	n.cambio()
}

// SayBye: a signed "I am going", so the others take this device off their lists at once.
func (n *Nodo) Chau() {
	if n.Yo == nil {
		return
	}
	me, at := n.Clave(), ahora()
	n.Router.Inundar(Msg{"t": "bye", "by": me, "at": at, "sig": n.Yo.Firmar(fmt.Sprintf("BY1|%s|%d", me, at))}, 3)
}

func (n *Nodo) alChau(m Msg, _ string) {
	by, _ := Texto(m, "by")
	at, ok := Entero(m, "at")
	sig, _ := Texto(m, "sig")
	if !ok || by == "" || abs(ahora()-at) > 600000 || !Verificar(by, fmt.Sprintf("BY1|%s|%d", by, at), sig) {
		return
	}
	if p := n.personas[by]; p != nil && p.Visto <= at {
		p.Visto = 0
		delete(n.via, by)
	}
	delete(n.beaconAt, by)
	n.cambio()
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func (n *Nodo) soltar(keys []string, menos string) {
	if len(keys) == 0 {
		return
	}
	for _, k := range keys {
		delete(n.via, k)
		if p := n.personas[k]; p != nil {
			p.Visto = 0
		}
	}
	ks := make([]any, len(keys))
	for i, k := range keys {
		ks[i] = k
	}
	for _, v := range n.Router.Vecinos() {
		if v != menos {
			n.Router.InundarHacia(v, Msg{"t": "gone", "ks": ks}, 1)
		}
	}
	n.cambio()
}

func (n *Nodo) alPerderEnlace(vecino string) {
	var ks []string
	for k, v := range n.via {
		if v == vecino {
			ks = append(ks, k)
		}
	}
	n.soltar(ks, "")
	n.helloDicho = ""
	n.Latido()
}

func (n *Nodo) alIdos(m Msg, de string) {
	l, ok := Lista(m, "ks")
	if !ok || de == "" {
		return
	}
	var ks []string
	for _, k := range l {
		if s, ok := k.(string); ok && n.via[s] == de {
			ks = append(ks, s)
		}
	}
	n.soltar(ks, de)
}

// OidoDe: a member's beacon arrived (a sign of life even without a link).
func (n *Nodo) OidoDe(key string, at int64) {
	if at > ahora() {
		at = ahora()
	}
	if at > n.beaconAt[key] {
		n.beaconAt[key] = at
	}
}

const (
	staleMs      = 5 * 60000
	beaconLifeMs = 5 * 60000
)

func (n *Nodo) presente(key string) bool {
	p := n.personas[key]
	if p == nil {
		return false
	}
	if _, ok := n.via[key]; ok {
		return true
	}
	t := ahora()
	b, ok := n.beaconAt[key]
	return t-p.Visto <= staleMs || (ok && t-b <= beaconLifeMs)
}

func (n *Nodo) EnLinea(key string) bool { return n.presente(key) }

// NombreDe: the name someone shows (room nickname or card name), or a short key.
func (n *Nodo) NombreDe(key string) string {
	if key == n.Clave() {
		return n.Nombre
	}
	if p := n.personas[key]; p != nil {
		if p.Nick != "" {
			return p.Nick
		}
		return p.Tarjeta.Name
	}
	if nk, ok := n.Encontrados[key]; ok && nk != "" {
		return nk
	}
	if nk, ok := n.nombres[key]; ok && nk != "" {
		return nk
	}
	// Someone never seen, as the app names them.
	if len(key) >= 6 {
		return "Miembro " + key[:6]
	}
	return "Miembro " + key
}

func (n *Nodo) FotoDe(key string) string {
	if key == n.Clave() {
		return n.MiFoto
	}
	if p := n.personas[key]; p != nil {
		return p.Tarjeta.Photo
	}
	return ""
}

func (n *Nodo) GeneroDe(key string) string {
	if key == n.Clave() {
		return n.Genero
	}
	if p := n.personas[key]; p != nil {
		return p.Tarjeta.Gender
	}
	return ""
}

// Miembro of a room's list.
type Miembro struct {
	Clave, Nombre, Genero, Foto string
	EnLinea, Creador            bool
}

/* ---------- sealed signals (SG2) ---------- */

// EnviarSenal seals data for to and sends it the shortest way: a direct link, else the Nostr board, else the mesh.
func (n *Nodo) EnviarSenal(to string, data Msg) {
	n.senalPor(to, data, true)
}

func (n *Nodo) senalPor(to string, data Msg, flood bool) bool {
	if n.Yo == nil {
		return false
	}
	at, from := ahora(), n.Clave()
	sealed, err := n.Yo.Sellar(to, []byte(Codificar(data)))
	if err != nil {
		return false
	}
	body := hex.EncodeToString(sealed)
	env := Msg{"t": "sig", "sf": from, "st": to, "at": at, "box": body, "sig": n.Yo.Firmar(fmt.Sprintf("SG2|%s|%s|%d|%s", from, to, at, body)), "chain": n.Cadena}
	if n.Router.InundarHacia(VecinoNet(to), env, 1) {
		return true
	}
	if n.Tablero.EnviarSenal(to, env) {
		return true
	}
	if flood {
		n.Router.Inundar(env, 6)
	}
	return false
}

// SenalDirecta seals data for to and sends it only over the direct link with them (heavy traffic such as screen
// tiles never goes through the public relays); false when there is no such link.
func (n *Nodo) SenalDirecta(to string, data Msg) bool {
	if n.Yo == nil || !n.Router.EsVecino(VecinoNet(to)) {
		return false
	}
	at, from := ahora(), n.Clave()
	sealed, err := n.Yo.Sellar(to, []byte(Codificar(data)))
	if err != nil {
		return false
	}
	body := hex.EncodeToString(sealed)
	env := Msg{"t": "sig", "sf": from, "st": to, "at": at, "box": body, "sig": n.Yo.Firmar(fmt.Sprintf("SG2|%s|%s|%d|%s", from, to, at, body)), "chain": n.Cadena}
	return n.Router.InundarHacia(VecinoNet(to), env, 1)
}

// RecibirSobre: a sealed signal envelope that arrived outside the mesh (Nostr).
func (n *Nodo) RecibirSobre(env Msg) { n.alSobre(env) }

func (n *Nodo) alSobre(m Msg) {
	sig, _ := Texto(m, "sig")
	if sig != "" {
		if n.sigVisto[sig] {
			return
		}
		if len(n.sigVisto) > 2000 {
			n.sigVisto = map[string]bool{}
		}
		n.sigVisto[sig] = true
	}
	from, _ := Texto(m, "sf")
	to, _ := Texto(m, "st")
	at, ok := Entero(m, "at")
	body, _ := Texto(m, "box")
	if n.Yo == nil || to != n.Clave() || from == "" || !ok || body == "" || sig == "" {
		return
	}
	if !Verificar(from, fmt.Sprintf("SG2|%s|%s|%d|%s", from, to, at, body), sig) || !n.EsMiembro(from, m["chain"]) {
		return
	}
	raw, err := hex.DecodeString(body)
	if err != nil {
		return
	}
	plain, ok := n.Yo.Abrir(from, raw)
	if !ok {
		return
	}
	d, ok := Decodificar(string(plain))
	if !ok {
		return
	}
	n.senalPropia(from, d)
}

func (n *Nodo) senalPropia(from string, d Msg) {
	kind, _ := Texto(d, "kind")
	switch {
	case kind == "pc":
		if c, ok := Objeto(d, "card"); ok {
			h := Msg{"card": c, "chain": d["chain"], "room": d["room"]}
			if nk, ok := Texto(d, "nick"); ok {
				h["nick"] = nk
			}
			n.alHola(h, "")
			if d["re"] == true {
				n.EnviarTarjeta(from, false)
			}
		}
	case kind == "dm":
		if m, ok := Objeto(d, "m"); ok {
			n.alDm(m)
		}
	case kind == "dm-ok":
		id, _ := Texto(d, "id")
		n.confirmado(from, id)
	case strings.HasPrefix(kind, "n-"):
		n.Red.alSenal(from, d)
	default:
		if n.OnSenal != nil {
			n.OnSenal(from, d)
		}
	}
}

/* ---------- photos and files travel by fingerprint ---------- */

const maxFoto = 64 << 20

func (n *Nodo) guardarFoto(b []byte) string {
	h := HashBytes(b)
	if n.Alm.Foto(h) == nil {
		n.Alm.GuardarFoto(h, b)
	}
	return h
}

// Foto returns the bytes of a fingerprint held here, nil if not (yet).
func (n *Nodo) Foto(h string) []byte { return n.Alm.Foto(h) }

// PedirFoto asks for a photo: of its owner over a direct link (opened on demand), else of the mesh.
func (n *Nodo) PedirFoto(h, de string) {
	if h == "" || n.Alm.Foto(h) != nil {
		return
	}
	q := Msg{"t": "photoreq", "h": h, "by": n.Router.Self}
	if n.Yo != nil {
		q["k"] = n.Clave()
	}
	if de != "" && de != n.Clave() {
		if n.Router.InundarHacia(VecinoNet(de), q, 1) {
			return
		}
		if n.Tablero.Abierto() {
			if n.fotoEspera[de] == nil {
				n.fotoEspera[de] = map[string]bool{}
			}
			n.fotoEspera[de][h] = true
			n.Red.Querer(de)
			return
		}
	}
	n.Router.Inundar(q, 4)
}

// alcanzable: the direct link with key opened: the photos waiting for it are asked now.
func (n *Nodo) alcanzable(key string) {
	for h := range n.fotoEspera[key] {
		n.Router.InundarHacia(VecinoNet(key), Msg{"t": "photoreq", "h": h, "by": n.Router.Self, "k": n.Clave()}, 1)
	}
	delete(n.fotoEspera, key)
}

func (n *Nodo) alPedidoFoto(m Msg, de string) {
	h, _ := Texto(m, "h")
	b := n.Alm.Foto(h)
	if b == nil {
		return
	}
	by, _ := Texto(m, "by")
	t, clave := ahora(), h+"|"+by
	k, _ := Texto(m, "k")
	directo := ""
	if len(k) >= 16 && n.Router.EsVecino(VecinoNet(k)) {
		directo = VecinoNet(k)
	}
	espera := int64(6000)
	if directo != "" {
		espera = 2500
	}
	if t-n.fotoRespondida[clave] < espera {
		return
	}
	n.fotoRespondida[clave] = t
	resp := Msg{"t": "photo", "h": h, "b64": base64.StdEncoding.EncodeToString(b)}
	if by != "" {
		resp["for"] = by
	}
	if directo != "" && n.Router.InundarHacia(directo, resp, 1) {
		return
	}
	if de != "" && n.Router.InundarHacia(de, resp, 4) {
		return
	}
	n.Router.Inundar(resp, 4)
}

func (n *Nodo) alFoto(m Msg, _ string) {
	h, _ := Texto(m, "h")
	b64, _ := Texto(m, "b64")
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(b) > maxFoto || HashBytes(b) != h {
		return
	}
	n.Alm.GuardarFoto(h, b)
	n.cambio()
}

/* ---------- lists for the screen ---------- */

// Gente: everyone present, by name.
func (n *Nodo) Gente() []Miembro {
	var out []Miembro
	for k, p := range n.personas {
		if n.presente(k) {
			out = append(out, Miembro{Clave: k, Nombre: n.NombreDe(k), Genero: p.Tarjeta.Gender, Foto: p.Tarjeta.Photo, EnLinea: true})
		}
	}
	ordenarMiembros(out)
	return out
}
