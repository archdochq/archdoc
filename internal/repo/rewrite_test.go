package repo_test

import (
	"strings"
	"testing"

	"github.com/archdochq/archdoc/internal/repo"
)

func TestSetFieldReplacesTheWholeValueWhenItSpansLines(t *testing.T) {
	// A block sequence is valid YAML and parses correctly, so a rewrite that
	// touches only the key's line leaves the remaining items orphaned.
	source := []byte(`---
title: Glossary
includes:
  - RFC-0001
  - ADR-0002
---

# Glossary
`)

	out, err := repo.SetField(source, "includes", 3, "[RFC-0001, ADR-0002, RFC-0004]")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}
	if strings.Contains(string(out), "- RFC-0001") {
		t.Errorf("the old block sequence was left behind:\n%s", out)
	}

	// It must still parse, and read back as the list we wrote.
	entries, ok := repo.GlossaryIn(out)
	_ = entries
	if !ok {
		t.Fatalf("the rewritten front matter no longer parses:\n%s", out)
	}
	if !strings.Contains(string(out), "includes: [RFC-0001, ADR-0002, RFC-0004]") {
		t.Errorf("the new value was not written:\n%s", out)
	}
}

func TestSetFieldKeepsACommentBetweenKeys(t *testing.T) {
	source := []byte(`---
title: Glossary
includes: [RFC-0001]
# a note about what follows
extra: value
---

# Glossary
`)

	out, err := repo.SetField(source, "includes", 3, "[RFC-0001, ADR-0002]")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}
	if !strings.Contains(string(out), "# a note about what follows\nextra: value") {
		t.Errorf("a comment between keys was swallowed:\n%s", out)
	}
}

func TestSetFieldRefusesALineOutsideTheFrontMatter(t *testing.T) {
	// The body may contain something that looks exactly like a key.
	source := []byte(`---
title: Example
includes: []
---

# Example

` + "```yaml" + `
status: draft
` + "```" + `
`)

	if _, err := repo.SetField(source, "status", 9, "accepted"); err == nil {
		t.Error("SetField rewrote a line in the body, outside the front matter block")
	}
}

func TestSetFieldKeepsACommentAfterAnApostrophe(t *testing.T) {
	// An apostrophe mid-value does not open a quoted scalar; only a quote at
	// the start of the value does.
	source := []byte("---\ntitle: Ollie's plan   # keep me\nincludes: []\n---\n\n# X\n")

	out, err := repo.SetField(source, "title", 2, "Another plan")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}
	if !strings.Contains(string(out), "title: Another plan   # keep me") {
		t.Errorf("the comment was lost:\n%s", out)
	}
}

func TestSetFieldHandlesEverySpellingOfAMultiLineValue(t *testing.T) {
	// A comment among the items is not here: that shape is refused rather than
	// rewritten, and TestSetFieldRefusesRatherThanDeletingCommentsInsideTheValue
	// covers it. It used to be in this list, and passed, because the assertions
	// below ask only that the result parses and that the old value is gone,
	// which a rewrite that deleted the comment satisfies.
	for _, tc := range []struct{ name, front string }{
		{"block sequence at the key's own indentation", "includes:\n- RFC-0001\n- ADR-0001\n"},
		{"block sequence indented", "includes:\n  - RFC-0001\n  - ADR-0001\n"},
		{"block sequence with a blank line between items", "includes:\n- RFC-0001\n\n- ADR-0001\n"},
		{"flow sequence spread over lines", "includes: [\n  RFC-0001,\n  ADR-0001,\n]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := []byte("---\ntitle: Glossary\n" + tc.front + "---\n\n# Glossary\n")

			out, err := repo.SetField(source, "includes", 3, "[RFC-0001, ADR-0001, RFC-0004]")
			if err != nil {
				t.Fatalf("SetField: %v", err)
			}
			if _, ok := repo.GlossaryIn(out); !ok {
				t.Fatalf("the rewritten front matter no longer parses:\n%s", out)
			}
			if strings.Contains(string(out), "- RFC-0001") || strings.Contains(string(out), "  RFC-0001,") {
				t.Errorf("the old value was left behind:\n%s", out)
			}
		})
	}
}

func TestSetFieldKeepsACommentIndentedUnderTheValue(t *testing.T) {
	source := []byte("---\nid: RFC-0001\nstatus: draft\n  # still being written\ncreated: 2026-01-01\n---\n\n# X\n")

	out, err := repo.SetField(source, "status", 3, "proposed")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}
	if !strings.Contains(string(out), "# still being written") {
		t.Errorf("an indented comment was swallowed:\n%s", out)
	}
}

func TestSetFieldNeverReturnsFrontMatterThatWillNotParse(t *testing.T) {
	// The invariant behind every case above: whatever shape the old value had,
	// SetField either produces a document that parses or it returns an error.
	// It must never hand back corruption and report success.
	for _, front := range []string{
		"includes:\n- RFC-0001\n",
		"includes:\n  - RFC-0001\n",
		"includes: [\n  RFC-0001,\n]\n",
		"includes: >-\n  RFC-0001\n",
	} {
		source := []byte("---\ntitle: T\n" + front + "---\n\n# T\n")

		out, err := repo.SetField(source, "includes", 3, "[RFC-0001]")
		if err != nil {
			continue // refusing is an acceptable outcome
		}
		if _, ok := repo.GlossaryIn(out); !ok {
			t.Errorf("returned unparseable front matter for %q instead of an error:\n%s", front, out)
		}
	}
}

func TestSetFieldWorksOnAFileWithAByteOrderMark(t *testing.T) {
	// The BOM is trimmed for parsing but kept in Source, because a writer must
	// not alter bytes it was not asked to change. The writing side has to know
	// that too, or every transition on a Windows-edited document fails on a
	// block the user can plainly see.
	const bom = "\ufeff"
	source := []byte(bom + "---\nid: RFC-0001\nstatus: draft\ncreated: 2026-01-01\n---\n\n# RFC-0001: X\n")

	out, err := repo.SetField(source, "status", 3, "proposed")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}
	if !strings.Contains(string(out), "status: proposed") {
		t.Errorf("the status was not rewritten:\n%q", out)
	}
	if !strings.HasPrefix(string(out), bom) {
		t.Errorf("the byte order mark was dropped:\n%q", out)
	}
}

func TestSetFieldRefusesRatherThanDeletingCommentsInsideTheValue(t *testing.T) {
	// The whole extent from the key's line to the last sequence item was
	// replaced, so every comment among the items went with it. One after the
	// last item survived, which is what made the loss easy to miss. Refusing is
	// the same shape as the existing guard against unparseable output: the edit
	// is rejected rather than made destructively.
	source := []byte(`---
title: Page
includes:
  # the storage decision
  - ADR-0001
  # and the naming one
  - RFC-0002
---

# Page
`)

	_, err := repo.SetField(source, "includes", 3, "[ADR-0001, ADR-0002]")
	if err == nil {
		t.Fatal("SetField reported success over comments it was about to delete")
	}
	if !strings.Contains(err.Error(), "comment") {
		t.Errorf("error = %v, want it to say why", err)
	}
}

func TestSetFieldStillRewritesAValueWithNoCommentsInIt(t *testing.T) {
	source := []byte(`---
title: Page
includes:
  - ADR-0001
  - RFC-0002
---

# Page
`)

	out, err := repo.SetField(source, "includes", 3, "[ADR-0001, ADR-0002]")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "includes: [ADR-0001, ADR-0002]") {
		t.Errorf("the value was not rewritten:\n%s", out)
	}
}

func TestSetFieldRefusesAKeyThatIsNotThere(t *testing.T) {
	// LineOf returns 0 for an absent key and lifecycle.Apply passes it straight
	// through, so a document whose "decided:" line was deleted reaches here
	// with line 0. Without the guard lines[line-1] indexes -1 and the process
	// panics in front of the user.
	source := []byte("---\nid: RFC-0001\ntitle: A\nstatus: proposed\n---\n\n# RFC-0001: A\n")

	if _, err := repo.SetField(source, "decided", 0, "2026-02-01"); err == nil {
		t.Error("SetField accepted line 0 for a key that is not in the front matter")
	}
}
