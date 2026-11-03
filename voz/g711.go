// Package voz is the room's voice in pure Go: winmm for the microphone and speaker, Opus as the codec (go-opus, the
// codec of the app and of every browser), a noise gate on the microphone, and pion for the connections, with the
// same "v-…" signals as the app's voice room. G.711 µ-law stays here for a peer that offers nothing else.
package voz

// Mu-law encoding and decoding (ITU-T G.711).

const (
	sesgo = 0x84
	tope  = 32635
)

func aMu(s int16) byte {
	x := int(s)
	signo := 0
	if x < 0 {
		x, signo = -x, 0x80
	}
	if x > tope {
		x = tope
	}
	x += sesgo
	exp := 7
	for m := 0x4000; x&m == 0 && exp > 0; m >>= 1 {
		exp--
	}
	man := (x >> (exp + 3)) & 0x0F
	return ^byte(signo | exp<<4 | man)
}

func deMu(u byte) int16 {
	u = ^u
	signo, exp, man := u&0x80, int(u>>4)&0x07, int(u&0x0F)
	x := ((man << 3) + sesgo) << exp
	x -= sesgo
	if signo != 0 {
		return int16(-x)
	}
	return int16(x)
}

func CodificarMu(pcm []int16) []byte {
	out := make([]byte, len(pcm))
	for i, s := range pcm {
		out[i] = aMu(s)
	}
	return out
}

func DecodificarMu(b []byte, out []int16) {
	for i := range b {
		if i < len(out) {
			out[i] = deMu(b[i])
		}
	}
}
