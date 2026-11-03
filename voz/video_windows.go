//go:build windows

package voz

import (
	"errors"
	"image"
	"sync"
	"syscall"
	"unsafe"
)

// The video components libwebrtc uses to share a screen, the real ones: libvpx (VP8) and libyuv (conversion and
// scaling), reached through video/zvid.cc and called with syscall, as the audio processing is.

var videoCargado struct {
	once                           sync.Once
	err                            error
	codCrear, codCuadro, codCerrar uintptr
	decCrear, decCuadro, decCerrar uintptr
}

func cargarVideo() error {
	v := &videoCargado
	v.once.Do(func() {
		dir, err := carpetaDLL()
		if err != nil {
			v.err = err
			return
		}
		dll, err := cargar(dir, "zvid.dll")
		if err != nil {
			v.err = err
			return
		}
		for _, f := range []struct {
			n string
			p *uintptr
		}{{"zvid_cod_crear", &v.codCrear}, {"zvid_cod_cuadro", &v.codCuadro}, {"zvid_cod_cerrar", &v.codCerrar},
			{"zvid_dec_crear", &v.decCrear}, {"zvid_dec_cuadro", &v.decCuadro}, {"zvid_dec_cerrar", &v.decCerrar}} {
			if *f.p, v.err = funcion(dll, f.n); v.err != nil {
				return
			}
		}
	})
	return v.err
}

// CodificadorVP8 turns RGBA frames into VP8 frames at a fixed size.
type CodificadorVP8 struct {
	h    uintptr
	W, H int
	out  []byte
}

func NuevoCodificadorVP8(w, h, fps, kbps int) (*CodificadorVP8, error) {
	if err := cargarVideo(); err != nil {
		return nil, err
	}
	w, h = w&^1, h&^1
	r, _, _ := syscall.SyscallN(videoCargado.codCrear, uintptr(w), uintptr(h), uintptr(fps), uintptr(kbps))
	if r == 0 {
		return nil, errors.New("libvpx no creó el codificador")
	}
	return &CodificadorVP8{h: r, W: w, H: h, out: make([]byte, 4<<20)}, nil
}

// Cuadro encodes one frame (scaled to the encoder's size); nil when the encoder skipped it.
func (c *CodificadorVP8) Cuadro(img *image.RGBA, forzarClave bool, ms int) (vp8 []byte, clave bool, err error) {
	var k int32
	f := 0
	if forzarClave {
		f = 1
	}
	b := img.Bounds()
	n, _, _ := syscall.SyscallN(videoCargado.codCuadro, c.h, uintptr(unsafe.Pointer(&img.Pix[0])), uintptr(b.Dx()), uintptr(b.Dy()),
		uintptr(f), uintptr(unsafe.Pointer(&c.out[0])), uintptr(len(c.out)), uintptr(unsafe.Pointer(&k)), uintptr(ms))
	if int32(n) < 0 {
		return nil, false, errors.New("libvpx no pudo codificar")
	}
	if n == 0 {
		return nil, false, nil
	}
	return append([]byte(nil), c.out[:n]...), k == 1, nil
}

func (c *CodificadorVP8) Cerrar() {
	if c != nil && c.h != 0 {
		syscall.SyscallN(videoCargado.codCerrar, c.h)
		c.h = 0
	}
}

// DecodificadorVP8 turns VP8 frames back into pictures.
type DecodificadorVP8 struct {
	h   uintptr
	buf []byte
}

func NuevoDecodificadorVP8() (*DecodificadorVP8, error) {
	if err := cargarVideo(); err != nil {
		return nil, err
	}
	r, _, _ := syscall.SyscallN(videoCargado.decCrear)
	if r == 0 {
		return nil, errors.New("libvpx no creó el decodificador")
	}
	return &DecodificadorVP8{h: r, buf: make([]byte, 1920*1080*4)}, nil
}

// ErrNecesitaClave: the decoder lost its reference; a key frame (asked for with a PLI) puts it back.
var ErrNecesitaClave = errors.New("hace falta un cuadro clave")

// Cuadro decodes one VP8 frame; nil, nil when it gave no picture.
func (d *DecodificadorVP8) Cuadro(vp8 []byte) (*image.RGBA, error) {
	if len(vp8) == 0 {
		return nil, nil
	}
	for {
		var w, h int32
		r, _, _ := syscall.SyscallN(videoCargado.decCuadro, d.h, uintptr(unsafe.Pointer(&vp8[0])), uintptr(len(vp8)),
			uintptr(unsafe.Pointer(&d.buf[0])), uintptr(len(d.buf)), uintptr(unsafe.Pointer(&w)), uintptr(unsafe.Pointer(&h)))
		switch int32(r) {
		case 1:
			img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
			copy(img.Pix, d.buf[:int(w)*int(h)*4])
			return img, nil
		case 0:
			return nil, nil
		case -2:
			// Bigger than the buffer: grow it; the frame decodes again from the same data (VP8 frames are whole).
			d.buf = make([]byte, int(w)*int(h)*4)
			continue
		}
		return nil, ErrNecesitaClave
	}
}

func (d *DecodificadorVP8) Cerrar() {
	if d != nil && d.h != 0 {
		syscall.SyscallN(videoCargado.decCerrar, d.h)
		d.h = 0
	}
}
