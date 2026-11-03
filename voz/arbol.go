package voz

import (
	"encoding/binary"
	"encoding/hex"
	"hash/fnv"
	"sort"

	"github.com/nicoconvoz/zonda-go/nucleo"
)

// The Captain's relay tree for the voice between Go devices: whoever speaks sends to Ramas members; each of those
// forwards to its share of the rest. Twelve people hear with one extra hop, and nobody uploads more than Ramas copies
// of anyone's voice. Every device computes the same tree from the channel's member list, with no coordination: the
// order for a speaker is the members sorted by a hash of (speaker, member), so each speaker gets a different first
// tier and the forwarding load spreads.

const Ramas = 3

// Arbol: for speaker src among miembros (src included or not), the first tier and who each of them forwards to.
func Arbol(src string, miembros []string) (primeros []string, hijos map[string][]string) {
	var orden []string
	for _, m := range miembros {
		if m != src {
			orden = append(orden, m)
		}
	}
	sort.Slice(orden, func(i, j int) bool {
		return nucleo.Hash(src+"|"+orden[i]) < nucleo.Hash(src+"|"+orden[j])
	})
	hijos = map[string][]string{}
	n := min(Ramas, len(orden))
	primeros = orden[:n]
	for j := n; j < len(orden); j++ {
		p := primeros[(j-n)%n]
		hijos[p] = append(hijos[p], orden[j])
	}
	return primeros, hijos
}

// A voice frame between Go devices: "Z1" | channel hash (4) | speaker key (32) | sequence (4) | µ-law payload.
const cabecera = 2 + 4 + 32 + 4

func huellaCanal(c string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(c))
	return h.Sum32()
}

func armarCuadro(canal, src string, seq uint32, mu []byte) []byte {
	k, _ := hex.DecodeString(src)
	b := make([]byte, cabecera+len(mu))
	b[0], b[1] = 'Z', '1'
	binary.BigEndian.PutUint32(b[2:], huellaCanal(canal))
	copy(b[6:38], k)
	binary.BigEndian.PutUint32(b[38:], seq)
	copy(b[cabecera:], mu)
	return b
}

func leerCuadro(b []byte) (canal uint32, src string, seq uint32, mu []byte, ok bool) {
	if len(b) <= cabecera || b[0] != 'Z' || b[1] != '1' {
		return 0, "", 0, nil, false
	}
	return binary.BigEndian.Uint32(b[2:]), hex.EncodeToString(b[6:38]), binary.BigEndian.Uint32(b[38:]), b[cabecera:], true
}
