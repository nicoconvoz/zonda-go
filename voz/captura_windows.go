//go:build windows

package voz

import (
	"errors"
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
	createCompatBmp  = gdi32.NewProc("CreateCompatibleBitmap")
	selectObject     = gdi32.NewProc("SelectObject")
	bitBlt           = gdi32.NewProc("BitBlt")
	getDIBits        = gdi32.NewProc("GetDIBits")
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
	defer deleteDC.Call(mem)
	bmp, _, _ := createCompatBmp.Call(scr, w, h)
	defer deleteObject.Call(bmp)
	old, _, _ := selectObject.Call(mem, bmp)
	const srccopy = 0x00CC0020
	bitBlt.Call(mem, 0, 0, w, h, scr, 0, 0, srccopy)
	selectObject.Call(mem, old)
	hdr := bitmapInfoHeader{Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	hdr.Size = uint32(unsafe.Sizeof(hdr))
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	if r, _, _ := getDIBits.Call(mem, bmp, 0, h, uintptr(unsafe.Pointer(&img.Pix[0])), uintptr(unsafe.Pointer(&hdr)), 0); r == 0 {
		return nil, errors.New("no se pudo leer la pantalla")
	}
	// GDI gives BGRA: swap to RGBA, opaque.
	p := img.Pix
	for i := 0; i < len(p); i += 4 {
		p[i], p[i+2], p[i+3] = p[i+2], p[i], 0xff
	}
	return img, nil
}
