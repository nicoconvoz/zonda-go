package nucleo

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// ClavesNostr is a device's Nostr key, derived from its Zonda key (NostrKeys.of): stable, never shared.
type ClavesNostr struct {
	Priv, Pub string
	k         *btcec.PrivateKey
}

func ClavesNostrDe(id *Identidad) *ClavesNostr {
	h := sha256.Sum256([]byte("zonda-nostr|" + id.Exportar()))
	k, _ := btcec.PrivKeyFromBytes(h[:])
	return &ClavesNostr{Priv: hex.EncodeToString(h[:]), Pub: hex.EncodeToString(schnorr.SerializePubKey(k.PubKey())), k: k}
}

// EventoNostr is a signed NIP-01 event: id = sha256 of [0, pubkey, created_at, kind, tags, content].
func EventoNostr(c *ClavesNostr, kind int, tags [][]string, content string, createdAt int64) Msg {
	var b strings.Builder
	canonico(&b, []any{0, c.Pub, createdAt, kind, tags, content})
	h := sha256.Sum256([]byte(b.String()))
	var aux [32]byte
	rand.Read(aux[:])
	sig, err := schnorr.Sign(c.k, h[:], schnorr.CustomNonce(aux))
	if err != nil {
		panic(err)
	}
	ts := make([]any, len(tags))
	for i, t := range tags {
		row := make([]any, len(t))
		for j, s := range t {
			row[j] = s
		}
		ts[i] = row
	}
	return Msg{"id": hex.EncodeToString(h[:]), "pubkey": c.Pub, "created_at": createdAt, "kind": kind, "tags": ts, "content": content, "sig": hex.EncodeToString(sig.Serialize())}
}

// idEvento recomputes an event's id from its fields (tests and checks).
func idEvento(e Msg) string {
	var b strings.Builder
	tags := [][]string{}
	for _, t := range e["tags"].([]any) {
		tags = append(tags, Textos(t))
	}
	ca, _ := Entero(e, "created_at")
	k, _ := Entero(e, "kind")
	canonico(&b, []any{0, e["pubkey"], ca, k, tags, e["content"]})
	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}
