package voz

import (
	"bytes"
	"encoding/base64"
	"errors"
	"hash/fnv"
	"image"
	"image/draw"
	"image/jpeg"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/nicoconvoz/zonda/zondago/nucleo"
)

// The screen, the way the web shares it: captured, converted and scaled by libyuv, encoded in VP8 by libvpx (the
// components libwebrtc uses, video_windows.go) and sent as a video track on each member's connection, which is
// renegotiated to carry it, as the app does. The app's screen arrives the same way and is decoded whole by libvpx.
//
// Between Go devices (no media connection: the tree) the screen still travels as JPEG tiles of what changed.

const (
	lado     = 64 // tile side, pixels
	anchoMax = 1280
	// Criterion, not a measurement: 5 frames a second, what a screen of text and windows needs. Measured on the
	// Captain's computer: a capture takes about 30 ms and a VP8 frame 6 ms, so sharing costs about 18 % of one core.
	cadaCaptura  = 200 * time.Millisecond
	kbpsPantalla = 1500
	calidad      = 70
)

type pantalla struct {
	s  *Sala
	mu sync.Mutex

	on          bool
	parada      chan struct{}
	huellas     []uint64
	ancho, alto int
	nuevos      map[string]bool // who must get the whole screen

	ajenas map[string]*image.RGBA

	// The VP8 side: the video track every connection carries while sharing, and its encoder.
	pista      *webrtc.TrackLocalStaticSample
	cod        *CodificadorVP8
	envios     map[*webrtc.PeerConnection]*webrtc.RTPSender
	pedirClave bool
}

func nuevaPantalla(s *Sala) *pantalla {
	return &pantalla{s: s, nuevos: map[string]bool{}, ajenas: map[string]*image.RGBA{}, envios: map[*webrtc.PeerConnection]*webrtc.RTPSender{}}
}

var vp8Pantalla = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}

// agregar puts the screen's track on a connection and keeps reading its feedback: a picture loss asks for a key frame.
func (p *pantalla) agregar(pc *webrtc.PeerConnection) bool {
	p.mu.Lock()
	pista := p.pista
	if pista == nil || p.envios[pc] != nil {
		p.mu.Unlock()
		return false
	}
	p.mu.Unlock()
	snd, err := pc.AddTrack(pista)
	if err != nil {
		return false
	}
	p.mu.Lock()
	p.envios[pc] = snd
	p.mu.Unlock()
	go func() {
		for {
			pkts, _, err := snd.ReadRTCP()
			if err != nil {
				return
			}
			for _, k := range pkts {
				switch k.(type) {
				case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
					p.mu.Lock()
					p.pedirClave = true
					p.mu.Unlock()
				}
			}
		}
	}()
	return true
}

func (p *pantalla) compartiendo() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.on
}

func (p *pantalla) alternar() error {
	if p.compartiendo() {
		p.parar()
		return nil
	}
	if p.s.Canal() == "" {
		return errSinCanal
	}
	img, err := capturar()
	if err != nil {
		return errors.New("no se pudo capturar la pantalla")
	}
	b := reducir(img).Bounds()
	cod, err := NuevoCodificadorVP8(b.Dx(), b.Dy(), int(time.Second/cadaCaptura), kbpsPantalla)
	if err != nil {
		return errors.New("no cargó el video de WebRTC: " + err.Error())
	}
	pista, err := webrtc.NewTrackLocalStaticSample(vp8Pantalla, "video", "zonda-pantalla")
	if err != nil {
		cod.Cerrar()
		return err
	}
	p.mu.Lock()
	p.on, p.parada, p.huellas = true, make(chan struct{}), nil
	p.pista, p.cod, p.pedirClave = pista, cod, true
	parada := p.parada
	p.mu.Unlock()
	// Every media connection now carries the screen: renegotiated with a new offer, as the app does.
	for peer, c := range p.s.conexionesConectadas() {
		if p.agregar(c.pc) {
			go p.s.renegociar(peer, c)
		}
	}
	go p.bucle(parada)
	return nil
}

func (p *pantalla) parar() {
	p.mu.Lock()
	if !p.on {
		p.mu.Unlock()
		return
	}
	p.on = false
	close(p.parada)
	envios, cod := p.envios, p.cod
	p.envios, p.pista, p.cod = map[*webrtc.PeerConnection]*webrtc.RTPSender{}, nil, nil
	p.mu.Unlock()
	canal := p.s.Canal()
	for _, k := range p.s.pares() {
		p.s.senal(k, nucleo.Msg{"kind": "v-unshare", "room": canal})
	}
	// The track leaves every connection (renegotiated) and the others are told the screen stopped.
	for peer, c := range p.s.conexionesConectadas() {
		if snd := envios[c.pc]; snd != nil {
			c.pc.RemoveTrack(snd)
			go p.s.renegociar(peer, c)
		}
		p.s.senal(peer, nucleo.Msg{"kind": "v-unshare", "room": canal})
	}
	go func() {
		time.Sleep(cadaCaptura)
		cod.Cerrar()
	}()
}

func (p *pantalla) alConectar(peer string) {
	p.mu.Lock()
	p.nuevos[peer] = true
	p.mu.Unlock()
}

func (p *pantalla) quitar(peer string) {
	p.mu.Lock()
	delete(p.ajenas, peer)
	p.mu.Unlock()
}

func (p *pantalla) bucle(parada chan struct{}) {
	t := time.NewTicker(cadaCaptura)
	defer t.Stop()
	for {
		if img, err := capturar(); err == nil {
			p.enviarVideo(img)
			if len(p.s.pares()) > 0 {
				p.enviarCambios(img)
			}
		}
		select {
		case <-parada:
			return
		case <-t.C:
		}
	}
}

func reducir(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	if b.Dx() <= anchoMax {
		return src
	}
	w, h := anchoMax, b.Dy()*anchoMax/b.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := y * b.Dy() / h
		for x := 0; x < w; x++ {
			sx := x * b.Dx() / w
			i, j := dst.PixOffset(x, y), src.PixOffset(sx, sy)
			copy(dst.Pix[i:i+4], src.Pix[j:j+4])
		}
	}
	return dst
}

// enviarVideo encodes one frame in VP8 and writes it on the screen's track (a key frame when someone lost the picture).
func (p *pantalla) enviarVideo(img *image.RGBA) {
	p.mu.Lock()
	cod, pista, clave := p.cod, p.pista, p.pedirClave
	p.pedirClave = false
	p.mu.Unlock()
	if cod == nil || pista == nil {
		return
	}
	vp8, _, err := cod.Cuadro(img, clave, int(cadaCaptura/time.Millisecond))
	if err != nil || len(vp8) == 0 {
		return
	}
	pista.WriteSample(media.Sample{Data: vp8, Duration: cadaCaptura})
}

// enviarCambios finds the tiles whose fingerprint changed and sends them to the Go devices (all of them to newcomers).
func (p *pantalla) enviarCambios(img *image.RGBA) {
	img = reducir(img)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	cols, filas := (w+lado-1)/lado, (h+lado-1)/lado
	p.mu.Lock()
	if p.ancho != w || p.alto != h || len(p.huellas) != cols*filas {
		p.ancho, p.alto, p.huellas = w, h, make([]uint64, cols*filas)
	}
	nuevos := p.nuevos
	p.nuevos = map[string]bool{}
	p.mu.Unlock()
	var cambiados, todos []any
	for ty := 0; ty < filas; ty++ {
		for tx := 0; tx < cols; tx++ {
			r := image.Rect(tx*lado, ty*lado, min((tx+1)*lado, w), min((ty+1)*lado, h))
			hs := fnv.New64a()
			for y := r.Min.Y; y < r.Max.Y; y++ {
				i := img.PixOffset(r.Min.X, y)
				hs.Write(img.Pix[i : i+r.Dx()*4])
			}
			f := hs.Sum64()
			idx := ty*cols + tx
			cambio := p.huellas[idx] != f
			p.huellas[idx] = f
			if !cambio && len(nuevos) == 0 {
				continue
			}
			var b bytes.Buffer
			jpeg.Encode(&b, img.SubImage(r), &jpeg.Options{Quality: calidad})
			m := nucleo.Msg{"x": r.Min.X, "y": r.Min.Y, "j": base64.StdEncoding.EncodeToString(b.Bytes())}
			todos = append(todos, m)
			if cambio {
				cambiados = append(cambiados, m)
			}
		}
	}
	canal := p.s.Canal()
	for _, k := range p.s.pares() {
		ts := cambiados
		if nuevos[k] {
			ts = todos
		}
		if len(ts) == 0 {
			continue
		}
		// In batches, so no single sealed signal grows past what a link carries comfortably.
		for i := 0; i < len(ts); i += 24 {
			m := nucleo.Msg{"kind": "zp", "room": canal, "w": w, "h": h, "t": ts[i:min(i+24, len(ts))]}
			p.s.n.Hacer(func() {
				if !p.s.n.SenalDirecta(k, m) {
					p.s.n.Red.Querer(k)
					p.alConectar(k) // the whole screen once the link is up
				}
			})
		}
	}
}

// alMosaicos paints the tiles another Go device sent onto its screen.
func (p *pantalla) alMosaicos(from string, d nucleo.Msg) {
	if room, _ := nucleo.Texto(d, "room"); room != p.s.Canal() || room == "" {
		return
	}
	w, _ := nucleo.Entero(d, "w")
	h, _ := nucleo.Entero(d, "h")
	if w <= 0 || h <= 0 || w > 8000 || h > 8000 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	lienzo := p.ajenas[from]
	if lienzo == nil || lienzo.Bounds().Dx() != int(w) || lienzo.Bounds().Dy() != int(h) {
		lienzo = image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	}
	ts, _ := nucleo.Lista(d, "t")
	for _, x := range ts {
		m, _ := x.(map[string]any)
		tx, _ := nucleo.Entero(m, "x")
		ty, _ := nucleo.Entero(m, "y")
		raw, err := base64.StdEncoding.DecodeString(nucleo.Interpolar(m["j"]))
		if err != nil {
			continue
		}
		t, err := jpeg.Decode(bytes.NewReader(raw))
		if err != nil {
			continue
		}
		draw.Draw(lienzo, t.Bounds().Add(image.Pt(int(tx), int(ty))), t, t.Bounds().Min, draw.Src)
	}
	// A new image each time: the screen shows a fresh texture only for a new image.
	copia := image.NewRGBA(lienzo.Bounds())
	copy(copia.Pix, lienzo.Pix)
	p.ajenas[from] = copia
}

func (p *pantalla) vistas() map[string]image.Image {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]image.Image{}
	for k, v := range p.ajenas {
		out[k] = v
	}
	return out
}

// recibirVideo decodes a VP8 screen from the app whole with libvpx; when the decoder loses its reference (a packet
// lost), it asks for a key frame (PLI), at most once a second.
func (p *pantalla) recibirVideo(peer string, pc *webrtc.PeerConnection, t *webrtc.TrackRemote) {
	if t.Codec().MimeType != webrtc.MimeTypeVP8 {
		return
	}
	dec, err := NuevoDecodificadorVP8()
	if err != nil {
		return
	}
	defer dec.Cerrar()
	var ultimoPedido time.Time
	pedir := func() {
		if time.Since(ultimoPedido) > time.Second {
			ultimoPedido = time.Now()
			pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(t.SSRC())}})
		}
	}
	pedir()
	var cuadro []byte
	var vp codecs.VP8Packet
	for {
		pk, _, err := t.ReadRTP()
		if err != nil {
			return
		}
		payload, err := vp.Unmarshal(pk.Payload)
		if err != nil {
			continue
		}
		if vp.S == 1 && vp.PID == 0 {
			cuadro = cuadro[:0]
		}
		cuadro = append(cuadro, payload...)
		if !pk.Marker {
			continue
		}
		img, err := dec.Cuadro(cuadro)
		cuadro = cuadro[:0]
		if err != nil {
			pedir()
			continue
		}
		if img == nil {
			continue
		}
		p.mu.Lock()
		p.ajenas[peer] = img
		p.mu.Unlock()
		p.s.n.Hacer(func() { p.s.n.OnCambio() })
	}
}
