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
