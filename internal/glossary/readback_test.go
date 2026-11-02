package glossary

import "testing"

// TestReadBackRefusesAFileThatDoesNotParseAsTheTerm covers what the encoder is
// trusted to get right: a title reading as another name, or front matter that
// does not parse, is refused even where the H1 reads as the term.
func TestReadBackRefusesAFileThatDoesNotParseAsTheTerm(t *testing.T) {
	for name, frontMatter := range map[string]string{
		"another title":   "title: Gadget\nformerly: []\nnamed_by:\n",
		"a parse problem": "title: Widget\nformerly: []\nformerly: []\nnamed_by:\n",
	} {
		t.Run(name, func(t *testing.T) {
			source := []byte("---\n" + frontMatter + "---\n\n# Widget\n\nA definition.\n")
			if _, err := readBack("term/widget.md", source, "Widget"); err == nil {
				t.Error("readBack accepted it")
			}
		})
	}
	good := []byte("---\ntitle: Widget\nformerly: []\nnamed_by:\n---\n\n# Widget\n\nA definition.\n")
	if _, err := readBack("term/widget.md", good, "Widget"); err != nil {
		t.Errorf("readBack refused a well-formed file: %v", err)
	}
}
