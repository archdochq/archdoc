package markdown

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzOutlineMatchesTheReference compares the outline of generated input
// with the reference's parse of it, where the version REFERENCE names is on
// the path, and asserts the position rules of every input either way. A
// difference it finds becomes a case, and a known difference where the
// parser is not corrected.
func FuzzOutlineMatchesTheReference(f *testing.F) {
	known := known(f)
	skip := map[string]bool{}
	for _, c := range cases(f) {
		f.Add(c.body)
		if _, ok := known[c.name]; ok {
			skip[string(c.body)] = true
		}
	}
	path, extensions, refErr := referenceOnPath(f)
	f.Fuzz(func(t *testing.T, body []byte) {
		src := withLoneCarriageReturnsAsLineFeeds(body)
		for _, p := range placementProblems(src, Parse(body)) {
			t.Errorf("%q: %s", body, p)
		}
		// The reference writes control characters other than these into its
		// XML unescaped.
		if refErr != nil || skip[string(body)] || !utf8.Valid(body) || strings.IndexFunc(string(body), isControl) >= 0 {
			return
		}
		parse, err := recordParse(path, extensions, body)
		if err != nil {
			t.Fatalf("%q: the reference fails: %v", body, err)
		}
		if at, diff := compareWithReference(body, parse); diff != "" {
			t.Errorf("%q: the outline first differs at %s: %s", body, at, diff)
		}
	})
}

func isControl(r rune) bool {
	return r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0x7f
}
