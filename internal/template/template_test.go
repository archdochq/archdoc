package template_test

import (
	"strings"
	"testing"

	"github.com/ollieread/archdoc/internal/repo"
	"github.com/ollieread/archdoc/internal/repotest"
	"github.com/ollieread/archdoc/internal/template"
)

// renderInto writes a rendered document into a throwaway repository and parses
// it back, so the test asserts on what archdoc would actually read.
func renderInto(t *testing.T, name, path string, data template.Data) *repo.Document {
	t.Helper()
	out, err := template.Render(name, data)
	if err != nil {
		t.Fatalf("Render(%s): %v", name, err)
	}
	t.Logf("%s rendered as:\n%s", name, out)

	d := repotest.Document(t, repotest.New(t, map[string]string{path: string(out)}), path)
	for _, p := range d.Problems {
		t.Errorf("rendered document has a parse problem at line %d: %s", p.Line, p.Message)
	}
	return d
}

func TestRFCTemplateRendersADraft(t *testing.T) {
	d := renderInto(t, "rfc.md", "rfc/0001-a-draft.md", template.Data{
		ID: "RFC-0001", Title: "A draft", Date: "2026-09-11", Status: "draft",
	})

	if d.FrontMatter.Status != repo.StatusDraft {
		t.Errorf("status = %q, want draft", d.FrontMatter.Status)
	}
	if !d.FrontMatter.Decided.IsZero() {
		t.Errorf("decided = %v, want unset", d.FrontMatter.Decided)
	}
	if d.FrontMatter.Has("backfilled") {
		t.Error("an ordinary draft carries a backfilled key")
	}
	if _, ok := repotest.Section(d, "Sources"); ok {
		t.Error("an ordinary draft has a Sources section")
	}
	for _, want := range repo.RequiredSections(repo.TypeRFC) {
		if _, ok := repotest.Section(d, want); !ok {
			t.Errorf("missing section %q", want)
		}
	}
}

func TestRFCTemplateRendersABackfilledDocument(t *testing.T) {
	d := renderInto(t, "rfc.md", "rfc/0002-backfilled.md", template.Data{
		ID: "RFC-0002", Title: "Backfilled: a title with a colon", Date: "2023-04-02",
		Status: "accepted", Decided: "2023-05-10", Backfilled: "2026-09-11",
	})

	if d.FrontMatter.Status != repo.StatusAccepted {
		t.Errorf("status = %q, want accepted", d.FrontMatter.Status)
	}
	if got := d.FrontMatter.Decided.Format("2006-01-02"); got != "2023-05-10" {
		t.Errorf("decided = %q, want 2023-05-10", got)
	}
	if got := d.FrontMatter.Backfilled.Format("2006-01-02"); got != "2026-09-11" {
		t.Errorf("backfilled = %q, want 2026-09-11", got)
	}
	if d.FrontMatter.Title != "Backfilled: a title with a colon" {
		t.Errorf("title = %q, want the colon preserved", d.FrontMatter.Title)
	}
	if _, ok := repotest.Section(d, "Sources"); !ok {
		t.Error("a backfilled document has no Sources section")
	}
	alternatives, ok := repotest.Section(d, "Alternatives considered")
	if !ok {
		t.Fatal("no Alternatives considered section")
	}
	if !strings.Contains(alternatives.Text, "Not recorded.") {
		t.Errorf("Alternatives considered = %q, want it pre-filled with Not recorded.", alternatives.Text)
	}
	if alternatives.Empty() {
		t.Error("Alternatives considered is empty, so L14 would report a document the tool just wrote")
	}
}

func TestADRTemplateRendersABackfilledDocument(t *testing.T) {
	d := renderInto(t, "adr.md", "adr/0001-backfilled.md", template.Data{
		ID: "ADR-0001", Title: "A decision", Date: "2023-01-01",
		Status: "accepted", Decided: "2023-02-01", Backfilled: "2026-09-11",
	})

	if _, ok := repotest.Section(d, "Sources"); !ok {
		t.Error("a backfilled ADR has no Sources section")
	}
	for _, want := range repo.RequiredSections(repo.TypeADR) {
		if _, ok := repotest.Section(d, want); !ok {
			t.Errorf("missing section %q", want)
		}
	}
}
