package nucleo

import (
	"sort"
	"strings"
)

func ordenar(l []string) { sort.Strings(l) }

// NickTag is the board tag a nickname is found by (Rendezvous.nickTag).
func NickTag(nick string) string {
	return "zd-n-" + Hash(strings.ToLower(strings.TrimSpace(nick)))[:16]
}

// KeyTag is the board tag a member's beacon carries so contacts follow it.
func KeyTag(key string) string {
	if len(key) >= 16 {
		key = key[:16]
	}
	return "zd-k-" + key
}

// RoomTag is the board tag of a room's beacons.
func RoomTag(room string) string { return "zd-r-" + room }

// VecinoNet is the mesh neighbor id of an Internet link (NetNode.neighborId).
func VecinoNet(key string) string { return "net:" + key[:16] }

// The nickname's length limits, in characters (Nick.min, Nick.max).
const ApodoMin, ApodoMax = 3, 15

// ProblemaApodo says why a nickname cannot be used ("short", "long" or "bad"), or "" when it can (Nick.problem).
// The app counts grapheme clusters; this counts runes, the same except for emoji made of several runes.
func ProblemaApodo(apodo string) string {
	a := strings.TrimSpace(apodo)
	for _, r := range a {
		if r == '|' || r < 0x20 || r == 0x7f {
			return "bad"
		}
	}
	switch l := len([]rune(a)); {
	case l < ApodoMin:
		return "short"
	case l > ApodoMax:
		return "long"
	}
	return ""
}
