package bandeja

import (
	"encoding/binary"
	"testing"
)

// The .ico picker takes the image whose side is nearest the one asked for (ties to the larger, which shrinks
// better), and says where its bytes are; a width byte of 0 means 256.
func TestElegirImagenDelIco(t *testing.T) {
	lados := []int{16, 32, 48, 256}
	ico := []byte{0, 0, 1, 0, byte(len(lados)), 0}
	off := 6 + 16*len(lados)
	for i, l := range lados {
		e := make([]byte, 16)
		e[0], e[1] = byte(l), byte(l)
		binary.LittleEndian.PutUint32(e[8:], uint32(10+i))
		binary.LittleEndian.PutUint32(e[12:], uint32(off))
		off += 10 + i
		ico = append(ico, e...)
	}
	ico = append(ico, make([]byte, off-len(ico))...)
	for pide, quiere := range map[int]int{16: 16, 20: 16, 24: 32, 32: 32, 40: 48, 300: 256} {
		img, lado, err := elegirImagen(ico, pide)
		if err != nil {
			t.Fatal(err)
		}
		if lado != quiere {
			t.Fatalf("pide %d: eligió %d, quiere %d", pide, lado, quiere)
		}
		if len(img) < 10 {
			t.Fatalf("pide %d: %d bytes", pide, len(img))
		}
	}
	if _, _, err := elegirImagen([]byte{1, 2, 3}, 16); err == nil {
		t.Fatal("un archivo roto no da error")
	}
}
