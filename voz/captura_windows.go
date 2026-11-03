//go:build windows

package voz

import (
	"errors"
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

// The primary screen through GDI (BitBlt into a DIB), pure Go.

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	gdi32            = syscall.NewLazyDLL("gdi32.dll")
	getDC            = user32.NewProc("GetDC")
	releaseDC        = user32.NewProc("ReleaseDC")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
	createCompatDC   = gdi32.NewProc("CreateCompatibleDC")
	selectObject     = gdi32.NewProc("SelectObject")
	bitBlt           = gdi32.NewProc("BitBlt")
	createDIBSection = gdi32.NewProc("CreateDIBSection")
	deleteObject     = gdi32.NewProc("DeleteObject")
	deleteDC         = gdi32.NewProc("DeleteDC")
)

type bitmapInfoHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}

// capturar draws the screen into a DIB section of its own (memory this program can read directly) and copies it out:
// GetDIBits on a compatible bitmap failed silently on the Captain's computer.
func capturar() (*image.RGBA, error) {
	w, _, _ := getSystemMetrics.Call(0)
	h, _, _ := getSystemMetrics.Call(1)
	if w == 0 || h == 0 {
		return nil, errors.New("sin pantalla")
	}
	scr, _, _ := getDC.Call(0)
	if scr == 0 {
		return nil, errors.New("sin contexto de pantalla")
	}
	defer releaseDC.Call(0, scr)
	mem, _, _ := createCompatDC.Call(scr)
	if mem == 0 {
		return nil, errors.New("sin contexto en memoria")
	}
	defer deleteDC.Call(mem)
	hdr := bitmapInfoHeader{Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	hdr.Size = uint32(unsafe.Sizeof(hdr))
	var bits unsafe.Pointer
	bmp, _, e := createDIBSection.Call(scr, uintptr(unsafe.Pointer(&hdr)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return nil, fmt.Errorf("no se pudo preparar la captura: %v", e)
	}
	defer deleteObject.Call(bmp)
	old, _, _ := selectObject.Call(mem, bmp)
	const srccopy, captureblt = 0x00CC0020, 0x40000000
	r, _, e := bitBlt.Call(mem, 0, 0, w, h, scr, 0, 0, srccopy|captureblt)
	selectObject.Call(mem, old)
	if r == 0 {
		return nil, fmt.Errorf("no se pudo copiar la pantalla: %v", e)
	}
	n := int(w) * int(h) * 4
	src := unsafe.Slice((*byte)(bits), n)
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	// GDI gives BGRA: swap to RGBA, opaque.
	p := img.Pix
	for i := 0; i < n; i += 4 {
		p[i], p[i+1], p[i+2], p[i+3] = src[i+2], src[i+1], src[i], 0xff
	}
	return img, nil
}
