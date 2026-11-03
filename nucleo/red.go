package nucleo

import (
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

/* ---------- Internet links between members (net/net_node.dart), as WebRTC data channels ---------- */

type estado int

const (
	inactivo estado = iota
	ofreciendo
	saludado
	respondiendo
	arriba
)

type par struct {
	st       estado
	at, next int64
	fallas   int
}

const (
	pendingMs  = 12000
	redialMs   = 10000
	maxBackoff = 2 * 60000
	maxEnlaces = 8
	maxPending = 4
	fragmento  = 60000
)

var stunPublicos = []string{"stun:stun.l.google.com:19302", "stun:stun.cloudflare.com:3478"}

// Red opens and keeps the direct links. Its state lives on the node's loop; the WebRTC work runs on its own
// goroutines and posts results back.
type Red struct {
	n     *Nodo
	pares map[string]*par
	cache []string
	// Stats: what happened while opening links, for the Red screen.
	Stats map[string]int

	mu   sync.Mutex // guards the connections, used from pion's goroutines
	pcs  map[string]*webrtc.PeerConnection
	dcs  map[string]*webrtc.DataChannel
	voz  map[string]*webrtc.DataChannel
	porV map[string]string

	// OnVoz gets the voice frames from another Go device (called on pion's goroutine).
	OnVoz func(key string, b []byte)
}

func nuevaRed(n *Nodo) *Red {
	r := &Red{n: n, pares: map[string]*par{}, Stats: map[string]int{}, pcs: map[string]*webrtc.PeerConnection{}, dcs: map[string]*webrtc.DataChannel{}, voz: map[string]*webrtc.DataChannel{}, porV: map[string]string{}}
	if v, ok := n.Alm.LeerJSON("net.cache"); ok {
		r.cache = Textos(v)
	}
	return r
}

func (r *Red) contar(k string) { r.Stats[k]++ }

func (r *Red) p(key string) *par {
	x := r.pares[key]
	if x == nil {
		x = &par{}
		r.pares[key] = x
	}
	return x
}

func (r *Red) Enlazados() []string {
	var out []string
	for k, p := range r.pares {
		if p.st == arriba {
			out = append(out, k)
		}
	}
	return out
}

func (r *Red) pendientes() int {
	c := 0
	for _, p := range r.pares {
		if p.st == ofreciendo || p.st == saludado || p.st == respondiendo {
			c++
		}
	}
	return c
}

func (r *Red) lugar() bool { return len(r.Enlazados())+r.pendientes() < maxEnlaces }

func backoff(f int) int64 {
	if f < 1 {
		f = 1
	}
	if f > 11 {
		f = 11
	}
	return min(int64(redialMs)<<(f-1), maxBackoff)
}

// queridos: contacts first, then the members of the room on screen, then phones linked before.
func (r *Red) queridos() []string {
	var out []string
	seen := map[string]bool{r.n.Clave(): true}
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for k := range r.n.Contactos {
		add(k)
	}
	if room := r.n.SalaActual; room != "" {
		for _, k := range r.n.EnSala(room) {
			add(k)
		}
	}
	for i := len(r.cache) - 1; i >= 0; i-- {
		add(r.cache[i])
	}
	return out
}

// Tick dials who should be linked and is not (every beat).
func (r *Red) Tick() {
	if r.n.Yo == nil {
		return
	}
	r.vencidos()
	t := ahora()
	for _, k := range r.queridos() {
		if len(r.Enlazados())+r.pendientes() >= maxEnlaces || r.pendientes() >= maxPending {
			break
		}
		p := r.p(k)
		if p.st != inactivo || p.next > t {
			continue
		}
		r.conectar(k, false)
	}
}

func (r *Red) vencidos() {
	t := ahora()
	for k, p := range r.pares {
		if (p.st == ofreciendo || p.st == saludado || p.st == respondiendo) && t-p.at > pendingMs {
			r.cerrar(k)
			p.st = inactivo
			p.fallas++
			p.next = t + backoff(p.fallas)
			r.contar("vencidos")
		}
	}
}

// MarcarYa: a member's beacon just arrived: dial it now.
func (r *Red) MarcarYa(key string) {
	if key == r.n.Clave() || r.n.Yo == nil {
		return
	}
	p := r.p(key)
	if p.st != inactivo || len(r.Enlazados()) >= maxEnlaces {
		return
	}
	p.next = 0
	r.conectar(key, false)
}

// Querer: a link is needed now with key (a photo waits for it).
func (r *Red) Querer(key string) {
	if key == r.n.Clave() || r.n.Yo == nil || r.p(key).st != inactivo {
		return
	}
	r.p(key).next = 0
	r.conectar(key, true)
}

// conectar: a hello first; a connection is prepared only once the other side answers.
func (r *Red) conectar(key string, need bool) {
	p := r.p(key)
	p.at, p.st = ahora(), saludado
	r.contar("saludos enviados")
	d := Msg{"kind": "n-ask"}
	if need {
		d["need"] = 1
	}
	r.n.EnviarSenal(key, d)
}

func (r *Red) ofrecer(key string, need bool) {
	p := r.p(key)
	p.st, p.at = ofreciendo, ahora()
	go func() {
		sdp, err := r.crearOferta(key)
		r.n.Hacer(func() {
			if err != nil || p.st != ofreciendo {
				return
			}
			r.contar("ofertas enviadas")
			d := Msg{"kind": "n-offer", "sdp": sdp}
			if need {
				d["need"] = 1
			}
			r.n.EnviarSenal(key, d)
		})
	}()
}

func (r *Red) alSenal(from string, d Msg) {
	me := r.n.Clave()
	if me == "" {
		return
	}
	p := r.p(from)
	need, _ := Entero(d, "need")
	switch d["kind"] {
	case "n-ask":
		r.contar("saludos recibidos")
		if p.st == arriba || p.st == ofreciendo || p.st == respondiendo || (p.st != saludado && !r.lugar()) {
			return
		}
		if me < from {
			r.ofrecer(from, need == 1)
		} else if p.st != saludado {
			p.st, p.at = saludado, ahora()
			r.contar("saludos enviados")
			x := Msg{"kind": "n-ask"}
			if need == 1 {
				x["need"] = 1
			}
			r.n.EnviarSenal(from, x)
		}
	case "n-offer":
		r.contar("ofertas recibidas")
		sdp, ok := Texto(d, "sdp")
		if !ok || p.st == arriba || (p.st == ofreciendo && me < from) || (p.st == inactivo && !r.lugar()) {
			return
		}
		p.st, p.at = respondiendo, ahora()
		go func() {
			ans, err := r.crearRespuesta(from, sdp)
			r.n.Hacer(func() {
				if err != nil {
					p.st = inactivo
					return
				}
				r.contar("respuestas enviadas")
				r.n.EnviarSenal(from, Msg{"kind": "n-answer", "sdp": ans})
			})
		}()
	case "n-answer":
		r.contar("respuestas recibidas")
		if sdp, ok := Texto(d, "sdp"); ok && p.st == ofreciendo {
			go r.completar(from, sdp)
		}
	}
}

func (r *Red) arriba(key string) {
	r.contar("enlaces arriba")
	p := r.p(key)
	p.st, p.fallas, p.next = arriba, 0, 0
	r.n.Router.Agregar(VecinoNet(key))
	for i, k := range r.cache {
		if k == key {
			r.cache = append(r.cache[:i], r.cache[i+1:]...)
			break
		}
	}
	r.cache = append(r.cache, key)
	if len(r.cache) > 32 {
		r.cache = r.cache[1:]
	}
	r.n.Alm.EscribirJSON("net.cache", r.cache)
	r.n.Latido()
	r.n.alcanzable(key)
	r.n.cambio()
}

func (r *Red) abajo(key string) {
	p := r.pares[key]
	if p == nil {
		return
	}
	r.contar("enlaces caídos")
	r.n.Router.Quitar(VecinoNet(key))
	p.st, p.next = inactivo, ahora()+redialMs
	r.n.cambio()
}

// legacyVecino: a neighbor on an old version (no protocol mark) is cut off for a day.
func (r *Red) legacyVecino(v string) {
	for k, p := range r.pares {
		if VecinoNet(k) == v {
			r.cerrar(k)
			r.abajo(k)
			p.next = ahora() + 24*3600000
		}
	}
}

/* ---------- the WebRTC side ---------- */

func iceServers() []webrtc.ICEServer {
	return []webrtc.ICEServer{{URLs: stunPublicos}}
}

func (r *Red) nuevaPC(key string) (*webrtc.PeerConnection, error) {
	r.cerrar(key)
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers()})
	if err != nil {
		return nil, err
	}
	// The voice between Go devices: a channel of its own, unordered and never retransmitted (a late voice frame is
	// worth nothing). Negotiated on both sides with a fixed id, so the app's side never sees it open.
	ordenado, intentos, idVoz := false, uint16(0), uint16(7)
	if dv, err := pc.CreateDataChannel("zvoz", &webrtc.DataChannelInit{Ordered: &ordenado, MaxRetransmits: &intentos, Negotiated: &[]bool{true}[0], ID: &idVoz}); err == nil {
		dv.OnMessage(func(m webrtc.DataChannelMessage) {
			if !m.IsString && r.OnVoz != nil {
				r.OnVoz(key, m.Data)
			}
		})
		r.mu.Lock()
		r.voz[key] = dv
		r.mu.Unlock()
	}
	r.mu.Lock()
	r.pcs[key] = pc
	r.porV[VecinoNet(key)] = key
	r.mu.Unlock()
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			r.mu.Lock()
			mine := r.pcs[key] == pc
			if mine {
				delete(r.pcs, key)
				delete(r.dcs, key)
			}
			r.mu.Unlock()
			if mine {
				pc.Close()
				r.n.Hacer(func() { r.abajo(key) })
			}
		}
	})
	return pc, nil
}

func (r *Red) cablear(key string, dc *webrtc.DataChannel) {
	r.mu.Lock()
	r.dcs[key] = dc
	r.mu.Unlock()
	re := NuevoRearmador()
	dc.OnOpen(func() { r.n.Hacer(func() { r.arriba(key) }) })
	dc.OnClose(func() {
		r.mu.Lock()
		mine := r.dcs[key] == dc
		if mine {
			delete(r.dcs, key)
		}
		r.mu.Unlock()
		if mine {
			r.n.Hacer(func() { r.abajo(key) })
		}
	})
	dc.OnMessage(func(m webrtc.DataChannelMessage) {
		if !m.IsString {
			return
		}
		if whole, ok := re.Agregar(string(m.Data)); ok {
			r.n.Hacer(func() { r.n.Router.Recibir(VecinoNet(key), whole) })
		}
	})
}

// juntar waits for the host and STUN candidates (no trickle: one sealed signal each way opens the link).
func juntar(pc *webrtc.PeerConnection) {
	done := webrtc.GatheringCompletePromise(pc)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func (r *Red) crearOferta(key string) (string, error) {
	pc, err := r.nuevaPC(key)
	if err != nil {
		return "", err
	}
	ordered := true
	dc, err := pc.CreateDataChannel("ibn", &webrtc.DataChannelInit{Ordered: &ordered})
	if err != nil {
		return "", err
	}
	r.cablear(key, dc)
	o, err := pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}
	if err := pc.SetLocalDescription(o); err != nil {
		return "", err
	}
	juntar(pc)
	return pc.LocalDescription().SDP, nil
}

func (r *Red) crearRespuesta(key, offer string) (string, error) {
	pc, err := r.nuevaPC(key)
	if err != nil {
		return "", err
	}
	pc.OnDataChannel(func(dc *webrtc.DataChannel) { r.cablear(key, dc) })
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		r.cerrar(key)
		return "", err
	}
	a, err := pc.CreateAnswer(nil)
	if err != nil {
		r.cerrar(key)
		return "", err
	}
	if err := pc.SetLocalDescription(a); err != nil {
		r.cerrar(key)
		return "", err
	}
	juntar(pc)
	return pc.LocalDescription().SDP, nil
}

func (r *Red) completar(key, answer string) {
	r.mu.Lock()
	pc := r.pcs[key]
	r.mu.Unlock()
	if pc == nil {
		return
	}
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		r.cerrar(key)
	}
}

func (r *Red) cerrar(key string) {
	r.mu.Lock()
	dc, pc := r.dcs[key], r.pcs[key]
	delete(r.dcs, key)
	delete(r.voz, key)
	delete(r.pcs, key)
	r.mu.Unlock()
	if dc != nil {
		go dc.Close()
	}
	if pc != nil {
		go pc.Close()
	}
}

// Enviar sends a mesh message to the neighbor "net:" + key prefix, in parts when big.
func (r *Red) Enviar(vecino, js string) {
	r.mu.Lock()
	key := r.porV[vecino]
	dc := r.dcs[key]
	r.mu.Unlock()
	if dc == nil || dc.ReadyState() != webrtc.DataChannelStateOpen {
		return
	}
	for _, part := range Partir(js, fragmento) {
		dc.SendText(part)
	}
}

// EnviarVoz sends one voice frame to another Go device over the voice channel; false without one open.
func (r *Red) EnviarVoz(key string, b []byte) bool {
	r.mu.Lock()
	dv := r.voz[key]
	r.mu.Unlock()
	if dv == nil || dv.ReadyState() != webrtc.DataChannelStateOpen {
		return false
	}
	return dv.Send(b) == nil
}

// Abiertos: how many links are open now.
func (r *Red) Abiertos() int { return len(r.Enlazados()) }
