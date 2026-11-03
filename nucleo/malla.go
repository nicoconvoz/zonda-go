package nucleo

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Protocolo is the mesh protocol stamped on every message (MeshRouter.protocol).
const Protocolo = 2

type Manejador func(m Msg, deVecino string)

// Router is the mesh (mesh/mesh_router.dart): each device talks only to its neighbors; floods carry an id and a hop
// budget and every device forwards each one once. Not safe for concurrent use: the node calls it from its own loop.
type Router struct {
	Self      string
	enviar    func(to, json string)
	vecinos   map[string]bool
	visto     map[string]bool
	orden     []string
	manej     map[string][]Manejador
	n         int64
	alAgregar []func(id string, solo bool)
	alQuitar  []func(id string)
	OnLegacy  func(vecino string)
}

func NuevoRouter(self string, enviar func(to, json string)) *Router {
	return &Router{Self: self, enviar: enviar, vecinos: map[string]bool{}, visto: map[string]bool{}, manej: map[string][]Manejador{}}
}

func (r *Router) Vecinos() []string {
	out := make([]string, 0, len(r.vecinos))
	for v := range r.vecinos {
		out = append(out, v)
	}
	return out
}

func (r *Router) EsVecino(id string) bool { return r.vecinos[id] }

func (r *Router) AlAgregar(f func(id string, solo bool)) { r.alAgregar = append(r.alAgregar, f) }
func (r *Router) AlQuitar(f func(id string))             { r.alQuitar = append(r.alQuitar, f) }

func (r *Router) Agregar(id string) {
	solo := len(r.vecinos) == 0
	if r.vecinos[id] {
		return
	}
	r.vecinos[id] = true
	for _, f := range r.alAgregar {
		f(id, solo)
	}
}

func (r *Router) Quitar(id string) {
	if !r.vecinos[id] {
		return
	}
	delete(r.vecinos, id)
	for _, f := range r.alQuitar {
		f(id)
	}
}

func (r *Router) On(t string, h Manejador) { r.manej[t] = append(r.manej[t], h) }

func (r *Router) mid() string {
	s := r.Self
	if len(s) > 8 {
		s = s[:8]
	}
	r.n++
	return s + "." + strconv.FormatInt(time.Now().UnixMicro(), 36) + "." + strconv.FormatInt(r.n, 36)
}

func (r *Router) recordar(mid string) {
	if r.visto[mid] {
		return
	}
	r.visto[mid] = true
	r.orden = append(r.orden, mid)
	if len(r.orden) > 4000 {
		delete(r.visto, r.orden[0])
		r.orden = r.orden[1:]
	}
}

// Enviar: a direct message to a neighbor; false when it is not linked.
func (r *Router) Enviar(to string, m Msg) bool {
	if !r.vecinos[to] {
		return false
	}
	o := Copiar(m)
	o["from"], o["to"], o["mid"], o["hv"] = r.Self, to, r.mid(), Protocolo
	r.enviar(to, Codificar(o))
	return true
}

// InundarHacia: a flood that starts toward one neighbor only (an answer going back the way its question came).
func (r *Router) InundarHacia(vecino string, m Msg, ttl int) bool {
	if !r.vecinos[vecino] {
		return false
	}
	o := Copiar(m)
	o["from"], o["mid"], o["ttl"], o["flood"], o["hv"] = r.Self, r.mid(), ttl, 1, Protocolo
	r.recordar(o["mid"].(string))
	r.enviar(vecino, Codificar(o))
	return true
}

func (r *Router) Inundar(m Msg, ttl int) {
	o := Copiar(m)
	o["from"], o["mid"], o["ttl"], o["flood"], o["hv"] = r.Self, r.mid(), ttl, 1, Protocolo
	r.recordar(o["mid"].(string))
	r.reenviar(o, "")
}

func (r *Router) reenviar(m Msg, menos string) {
	m["hv"] = Protocolo
	js := Codificar(m)
	for n := range r.vecinos {
		if n != menos {
			r.enviar(n, js)
		}
	}
}

// Recibir takes one whole mesh message (JSON) from a neighbor.
func (r *Router) Recibir(de, js string) {
	m, ok := Decodificar(js)
	if !ok {
		return
	}
	t, ok := Texto(m, "t")
	if !ok {
		return
	}
	if hv, ok := Entero(m, "hv"); !ok || hv < Protocolo {
		if r.OnLegacy != nil {
			r.OnLegacy(de)
		}
		return
	}
	if m["from"] == r.Self {
		return
	}
	if mid, ok := Texto(m, "mid"); ok {
		if r.visto[mid] {
			return
		}
		r.recordar(mid)
	}
	if to, ok := m["to"]; ok && to != nil && to != r.Self {
		return
	}
	flood, _ := Entero(m, "flood")
	ttl, _ := Entero(m, "ttl")
	if flood == 1 && ttl > 1 {
		o := Copiar(m)
		o["ttl"] = ttl - 1
		r.reenviar(o, de)
	}
	if f, ok := m["for"]; ok && f != nil && f != r.Self {
		return
	}
	for _, h := range append(append([]Manejador{}, r.manej[t]...), r.manej["*"]...) {
		h(m, de)
	}
}

/* ---------- big messages travel in numbered parts (mesh/fragments.dart) ---------- */

// Partir splits json into parts of at most max UTF-8 bytes: {"t":"frag","id":..,"i":..,"n":..,"d":<slice>}.
func Partir(js string, max int) []string {
	if len(js) <= max {
		return []string{js}
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(0x7FFFFFFF))
	id := strconv.FormatInt(n.Int64(), 36)
	room := max - 80
	var slices []string
	for start := 0; start < len(js); {
		end, bytes := start, 0
		for end < len(js) {
			r, w := utf8.DecodeRuneInString(js[end:])
			b, _ := json.Marshal(string(r))
			cost := len(b) - 2
			if r == '<' || r == '>' || r == '&' {
				cost = 1 // Go escapes these as <; Dart does not
			}
			if bytes+cost > room {
				break
			}
			bytes += cost
			end += w
		}
		slices = append(slices, js[start:end])
		start = end
	}
	out := make([]string, len(slices))
	for i, s := range slices {
		out[i] = Codificar(Msg{"t": "frag", "id": id, "i": i, "n": len(slices), "d": s})
	}
	return out
}

type mitad struct {
	partes []string
	faltan int
	ultima time.Time
}

// Rearmador puts parts back together (Reassembler). One per link.
type Rearmador struct{ pend map[string]*mitad }

func NuevoRearmador() *Rearmador { return &Rearmador{pend: map[string]*mitad{}} }

// Agregar returns the whole message when part completes one (or part itself when it is not a fragment).
func (r *Rearmador) Agregar(part string) (string, bool) {
	// A raw `"t":"frag"` can only be a key of the message itself: inside a string the quotes come escaped.
	if !strings.Contains(part, `"t":"frag"`) {
		return part, true
	}
	m, ok := Decodificar(part)
	if !ok || m["t"] != "frag" {
		return part, true
	}
	id, ok1 := Texto(m, "id")
	i, ok2 := Entero(m, "i")
	n, ok3 := Entero(m, "n")
	d, ok4 := Texto(m, "d")
	if !ok1 || !ok2 || !ok3 || !ok4 || n <= 0 || i < 0 || i >= n || n > 4096 {
		return "", false
	}
	now := time.Now()
	for k, v := range r.pend {
		if now.Sub(v.ultima) > time.Duration(8000+len(v.partes)*30000/(128*1024)*1000)*time.Millisecond {
			delete(r.pend, k)
		}
	}
	h := r.pend[id]
	if h == nil {
		h = &mitad{partes: make([]string, n), faltan: int(n)}
		h.partes = h.partes[:n]
		for j := range h.partes {
			h.partes[j] = "\x00"
		}
		r.pend[id] = h
	}
	h.ultima = now
	if int64(len(h.partes)) != n {
		return "", false
	}
	if h.partes[i] == "\x00" {
		h.partes[i] = d
		h.faltan--
	}
	if h.faltan > 0 {
		return "", false
	}
	delete(r.pend, id)
	return strings.Join(h.partes, ""), true
}
