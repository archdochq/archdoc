package repo

import (
	"slices"
	"testing"
)

// TestAnchorsInFindsExplicitAnchors covers the target GLOSSARY.md writes for a
// term's former names. A rename moves the heading's anchor, so without an
// explicit one every link written before the rename breaks, and L17 would
// report a frozen document that cannot be edited to fix it.
func TestAnchorsInFindsExplicitAnchors(t *testing.T) {
	source := []byte("---\ntitle: Glossary\n---\n\n# Glossary\n\n<a id=\"editable\"></a>\n## open\n\nWords.\n")
	got := AnchorsIn(source)
	for _, want := range []string{"glossary", "open", "editable"} {
		if !slices.Contains(got, want) {
			t.Errorf("AnchorsIn did not find %q: %v", want, got)
		}
	}
}
