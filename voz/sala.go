package voz

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/tphakala/go-opus/opus"

	"github.com/nicoconvoz/zonda/zondago/nucleo"
)

// Sala is the voice of one channel of a room: a connection with each member in it (the lower key offers), the same
// signals as the app (v-hello, v-here, v-offer, v-answer, v-ice, v-reoffer, v-reanswer, v-unshare, v-bye).
type Sala struct {
	n  *nucleo.Nodo
	mu sync.Mutex

	canal    string
	pcs      map[string]*conexion
	pista    *webrtc.TrackLocalStaticSample
	mic      *Microfono
	parlante *Parlante
	hablando bool
	api      *webrtc.API

	pantalla *pantalla
	enc      *opus.Encoder
	proc     *Procesador
	comp     *Compuerta

	// The Go devices in the channel: their voice goes by the relay tree over the data links (arbol.go).
	goPares map[string]bool
	fuentes map[string]*Fuente
	vistos  map[string]map[uint32]bool
	seq     uint32
}

type conexion struct {
	pc         *webrtc.PeerConnection
	remota     bool
	pendientes []webrtc.ICECandidateInit
	sonido     *Fuente
	conectada  bool
	segunda    bool
}

// opusVoz: Opus at 48 kHz as WebRTC names it (always "/2" in the SDP, mono content), with in-band FEC.
var opusVoz = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1"}

// Bitrate of the voice: about 40 kbps on the wire with RTP and UDP, the Captain's figure for the relay tree.
const bitrateVoz = 32000

func nuevoDecodificador() *opus.Decoder {
	d, _ := opus.NewDecoder(48000, 1)
	return d
}

// decodificar turns one Opus packet into 48 kHz mono samples (nil when it cannot be read).
func decodificar(d *opus.Decoder, pkt []byte) []int16 {
	if d == nil {
		return nil
	}
	out := make([]int16, 5760) // up to 120 ms, the longest Opus packet
	n, err := d.Decode(pkt, out)
	if err != nil || n <= 0 {
		return nil
	}
	return out[:n]
}

const maxPares = 12

func NuevaSala(n *nucleo.Nodo) *Sala {
	m := &webrtc.MediaEngine{}
	// Opus for the voice, the codec the app and every browser use (pure Go: go-opus, a port of libopus). Video codecs
	// are registered to receive a screen shared from the app (its renegotiation must not fail).
	m.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: opusVoz, PayloadType: 111}, webrtc.RTPCodecTypeAudio)
	m.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, PayloadType: 96}, webrtc.RTPCodecTypeVideo)
	s := &Sala{n: n, pcs: map[string]*conexion{}, api: webrtc.NewAPI(webrtc.WithMediaEngine(m)), goPares: map[string]bool{}, fuentes: map[string]*Fuente{}, vistos: map[string]map[uint32]bool{}}
	n.Red.OnVoz = s.alCuadro
	s.pantalla = nuevaPantalla(s)
	n.OnSenal = s.alSenal
	return s
}

func (s *Sala) senal(to string, d nucleo.Msg) {
	s.n.Hacer(func() { s.n.EnviarSenal(to, d) })
}

func (s *Sala) Canal() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.canal
}

func (s *Sala) Hablando() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hablando
}

func (s *Sala) AlternarHablar() {
	s.mu.Lock()
	s.hablando = !s.hablando
	s.mu.Unlock()
}

func (s *Sala) Conectados() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := 0
	for _, x := range s.pcs {
		if x.conectada {
			c++
		}
	}
	return c + len(s.goPares)
}

// Entrar opens the channel's audio (listening; the microphone is latched with Hablar) and greets its members.
func (s *Sala) Entrar(canal string, miembros []string) error {
	s.Salir()
	pista, err := webrtc.NewTrackLocalStaticSample(opusVoz, "audio", "zonda")
	if err != nil {
		return err
	}
	enc, err := opus.NewEncoder(opus.EncoderConfig{SampleRate: 48000, Channels: 1, Bitrate: bitrateVoz, ConstrainedVBR: true, Complexity: 5})
	if err != nil {
		return err
	}
	parlante, err := AbrirParlante(s.mezclar)
	if err != nil {
		return err
	}
	mic, err := AbrirMicrofono(s.alMicrofono)
	if err != nil {
		parlante.Cerrar()
		return err
	}
	s.mu.Lock()
	s.canal, s.pista, s.mic, s.parlante, s.hablando = canal, pista, mic, parlante, false
	s.enc, s.proc, s.comp = enc, NuevoProcesador(), NuevaCompuerta()
	s.goPares, s.fuentes, s.vistos = map[string]bool{}, map[string]*Fuente{}, map[string]map[uint32]bool{}
	s.mu.Unlock()
	for _, k := range miembros {
		s.senal(k, nucleo.Msg{"kind": "v-hello", "room": canal, "zg": 1})
	}
	return nil
}

// Medidas: the voice's live counters, for the room.
func (s *Sala) Medidas() string { return Medidas.Texto() }

// Guardar writes the recording so far (on leaving the channel and on closing the window).
func (s *Sala) Guardar() { volcar(filepath.Join(os.Getenv("LOCALAPPDATA"), "ZondaGo", "grabaciones")) }

func (s *Sala) Salir() {
	s.Guardar()
	s.pantalla.parar()
	s.mu.Lock()
	canal, pcs, mic, parlante := s.canal, s.pcs, s.mic, s.parlante
	gos := s.goPares
	s.canal, s.pcs, s.mic, s.parlante, s.hablando = "", map[string]*conexion{}, nil, nil, false
	s.goPares, s.fuentes = map[string]bool{}, map[string]*Fuente{}
	s.mu.Unlock()
	for k, c := range pcs {
		s.senal(k, nucleo.Msg{"kind": "v-bye", "room": canal})
		if c.pc != nil {
			go c.pc.Close()
		}
	}
	for k := range gos {
		s.senal(k, nucleo.Msg{"kind": "v-bye", "room": canal})
	}
	if mic != nil {
		go mic.Cerrar()
	}
	if parlante != nil {
		go parlante.Cerrar()
	}
	s.n.Hacer(func() { s.n.OnCambio() })
}

func (s *Sala) alMicrofono(pcm []int16) {
	s.mu.Lock()
	on, pista, canal, enc, proc, comp := s.hablando, s.pista, s.canal, s.enc, s.proc, s.comp
	s.seq++
	seq, miembros := s.seq, s.miembrosGo()
	s.mu.Unlock()
	if pista != nil {
		anotar("microfono-crudo", pcm)
	}
	if !on || pista == nil || enc == nil || len(pcm) != Muestras {
		return
	}
	// The hum and the hiss go first; the automatic gain then brings up the voice, not the noise.
	proc.Procesar(pcm)
	comp.Limpiar(pcm)
	anotar("microfono-enviado", pcm)
	buf := make([]byte, 1500)
	n, err := enc.Encode(pcm, buf)
	if err != nil || n <= 0 {
		return
	}
	mu := buf[:n]
	pista.WriteSample(media.Sample{Data: mu, Duration: 20 * time.Millisecond})
	if len(miembros) > 1 {
		me := s.n.Clave()
		primeros, _ := Arbol(me, miembros)
		cuadro := armarCuadro(canal, me, seq, mu)
		for _, p := range primeros {
			s.n.Red.EnviarVoz(p, cuadro)
		}
	}
}

// miembrosGo: the Go devices of the channel and me (with s.mu held).
func (s *Sala) miembrosGo() []string {
	out := []string{s.n.Clave()}
	for k := range s.goPares {
		out = append(out, k)
	}
	return out
}

// alCuadro: a voice frame from the tree: played, and forwarded to my share when I am in the speaker's first tier.
func (s *Sala) alCuadro(de string, b []byte) {
	ch, src, seq, mu, ok := leerCuadro(b)
	if !ok || src == s.n.Clave() {
		return
	}
	s.mu.Lock()
	if s.canal == "" || ch != huellaCanal(s.canal) {
		s.mu.Unlock()
		return
	}
	v := s.vistos[src]
	if v == nil || len(v) > 500 {
		v = map[uint32]bool{}
		s.vistos[src] = v
	}
	if v[seq] {
		s.mu.Unlock()
		return
	}
	v[seq] = true
	f := s.fuentes[src]
	if f == nil {
		f = nuevaFuente()
		s.fuentes[src] = f
	}
	_, hijos := Arbol(src, append(s.miembrosGo(), src))
	s.mu.Unlock()
	f.Llego(mu)
	for _, h := range hijos[s.n.Clave()] {
		if h != de {
			s.n.Red.EnviarVoz(h, b)
		}
	}
}

// mezclar adds one 20 ms frame of every member who sent one.
func (s *Sala) mezclar(out []int16) {
	s.mu.Lock()
	var fuentes []*Fuente
	for _, c := range s.pcs {
		if c.sonido != nil {
			fuentes = append(fuentes, c.sonido)
		}
	}
	for _, f := range s.fuentes {
		fuentes = append(fuentes, f)
	}
	s.mu.Unlock()
	acc := make([]int32, len(out))
	for _, f := range fuentes {
		fr := f.Siguiente()
		for i := 0; i < len(fr) && i < len(acc); i++ {
			acc[i] += int32(fr[i])
		}
	}
	for i, v := range acc {
		out[i] = limitar(float64(v))
	}
	anotar("parlante", out)
}

func (s *Sala) nuevaConexion(peer string) (*conexion, error) {
	pc, err := s.api.NewPeerConnection(webrtc.Configuration{ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302", "stun:stun.cloudflare.com:3478"}}}})
	if err != nil {
		return nil, err
	}
	c := &conexion{pc: pc, sonido: nuevaFuente()}
	pc.OnTrack(func(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if t.Kind() == webrtc.RTPCodecTypeVideo {
			go s.pantalla.recibirVideo(peer, pc, t)
			return
		}
		go func() {
			for {
				p, _, err := t.ReadRTP()
				if err != nil {
					return
				}
				if len(p.Payload) > 0 {
					c.sonido.Llego(p.Payload)
				}
			}
		}()
	})
	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		s.mu.Lock()
		if s.pcs[peer] == c {
			c.conectada = st == webrtc.PeerConnectionStateConnected
			if st == webrtc.PeerConnectionStateFailed {
				delete(s.pcs, peer)
				go pc.Close()
			}
		}
		s.mu.Unlock()
		if st == webrtc.PeerConnectionStateConnected {
			s.pantalla.alConectar(peer)
		}
		s.n.Hacer(func() { s.n.OnCambio() })
	})
	s.mu.Lock()
	if s.pista != nil {
		pc.AddTrack(s.pista)
	}
	s.mu.Unlock()
	// A connection not up in 12 s lost a signal on the way: dropped, and one more hello.
	time.AfterFunc(12*time.Second, func() {
		s.mu.Lock()
		vieja := s.pcs[peer] == c && !c.conectada
		canal, segunda := s.canal, c.segunda
		if vieja {
			delete(s.pcs, peer)
		}
		s.mu.Unlock()
		if vieja {
			pc.Close()
			if !segunda && canal != "" {
				s.senal(peer, nucleo.Msg{"kind": "v-hello", "room": canal, "2": 1})
			}
		}
	})
	return c, nil
}

func juntar(pc *webrtc.PeerConnection) {
	select {
	case <-webrtc.GatheringCompletePromise(pc):
	case <-time.After(2 * time.Second):
	}
}

func (s *Sala) ocupado(peer string) bool {
	_, ok := s.pcs[peer]
	return ok
}

func (s *Sala) ofrecer(peer string) {
	s.mu.Lock()
	if s.ocupado(peer) || len(s.pcs) >= maxPares || s.canal == "" {
		s.mu.Unlock()
		return
	}
	canal := s.canal
	s.pcs[peer] = &conexion{sonido: nuevaFuente()} // reserved before any wait
	s.mu.Unlock()
	c, err := s.nuevaConexion(peer)
	if err != nil {
		s.quitar(peer)
		return
	}
	s.mu.Lock()
	s.pcs[peer] = c
	s.mu.Unlock()
	o, err := c.pc.CreateOffer(nil)
	if err != nil || c.pc.SetLocalDescription(o) != nil {
		s.quitar(peer)
		return
	}
	juntar(c.pc)
	s.senal(peer, nucleo.Msg{"kind": "v-offer", "room": canal, "sdp": c.pc.LocalDescription().SDP})
}

func (s *Sala) responder(peer, sdp string, me string) {
	s.mu.Lock()
	if s.ocupado(peer) {
		if me < peer {
			s.mu.Unlock()
			return // both offered: the lower key's offer wins
		}
		if c := s.pcs[peer]; c.pc != nil {
			go c.pc.Close()
		}
		delete(s.pcs, peer)
	}
	if len(s.pcs) >= maxPares || s.canal == "" {
		s.mu.Unlock()
		return
	}
	canal := s.canal
	s.pcs[peer] = &conexion{sonido: nuevaFuente()}
	s.mu.Unlock()
	c, err := s.nuevaConexion(peer)
	if err != nil {
		s.quitar(peer)
		return
	}
	s.mu.Lock()
	s.pcs[peer] = c
	s.mu.Unlock()
	if c.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}) != nil {
		s.quitar(peer)
		return
	}
	s.vaciar(peer, c)
	a, err := c.pc.CreateAnswer(nil)
	if err != nil || c.pc.SetLocalDescription(a) != nil {
		s.quitar(peer)
		return
	}
	juntar(c.pc)
	s.senal(peer, nucleo.Msg{"kind": "v-answer", "room": canal, "sdp": c.pc.LocalDescription().SDP})
}

func (s *Sala) vaciar(peer string, c *conexion) {
	s.mu.Lock()
	c.remota = true
	pend := c.pendientes
	c.pendientes = nil
	s.mu.Unlock()
	for _, x := range pend {
		c.pc.AddICECandidate(x)
	}
}

func (s *Sala) quitar(peer string) {
	s.mu.Lock()
	c := s.pcs[peer]
	delete(s.pcs, peer)
	s.mu.Unlock()
	if c != nil && c.pc != nil {
		go c.pc.Close()
	}
	s.pantalla.quitar(peer)
}

// alSenal handles the voice signals (called on the node's loop: the slow work goes to its own goroutine).
func (s *Sala) alSenal(from string, d nucleo.Msg) {
	kind, _ := nucleo.Texto(d, "kind")
	if kind == "zp" {
		s.pantalla.alMosaicos(from, d)
		return
	}
	room, _ := nucleo.Texto(d, "room")
	s.mu.Lock()
	canal := s.canal
	c := s.pcs[from]
	s.mu.Unlock()
	if canal == "" || room != canal {
		return
	}
	me := s.n.Clave()
	sdp, _ := nucleo.Texto(d, "sdp")
	if zg, _ := nucleo.Entero(d, "zg"); zg == 1 && (kind == "v-hello" || kind == "v-here") {
		// Another Go device: no media connection, its voice comes by the tree over the data link.
		s.mu.Lock()
		nuevo := !s.goPares[from]
		s.goPares[from] = true
		s.mu.Unlock()
		if kind == "v-hello" || nuevo {
			s.senal(from, nucleo.Msg{"kind": "v-here", "room": canal, "zg": 1})
		}
		s.n.Red.Querer(from)
		s.pantalla.alConectar(from)
		s.n.OnCambio()
		return
	}
	s.mu.Lock()
	esGo := s.goPares[from]
	s.mu.Unlock()
	if esGo && kind != "v-bye" {
		return
	}
	switch kind {
	case "v-hello":
		s.senal(from, nucleo.Msg{"kind": "v-here", "room": canal})
		if me < from {
			go s.ofrecer(from)
		}
	case "v-here":
		if me < from {
			go s.ofrecer(from)
		}
	case "v-offer":
		if sdp != "" {
			go s.responder(from, sdp, me)
		}
	case "v-answer":
		if c == nil || c.pc == nil || c.remota || sdp == "" {
			return
		}
		go func() {
			if c.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdp}) == nil {
				s.vaciar(from, c)
			}
		}()
	case "v-ice":
		cand, _ := nucleo.Texto(d, "c")
		mid, _ := nucleo.Texto(d, "m")
		init := webrtc.ICECandidateInit{Candidate: cand, SDPMid: &mid}
		if i, ok := nucleo.Entero(d, "i"); ok {
			x := uint16(i)
			init.SDPMLineIndex = &x
		}
		if c == nil || c.pc == nil {
			return
		}
		s.mu.Lock()
		if !c.remota {
			c.pendientes = append(c.pendientes, init)
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
		go c.pc.AddICECandidate(init)
	case "v-reoffer":
		if c == nil || c.pc == nil || sdp == "" {
			return
		}
		go func() {
			if c.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}) != nil {
				return
			}
			a, err := c.pc.CreateAnswer(nil)
			if err != nil || c.pc.SetLocalDescription(a) != nil {
				return
			}
			s.senal(from, nucleo.Msg{"kind": "v-reanswer", "room": canal, "sdp": c.pc.LocalDescription().SDP})
		}()
	case "v-reanswer":
		if c != nil && c.pc != nil && sdp != "" {
			go c.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdp})
		}
	case "v-unshare":
		s.pantalla.quitar(from)
	case "v-bye":
		s.mu.Lock()
		delete(s.goPares, from)
		delete(s.fuentes, from)
		s.mu.Unlock()
		s.quitar(from)
	}
	s.n.OnCambio()
}

// pares: the Go devices of the channel (screen tiles only mean something to them).
func (s *Sala) pares() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k := range s.goPares {
		out = append(out, k)
	}
	return out
}

/* ---------- the screen (pantalla.go) ---------- */

func (s *Sala) Compartiendo() bool                { return s.pantalla.compartiendo() }
func (s *Sala) AlternarPantalla() error           { return s.pantalla.alternar() }
func (s *Sala) Pantallas() map[string]image.Image { return s.pantalla.vistas() }

var errSinCanal = errors.New("entrá a un canal de voz para compartir la pantalla")
