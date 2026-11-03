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
	"golang.org/x/image/vp8"

	"github.com/nicoconvoz/zonda/zondago/nucleo"
)

// The screen, the way RDP and VNC send it: a shared screen is mostly still, so only the tiles that changed travel,
// each one a JPEG (standard library), as a sealed signal to each member of the voice channel. A device that just
// connected gets the whole screen once.
//
// A screen shared from the Flutter app arrives as VP8 video; Go has no VP8 encoder or full decoder in pure Go, so
// only its key frames are decoded (golang.org/x/image/vp8 decodes key frames), asked for once a second: about one
// frame per second.

const (
	lado       = 64 // tile side, pixels
	anchoMax   = 1280
	cadaCaptura = 300 * time.Millisecond // criterion, not a measurement: about 3 frames a second while sharing
	calidad    = 70
)

type pantalla struct {
	s  *Sala
	mu sync.Mutex

	on      bool
	parada  chan struct{}
	huellas []uint64
	ancho, alto int
	nuevos  map[string]bool // who must get the whole screen

	ajenas map[string]*image.RGBA
}

func nuevaPantalla(s *Sala) *pantalla {
	return &pantalla{s: s, nuevos: map[string]bool{}, ajenas: map[string]*image.RGBA{}}
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
	if _, err := capturar(); err != nil {
		return errors.New("no se pudo capturar la pantalla")
	}
	p.mu.Lock()
	p.on, p.parada, p.huellas = true, make(chan struct{}), nil
	parada := p.parada
	p.mu.Unlock()
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
	p.mu.Unlock()
	canal := p.s.Canal()
	for _, k := range p.s.pares() {
		p.s.senal(k, nucleo.Msg{"kind": "v-unshare", "room": canal})
	}
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
		p.enviarCambios()
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

// enviarCambios captures, finds the tiles whose fingerprint changed, and sends them (all of them to newcomers).
func (p *pantalla) enviarCambios() {
	img, err := capturar()
	if err != nil {
		return
	}
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

// recibirVideo decodes the key frames of a VP8 screen from the app, asking for one a second (PLI).
func (p *pantalla) recibirVideo(peer string, pc *webrtc.PeerConnection, t *webrtc.TrackRemote) {
	if t.Codec().MimeType != webrtc.MimeTypeVP8 {
		return
	}
	pedir := time.NewTicker(time.Second)
	defer pedir.Stop()
	go func() {
		for range pedir.C {
			if pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(t.SSRC())}}) != nil {
				return
			}
		}
	}()
	var cuadro []byte
	var vp codecs.VP8Packet
	dec := vp8.NewDecoder()
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
		if !pk.Marker || len(cuadro) < 3 || cuadro[0]&1 != 0 { // bit 0 of the first byte: 0 = key frame
			continue
		}
		dec.Init(bytes.NewReader(cuadro), len(cuadro))
		if _, err := dec.DecodeFrameHeader(); err != nil {
			continue
		}
		img, err := dec.DecodeFrame()
		if err != nil {
			continue
		}
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
		p.mu.Lock()
		p.ajenas[peer] = rgba
		p.mu.Unlock()
		p.s.n.Hacer(func() { p.s.n.OnCambio() })
	}
}
