package nucleo

import "testing"

// The same answers as Nick.problem in the app (social/nick.dart).
func TestProblemaApodo(t *testing.T) {
	casos := map[string]string{
		"Capitan86":        "",
		"  Ana  ":          "",
		"Al":               "short",
		"   Al   ":         "short",
		"Ñandú":            "",
		"abcdefghijklmnop": "long",
		"abcdefghijklmno":  "",
		"Ana|Bel":          "bad",
		"Ana\nBel":         "bad",
		"Ana\x7fBel":       "bad",
	}
	for apodo, quiere := range casos {
		if got := ProblemaApodo(apodo); got != quiere {
			t.Errorf("ProblemaApodo(%q) = %q, quiere %q", apodo, got, quiere)
		}
	}
}
