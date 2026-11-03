package nucleo

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
)

/* ---------- what one device knows about one room (RoomBook) ---------- */

type Libro struct {
	Carta     Carta
	admis     map[string]Admision
	rotacion  *Rotacion
	canales   *Canales
	Cerrada   bool
}

func nuevoLibro(c Carta) *Libro { return &Libro{Carta: c, admis: map[string]Admision{}} }

func libroDeLineas(lines []string) *Libro {
	if len(lines) == 0 {
		return nil
	}
	c, ok := LeerCarta(lines[0])
	if !ok {
		return nil
	}
	b := nuevoLibro(c)
	for _, l := range lines[1:] {
		b.agregarLinea(l)
	}
	return b
}

func (b *Libro) agregarLinea(l string) bool {
	if a, ok := LeerAdmision(l); ok {
		return b.agregarAdmision(a)
	}
	if r, ok := LeerRotacion(l); ok {
		return b.agregarRotacion(r)
	}
	if c, ok := LeerCierre(l); ok {
		return b.agregarCierre(c)
	}
	if v, ok := LeerCanales(l); ok {
		return b.agregarCanales(v)
	}
	return false
}

func (b *Libro) ID() string      { return b.Carta.ID }
func (b *Libro) Nombre() string  { return b.Carta.Name }
func (b *Libro) Epoca() int64 {
	if b.rotacion == nil {
		return 0
	}
	return b.rotacion.Epoch
}
func (b *Libro) EsCreador(k string) bool { return k == b.Carta.Creator }
func (b *Libro) Baneado(k string) bool  { return b.rotacion != nil && b.rotacion.Banned[k] }

func (b *Libro) Canales() []string {
	if b.canales == nil {
		return []string{"General"}
	}
	return b.canales.Channels
}

// Miembros: the current epoch's roster, then whoever a member of it admitted (until nothing changes).
func (b *Libro) Miembros() map[string]bool {
	out := map[string]bool{b.Carta.Creator: true}
	if b.rotacion != nil {
		for k := range b.rotacion.Roster {
			out[k] = true
		}
	}
	for crecio := true; crecio; {
		crecio = false
		for _, a := range b.admis {
			if a.Epoch == b.Epoca() && out[a.By] && !b.Baneado(a.Member) && !out[a.Member] {
				out[a.Member] = true
				crecio = true
			}
		}
	}
	return out
}

func (b *Libro) EsMiembro(k string) bool { return !b.Cerrada && b.Miembros()[k] }

func (b *Libro) agregarAdmision(a Admision) bool {
	if a.Room != b.ID() {
		return false
	}
	if _, ok := b.admis[a.Line]; ok {
		return false
	}
	b.admis[a.Line] = a
	return true
}

func (b *Libro) agregarRotacion(r Rotacion) bool {
	if r.Room != b.ID() || !b.EsCreador(r.By) || r.Epoch <= b.Epoca() || !r.Roster[b.Carta.Creator] {
		return false
	}
	b.rotacion = &r
	return true
}

func (b *Libro) agregarCanales(v Canales) bool {
	if v.Room != b.ID() || !b.EsCreador(v.By) || (b.canales != nil && v.T <= b.canales.T) {
		return false
	}
	b.canales = &v
	return true
}

func (b *Libro) agregarCierre(c Cierre) bool {
	if c.Room != b.ID() || !b.EsCreador(c.By) {
		return false
	}
	b.Cerrada = true
	return true
}

// Lineas: every record held, to hand to another member (the charter first).
func (b *Libro) Lineas() []string {
	out := []string{b.Carta.Line}
	if b.rotacion != nil {
		out = append(out, b.rotacion.Line)
	}
	if b.canales != nil {
		out = append(out, b.canales.Line)
	}
	for l := range b.admis {
		out = append(out, l)
	}
	return out
}

/* ---------- rooms with an owner on the mesh (RoomsNode) ---------- */

// Pedido is someone asking to enter a room.
type Pedido struct {
	Clave, Nick string
	At          int64
}

type Salas struct {
	n       *Nodo
	libros  map[string]*Libro
	llaves  map[string]map[int64][]byte
	pedidos map[string][]Pedido
	idas    map[string]bool
	// Golpe: the room this person asked to enter and is waiting on.
	Golpe *Carta
}

const ttlSalas = 6

func nuevasSalas(n *Nodo) *Salas {
	s := &Salas{n: n, libros: map[string]*Libro{}, llaves: map[string]map[int64][]byte{}, pedidos: map[string][]Pedido{}, idas: map[string]bool{}}
	s.cargar()
	r := n.Router
	r.On("prq", s.alPedido)
	r.On("pad", s.alAdmision)
	r.On("prk", s.alRotacion)
	r.On("pcl", s.alCierre)
	r.On("pch", s.alCanales)
	// Knocking: each new link may be a member, so the request goes out again then (an event, not a timer).
	r.AlAgregar(func(string, bool) {
		if g := s.Golpe; g != nil && !s.EsMiembro(g.ID, n.Clave()) {
			s.pedirEntrar(*g)
		}
	})
	return s
}

func (n *Nodo) Salas() *Salas { return n.salas }

func (s *Salas) Libro(id string) *Libro { return s.libros[id] }

func (s *Salas) EsMiembro(room, key string) bool {
	b := s.libros[room]
	return b != nil && b.EsMiembro(key)
}

// Mias: the rooms this person is in, newest first.
func (s *Salas) Mias() []*Libro {
	var out []*Libro
	me := s.n.Clave()
	for _, b := range s.libros {
		if !b.Cerrada && b.EsMiembro(me) {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Carta.T > out[j].Carta.T })
	return out
}

func (s *Salas) Pedidos(room string) []Pedido { return append([]Pedido(nil), s.pedidos[room]...) }

func (s *Salas) Crear(nombre string) Carta {
	c := NuevaCarta(s.n.Yo, strings.TrimSpace(nombre), ahora())
	s.libros[c.ID] = nuevoLibro(c)
	s.llaves[c.ID] = map[int64][]byte{0: NuevaLlaveSala()}
	s.guardar()
	return c
}

func cuerpoPedido(room, by, nick string, at int64) string {
	return fmt.Sprintf("PQ1|%s|%s|%s|%d", room, by, nick, at)
}

func (s *Salas) pedirEntrar(c Carta) {
	id := s.n.Yo
	if id == nil {
		return
	}
	at, by, nick := ahora(), id.Publica(), s.n.Nombre
	s.n.Router.Inundar(Msg{"t": "prq", "charter": c.Line, "by": by, "nick": nick, "at": at, "sig": id.Firmar(cuerpoPedido(c.ID, by, nick, at))}, ttlSalas)
}

// Golpear asks to enter c's room: the request goes to every member linked now, and again to each one who links.
func (s *Salas) Golpear(c Carta) {
	s.Golpe = &c
	s.pedirEntrar(c)
	s.n.Tablero.sincronizarSala()
}

func (s *Salas) DejarDeGolpear() {
	s.Golpe = nil
	s.n.Tablero.sincronizarSala()
}

func llaveJSON(e int64, k []byte) []byte {
	return []byte(Codificar(Msg{"e": e, "k": base64.StdEncoding.EncodeToString(k)}))
}

func (s *Salas) llaveActual(room string) []byte {
	b := s.libros[room]
	if b == nil {
		return nil
	}
	return s.llaves[room][b.Epoca()]
}

// Aprobar lets key in (any member may).
func (s *Salas) Aprobar(room, key string) bool {
	id, b, k := s.n.Yo, s.libros[room], s.llaveActual(room)
	if id == nil || b == nil || k == nil || !b.EsMiembro(id.Publica()) || b.Baneado(key) || b.EsMiembro(key) {
		return false
	}
	a := NuevaAdmision(id, room, key, b.Epoca(), ahora())
	b.agregarAdmision(a)
	s.quitarPedido(room, key)
	box, err := id.Sellar(key, llaveJSON(b.Epoca(), k))
	if err != nil {
		return false
	}
	s.guardar()
	s.n.Router.Inundar(Msg{"t": "pad", "line": a.Line, "box": base64.StdEncoding.EncodeToString(box), "book": b.Lineas()}, ttlSalas)
	return true
}

func (s *Salas) Rechazar(room, key string) {
	s.quitarPedido(room, key)
	s.guardar()
}

func (s *Salas) quitarPedido(room, key string) {
	l := s.pedidos[room][:0]
	for _, p := range s.pedidos[room] {
		if p.Clave != key {
			l = append(l, p)
		}
	}
	s.pedidos[room] = l
}

// Echar: the creator leaves key out (for good with ban): a new key for everyone who stays.
func (s *Salas) Echar(room, key string, ban bool) bool {
	id, b := s.n.Yo, s.libros[room]
	if id == nil || b == nil || !b.EsCreador(id.Publica()) || b.EsCreador(key) {
		return false
	}
	roster := b.Miembros()
	delete(roster, key)
	banned := map[string]bool{}
	if b.rotacion != nil {
		for k := range b.rotacion.Banned {
			banned[k] = true
		}
	}
	if ban {
		banned[key] = true
	}
	r := NuevaRotacion(id, room, b.Epoca()+1, roster, banned, ahora())
	b.agregarRotacion(r)
	k := NuevaLlaveSala()
	s.llaves[room][r.Epoch] = k
	keys := Msg{}
	for m := range r.Roster {
		if m != id.Publica() {
			if box, err := id.Sellar(m, llaveJSON(r.Epoch, k)); err == nil {
				keys[m] = base64.StdEncoding.EncodeToString(box)
			}
		}
	}
	s.guardar()
	s.n.Router.Inundar(Msg{"t": "prk", "line": r.Line, "keys": keys}, ttlSalas)
	return true
}

func (s *Salas) Cerrar(room string) bool {
	id, b := s.n.Yo, s.libros[room]
	if id == nil || b == nil || !b.EsCreador(id.Publica()) {
		return false
	}
	c := NuevoCierre(id, room, ahora())
	s.soltar(room)
	s.n.Router.Inundar(Msg{"t": "pcl", "line": c.Line}, ttlSalas)
	return true
}

func (s *Salas) FijarCanales(room string, canales []string) bool {
	id, b := s.n.Yo, s.libros[room]
	var limpio []string
	for _, c := range canales {
		if c = strings.TrimSpace(c); c != "" {
			limpio = append(limpio, c)
		}
	}
	if id == nil || b == nil || !b.EsCreador(id.Publica()) || len(limpio) == 0 {
		return false
	}
	v := NuevosCanales(id, room, limpio, ahora())
	b.agregarCanales(v)
	s.guardar()
	s.n.Router.Inundar(Msg{"t": "pch", "line": v.Line}, ttlSalas)
	return true
}

// Sellar: plain sealed with the room's current key, ready to travel.
func (s *Salas) Sellar(room string, plain []byte) Msg {
	b, k := s.libros[room], s.llaveActual(room)
	if b == nil || k == nil {
		return nil
	}
	return Msg{"room": room, "e": b.Epoca(), "box": base64.StdEncoding.EncodeToString(SellarSala(k, plain))}
}

func (s *Salas) Abrir(sealed Msg) ([]byte, bool) {
	room, _ := Texto(sealed, "room")
	e, ok := Entero(sealed, "e")
	box, _ := Texto(sealed, "box")
	k := s.llaves[room][e]
	if !ok || k == nil {
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(box)
	if err != nil {
		return nil, false
	}
	return AbrirSala(k, raw)
}

func (s *Salas) abrirLlave(de string, box any) (int64, []byte, bool) {
	b64, ok := box.(string)
	if !ok || s.n.Yo == nil {
		return 0, nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return 0, nil, false
	}
	plain, ok := s.n.Yo.Abrir(de, raw)
	if !ok {
		return 0, nil, false
	}
	j, ok := Decodificar(string(plain))
	e, oke := Entero(j, "e")
	k64, _ := Texto(j, "k")
	k, err := base64.StdEncoding.DecodeString(k64)
	if !ok || !oke || err != nil {
		return 0, nil, false
	}
	return e, k, true
}

func (s *Salas) alPedido(m Msg, _ string) {
	c, ok := LeerCarta(Interpolar(m["charter"]))
	by, _ := Texto(m, "by")
	nick, okn := Texto(m, "nick")
	at, oka := Entero(m, "at")
	sig, _ := Texto(m, "sig")
	if !ok || by == "" || !okn || !oka || sig == "" {
		return
	}
	b, me := s.libros[c.ID], s.n.Clave()
	if b == nil || me == "" || !b.EsMiembro(me) || b.EsMiembro(by) || b.Baneado(by) || !Verificar(by, cuerpoPedido(c.ID, by, nick, at), sig) {
		return
	}
	for _, p := range s.pedidos[c.ID] {
		if p.Clave == by {
			return
		}
	}
	s.pedidos[c.ID] = append(s.pedidos[c.ID], Pedido{by, nick, at})
	s.guardar()
}

func (s *Salas) alAdmision(m Msg, _ string) {
	a, ok := LeerAdmision(Interpolar(m["line"]))
	me := s.n.Clave()
	if !ok || me == "" || s.idas[a.Room] {
		return
	}
	if a.Member == me {
		e, k, ok := s.abrirLlave(a.By, m["box"])
		book := Textos(m["book"])
		b := s.libros[a.Room]
		if b == nil {
			b = libroDeLineas(book)
		}
		if !ok || b == nil || b.ID() != a.Room {
			return
		}
		for _, l := range book {
			if _, cierre := LeerCierre(l); !cierre {
				b.agregarLinea(l)
			}
		}
		b.agregarAdmision(a)
		s.libros[a.Room] = b
		if s.llaves[a.Room] == nil {
			s.llaves[a.Room] = map[int64][]byte{}
		}
		s.llaves[a.Room][e] = k
	} else {
		b := s.libros[a.Room]
		if b == nil {
			return
		}
		b.agregarAdmision(a)
		s.quitarPedido(a.Room, a.Member)
	}
	s.guardar()
}

func (s *Salas) alRotacion(m Msg, _ string) {
	r, ok := LeerRotacion(Interpolar(m["line"]))
	if !ok {
		return
	}
	b, me := s.libros[r.Room], s.n.Clave()
	if b == nil || me == "" || !b.agregarRotacion(r) {
		return
	}
	if keys, ok := Objeto(m, "keys"); ok {
		if e, k, ok := s.abrirLlave(r.By, keys[me]); ok && e == r.Epoch {
			s.llaves[r.Room][e] = k
		}
	}
	s.guardar()
}

func (s *Salas) alCanales(m Msg, _ string) {
	v, ok := LeerCanales(Interpolar(m["line"]))
	if !ok {
		return
	}
	if b := s.libros[v.Room]; b != nil && b.agregarCanales(v) {
		s.guardar()
	}
}

func (s *Salas) alCierre(m Msg, _ string) {
	c, ok := LeerCierre(Interpolar(m["line"]))
	if !ok {
		return
	}
	if b := s.libros[c.Room]; b != nil && b.agregarCierre(c) {
		s.soltar(c.Room)
	}
}

func (s *Salas) soltar(room string) {
	delete(s.libros, room)
	delete(s.llaves, room)
	delete(s.pedidos, room)
	s.idas[room] = true
	if s.n.SalaActual == room {
		s.n.SalirDeSala()
	}
	s.guardar()
}

func (s *Salas) guardar() {
	rooms := Msg{}
	for id, b := range s.libros {
		keys := Msg{}
		for e, k := range s.llaves[id] {
			keys[fmt.Sprint(e)] = base64.StdEncoding.EncodeToString(k)
		}
		rooms[id] = Msg{"lines": b.Lineas(), "keys": keys}
	}
	req := Msg{}
	for id, l := range s.pedidos {
		out := []any{}
		for _, p := range l {
			out = append(out, Msg{"key": p.Clave, "nick": p.Nick, "at": p.At})
		}
		req[id] = out
	}
	var idas []string
	for id := range s.idas {
		idas = append(idas, id)
	}
	a := s.n.Alm
	a.EscribirJSON("prooms", rooms)
	a.EscribirJSON("prooms.req", req)
	a.EscribirJSON("prooms.gone", idas)
	s.n.cambio()
}

func (s *Salas) cargar() {
	a := s.n.Alm
	if v, ok := a.LeerJSON("prooms"); ok {
		for id, x := range ComoObjeto(v) {
			o := ComoObjeto(x)
			b := libroDeLineas(Textos(o["lines"]))
			if b == nil {
				continue
			}
			s.libros[id] = b
			s.llaves[id] = map[int64][]byte{}
			ks := ComoObjeto(o["keys"])
			for e, k := range ks {
				var ep int64
				fmt.Sscan(e, &ep)
				if raw, err := base64.StdEncoding.DecodeString(Interpolar(k)); err == nil {
					s.llaves[id][ep] = raw
				}
			}
		}
	}
	if v, ok := a.LeerJSON("prooms.req"); ok {
		for id, l := range ComoObjeto(v) {
			for _, x := range ComoLista(l) {
				o := ComoObjeto(x)
				at, _ := Entero(o, "at")
				s.pedidos[id] = append(s.pedidos[id], Pedido{Interpolar(o["key"]), Interpolar(o["nick"]), at})
			}
		}
	}
	if v, ok := a.LeerJSON("prooms.gone"); ok {
		for _, id := range Textos(v) {
			s.idas[id] = true
		}
	}
}

/* ---------- room codes: the signed charter, nothing secret ---------- */

func (s *Salas) Codigo(room string) string {
	b := s.libros[room]
	if b == nil {
		return ""
	}
	return "ZR1:" + base64.URLEncoding.EncodeToString([]byte(b.Carta.Line))
}

func CartaDeCodigo(code string) (Carta, bool) {
	c := strings.TrimSpace(code)
	if !strings.HasPrefix(c, "ZR1:") {
		return Carta{}, false
	}
	raw, err := base64.URLEncoding.DecodeString(c[4:])
	if err != nil {
		return Carta{}, false
	}
	return LeerCarta(string(raw))
}

/* ---------- a room's chat ---------- */

// EntrarASala puts the room on screen: its lines are only the ones said while inside.
func (n *Nodo) EntrarASala(room string) {
	n.SalaActual = room
	n.LineasSala = nil
	n.Latido()
	n.Tablero.sincronizarSala()
	n.cambio()
}

func (n *Nodo) SalirDeSala() {
	n.SalaActual = ""
	n.LineasSala = nil
	n.Latido()
	n.Tablero.sincronizarSala()
	n.cambio()
}

func (n *Nodo) lineaSistema(t string) {
	n.LineasSala = append(n.LineasSala, LineaSala{Sala: n.SalaActual, Texto: t, At: ahora(), Sistema: true, Epoca: -1})
}

func cuerpoLinea(r Msg) string {
	s := fmt.Sprintf("RM1|%s|%s|%s|%s|%s|%s", Interpolar(r["room"]), Interpolar(r["by"]), Interpolar(r["at"]), Interpolar(r["nick"]), Interpolar(r["g"]), Interpolar(r["text"]))
	ph, f := r["ph"], r["f"]
	if ph != nil || f != nil {
		fh := ""
		if fo, ok := f.(map[string]any); ok {
			fh = Interpolar(fo["h"])
		}
		e := ""
		if r["e"] != nil {
			e = Interpolar(r["e"])
		}
		phs := ""
		if ph != nil {
			phs = Interpolar(ph)
		}
		s += "|ph=" + phs + "|f=" + fh + "|e=" + e
	}
	return s
}

func lineaDe(r Msg) LineaSala {
	l := LineaSala{Epoca: -1}
	l.Sala, _ = Texto(r, "room")
	l.Por, _ = Texto(r, "by")
	l.Nick = Interpolar(r["nick"])
	l.Genero = Interpolar(r["g"])
	l.Texto, _ = Texto(r, "text")
	l.At, _ = Entero(r, "at")
	l.Foto, _ = Texto(r, "ph")
	l.Archivo, _ = Objeto(r, "f")
	if e, ok := Entero(r, "e"); ok {
		l.Epoca = e
	}
	return l
}

// Archivo to attach to a room line or a chat.
type Archivo struct {
	Bytes             []byte
	Nombre, Mime, Tipo string // Tipo: file | voice | sticker
}

// DecirEnSala says text (with an optional photo or file) in the current room, sealed with its key.
func (n *Nodo) DecirEnSala(texto string, foto []byte, arch *Archivo) string {
	room := n.SalaActual
	if n.Yo == nil || room == "" || (strings.TrimSpace(texto) == "" && foto == nil && arch == nil) {
		return "empty"
	}
	dicho := strings.TrimSpace(texto)
	if dicho == "" {
		if arch != nil {
			dicho = "📎 " + arch.Nombre
		} else {
			dicho = "🖼️"
		}
	}
	privada := n.salas.Libro(room) != nil
	guardar := func(b []byte) (string, int64, bool) {
		if !privada {
			return n.guardarFoto(b), -1, true
		}
		s := n.salas.Sellar(room, b)
		if s == nil {
			return "", 0, false
		}
		raw, _ := base64.StdEncoding.DecodeString(s["box"].(string))
		e, _ := Entero(s, "e")
		return n.guardarFoto(raw), e, true
	}
	r := Msg{"room": room, "by": n.Clave(), "at": ahora(), "nick": n.Nombre, "g": n.Genero, "text": dicho}
	epoca := int64(-1)
	if foto != nil {
		h, e, ok := guardar(foto)
		if !ok {
			return "empty"
		}
		r["ph"], epoca = h, e
	}
	if arch != nil {
		h, e, ok := guardar(arch.Bytes)
		if !ok {
			return "empty"
		}
		r["f"], epoca = Msg{"h": h, "n": arch.Nombre, "m": arch.Mime, "k": arch.Tipo}, e
	}
	if epoca >= 0 {
		r["e"] = epoca
	}
	r["sig"] = n.Yo.Firmar(cuerpoLinea(r))
	l := lineaDe(r)
	if foto != nil || arch != nil {
		// My own attachment opens from what I just sealed: keep the plain bytes for the screen.
		if foto != nil {
			n.aperturas[fmt.Sprintf("%s|%d|%s", room, epoca, l.Foto)] = foto
		}
		if arch != nil {
			n.aperturas[fmt.Sprintf("%s|%d|%s", room, epoca, Interpolar(l.Archivo["h"]))] = arch.Bytes
		}
	}
	n.LineasSala = append(n.LineasSala, l)
	out := Msg{"t": "room", "chain": n.Cadena}
	if privada {
		s := n.salas.Sellar(room, []byte(Codificar(r)))
		if s == nil {
			return "empty"
		}
		out["sealed"] = s
	} else {
		out["msg"] = r
	}
	n.Router.Inundar(out, 4)
	n.cambio()
	return "ok"
}

func (n *Nodo) alSala(m Msg, _ string) {
	r, _ := Objeto(m, "msg")
	if s, ok := Objeto(m, "sealed"); ok {
		plain, ok := n.salas.Abrir(s)
		if !ok {
			return
		}
		x, ok := Decodificar(string(plain))
		if !ok || x["room"] != s["room"] || !n.salas.EsMiembro(Interpolar(s["room"]), Interpolar(x["by"])) {
			return
		}
		r = x
	}
	if r == nil || r["room"] != n.SalaActual || n.SalaActual == "" {
		return
	}
	by, ok1 := Texto(r, "by")
	sig, ok2 := Texto(r, "sig")
	_, ok3 := Texto(r, "text")
	_, ok4 := Entero(r, "at")
	if !ok1 || !ok2 || !ok3 || !ok4 || !Verificar(by, cuerpoLinea(r), sig) || !n.EsMiembro(by, m["chain"]) {
		return
	}
	n.LineasSala = append(n.LineasSala, lineaDe(r))
	n.cambio()
}

// Adjunto: an attachment of l ready to show, opened with the room's key; nil until it arrives (it is asked for).
func (n *Nodo) Adjunto(l LineaSala, h string) []byte {
	clave := fmt.Sprintf("%s|%d|%s", l.Sala, l.Epoca, h)
	if b, ok := n.aperturas[clave]; ok {
		return b
	}
	b := n.Alm.Foto(h)
	if b == nil {
		n.PedirFoto(h, l.Por)
		return nil
	}
	if l.Epoca < 0 {
		return b
	}
	p, ok := n.salas.Abrir(Msg{"room": l.Sala, "e": l.Epoca, "box": base64.StdEncoding.EncodeToString(b)})
	if !ok {
		return nil
	}
	n.aperturas[clave] = p
	return p
}

// MiembrosSala: the room's members (from its book), online first.
func (n *Nodo) MiembrosSala(room string) []Miembro {
	b := n.salas.Libro(room)
	if b == nil {
		return nil
	}
	var out []Miembro
	for k := range b.Miembros() {
		en := k == n.Clave() || (n.presente(k) && n.personas[k] != nil && n.personas[k].Sala == room)
		out = append(out, Miembro{Clave: k, Nombre: n.NombreDe(k), Genero: n.GeneroDe(k), Foto: n.FotoDe(k), EnLinea: en, Creador: b.EsCreador(k)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EnLinea != out[j].EnLinea {
			return out[i].EnLinea
		}
		return strings.ToLower(out[i].Nombre) < strings.ToLower(out[j].Nombre)
	})
	return out
}

// EnSala: the members present in room now (keys), me excluded.
func (n *Nodo) EnSala(room string) []string {
	var out []string
	for k, p := range n.personas {
		if p.Sala == room && n.presente(k) {
			out = append(out, k)
		}
	}
	return out
}
