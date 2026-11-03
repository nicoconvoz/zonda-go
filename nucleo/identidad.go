// Package nucleo is Zonda's core in Go: the same signed lines, sealed envelopes and mesh messages as the Dart core
// (app/packages/zonda_core), byte for byte, so a Go device and a phone running the Flutter app talk to each other.
package nucleo

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/nacl/box"
)

// Identidad is a member's Ed25519 key pair, compatible with tweetnacl (Identity in crypto/identity.dart).
type Identidad struct {
	priv ed25519.PrivateKey
}

// NuevaIdentidad draws a fresh key pair.
func NuevaIdentidad() *Identidad {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return &Identidad{priv}
}

// DesdeSemilla: the same 32-byte seed always gives the same key.
func DesdeSemilla(seed []byte) *Identidad { return &Identidad{ed25519.NewKeyFromSeed(seed)} }

// Restaurar rebuilds the identity from its exported 64-byte secret (hex).
func Restaurar(secretHex string) (*Identidad, error) {
	b, err := hex.DecodeString(secretHex)
	if err != nil || len(b) != ed25519.PrivateKeySize {
		return nil, errors.New("secreto inválido")
	}
	return DesdeSemilla(b[:32]), nil
}

func (id *Identidad) Publica() string {
	return hex.EncodeToString(id.priv.Public().(ed25519.PublicKey))
}
func (id *Identidad) Exportar() string { return hex.EncodeToString(id.priv) }

// Firmar: detached signature of the UTF-8 message, hex.
func (id *Identidad) Firmar(msg string) string {
	return hex.EncodeToString(ed25519.Sign(id.priv, []byte(msg)))
}

// Verificar checks a hex signature of msg by the hex public key.
func Verificar(pub, msg, sig string) bool {
	p, err := hex.DecodeString(pub)
	if err != nil || len(p) != ed25519.PublicKeySize {
		return false
	}
	s, err := hex.DecodeString(sig)
	if err != nil || len(s) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(p, []byte(msg), s)
}

// Hash is SHA-512 truncated to 256 bits, hex (Identity.hash).
func Hash(msg string) string { return HashBytes([]byte(msg)) }

// HashBytes is the fingerprint of photos and files (SocialNode.photoHash).
func HashBytes(b []byte) string {
	h := sha512.Sum512(b)
	return hex.EncodeToString(h[:])[:64]
}

func (id *Identidad) xPriv() *[32]byte {
	h := sha512.Sum512(id.priv[:32])
	var k [32]byte
	copy(k[:], h[:32])
	k[0] &= 248
	k[31] &= 127
	k[31] |= 64
	return &k
}

func xPub(pubHex string) (*[32]byte, error) {
	b, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, err
	}
	p, err := new(edwards25519.Point).SetBytes(b)
	if err != nil {
		return nil, err
	}
	var k [32]byte
	copy(k[:], p.BytesMontgomery())
	return &k, nil
}

func (id *Identidad) compartida(otro string) (*[32]byte, error) {
	pub, err := xPub(otro)
	if err != nil {
		return nil, err
	}
	var k [32]byte
	box.Precompute(&k, pub, id.xPriv())
	return &k, nil
}

// Sellar encrypts plain so only the owner of para can read it: NaCl box over the X25519 forms of both keys,
// nonce + ciphertext.
func (id *Identidad) Sellar(para string, plain []byte) ([]byte, error) {
	k, err := id.compartida(para)
	if err != nil {
		return nil, err
	}
	return sellarCon(k, plain), nil
}

func sellarCon(k *[32]byte, plain []byte) []byte {
	var n [24]byte
	rand.Read(n[:])
	return box.SealAfterPrecomputation(n[:], plain, &n, k)
}

func abrirCon(k *[32]byte, sealed []byte) ([]byte, bool) {
	if len(sealed) < 24+box.Overhead {
		return nil, false
	}
	var n [24]byte
	copy(n[:], sealed[:24])
	return box.OpenAfterPrecomputation(nil, sealed[24:], &n, k)
}

// Abrir opens what de sealed for this identity; false when it was not for us or was altered.
func (id *Identidad) Abrir(de string, sealed []byte) ([]byte, bool) {
	k, err := id.compartida(de)
	if err != nil {
		return nil, false
	}
	return abrirCon(k, sealed)
}

// Pieza is the size of each sealed piece of a big payload.
const Pieza = 512 * 1024

var magia = []byte("IBK1")

// SellarGrande seals anything: up to Pieza bytes a single seal, bigger ones in pieces framed as
// `IBK1 | count | (length | sealed piece)…` (big-endian).
func (id *Identidad) SellarGrande(para string, plain []byte) ([]byte, error) {
	if len(plain) <= Pieza {
		return id.Sellar(para, plain)
	}
	k, err := id.compartida(para)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.Write(magia)
	count := (len(plain) + Pieza - 1) / Pieza
	binary.Write(&out, binary.BigEndian, uint32(count))
	for i := 0; i < count; i++ {
		end := min((i+1)*Pieza, len(plain))
		p := sellarCon(k, plain[i*Pieza:end])
		binary.Write(&out, binary.BigEndian, uint32(len(p)))
		out.Write(p)
	}
	return out.Bytes(), nil
}

// AbrirGrande opens what SellarGrande (or Sellar) made.
func (id *Identidad) AbrirGrande(de string, sealed []byte) ([]byte, bool) {
	if len(sealed) < 8 || !bytes.HasPrefix(sealed, magia) {
		return id.Abrir(de, sealed)
	}
	k, err := id.compartida(de)
	if err != nil {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(sealed[4:]))
	at := 8
	var out bytes.Buffer
	for i := 0; i < count; i++ {
		if at+4 > len(sealed) {
			return nil, false
		}
		n := int(binary.BigEndian.Uint32(sealed[at:]))
		at += 4
		if at+n > len(sealed) {
			return nil, false
		}
		p, ok := abrirCon(k, sealed[at:at+n])
		if !ok {
			return nil, false
		}
		out.Write(p)
		at += n
	}
	return out.Bytes(), at == len(sealed)
}
