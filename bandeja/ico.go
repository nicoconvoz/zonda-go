// Package bandeja puts Zonda next to the clock, as the app does on Windows (platform/desktop_tray.dart): closing the
// window only hides it, so the device stays linked and messages keep arriving; a click on the icon brings it back,
// and its menu quits for real. Everything answers a click: nothing runs on a timer.
package bandeja

import (
	_ "embed"
	"encoding/binary"
	"errors"
)

// The app's own icons: the small one drawn for the tray, and the window's.
var (
	//go:embed zonda_tray.ico
	icoBandeja []byte
	//go:embed zonda.ico
	icoVentana []byte
)

// elegirImagen finds, in an .ico file, the image whose side is nearest lado (a tie goes to the larger, which
// shrinks better) and returns its bytes and side.
func elegirImagen(ico []byte, lado int) ([]byte, int, error) {
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:]) != 1 {
		return nil, 0, errors.New("no es un .ico")
	}
	n := int(binary.LittleEndian.Uint16(ico[4:]))
	mejor, mejorLado, mejorDist := -1, 0, 0
	for i := 0; i < n; i++ {
		e := 6 + 16*i
		if e+16 > len(ico) {
			break
		}
		l := int(ico[e])
		if l == 0 {
			l = 256
		}
		d := l - lado
		if d < 0 {
			d = -d
		}
		if mejor < 0 || d < mejorDist || (d == mejorDist && l > mejorLado) {
			mejor, mejorLado, mejorDist = e, l, d
		}
	}
	if mejor < 0 {
		return nil, 0, errors.New("el .ico no tiene imágenes")
	}
	tam := int(binary.LittleEndian.Uint32(ico[mejor+8:]))
	off := int(binary.LittleEndian.Uint32(ico[mejor+12:]))
	if off < 0 || tam <= 0 || off+tam > len(ico) {
		return nil, 0, errors.New("el .ico está cortado")
	}
	return ico[off : off+tam], mejorLado, nil
}
