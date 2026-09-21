package export_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"archdoc.dev/internal/export"
	"archdoc.dev/internal/repotest"
)

func rfc(id, title, status, body string) string {
	decided := ""
	if status == "accepted" || status == "rejected" || status == "withdrawn" {
		decided = "2026-02-02"
	}
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: " + status +
		"\ncreated: 2026-01-01\ndecided: " + decided +
		"\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# " + id + ": " + title + "\n\n" + body
}

// TestTheDerivedGraphIsExported is the point of the command. Nothing inside a
// document records what happened to it later, so a consumer cannot compute
// these from one file and would have to re-implement the whole repository walk.
func TestTheDerivedGraphIsExported(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-old.md": rfc("RFC-0001", "Old", "accepted", "## Abstract\n\nWords.\n"),
		"rfc/0002-new.md": strings.Replace(
			rfc("RFC-0002", "New", "accepted", "## Abstract\n\nWords.\n"),
			"obsoletes: []", "obsoletes: [RFC-0001]", 1),
		"spec/database.md": "---\ntitle: Database\nincludes: [RFC-0002]\n---\n\n# Database\n\nWords.\n",
	})

	out := export.Build(r, export.Options{})
	byID := map[string]export.Document{}
	for _, d := range out.Documents {
		byID[d.ID] = d
	}

	old := byID["RFC-0001"]
	if got := old.ObsoletedBy; len(got) != 1 || got[0] != "RFC-0002" {
		t.Errorf("obsoleted_by = %v, want [RFC-0002]", got)
	}
	if !old.EffectivelyObsolete {
		t.Error("RFC-0001 is obsoleted by an accepted document but is not marked effectively obsolete")
	}
	newer := byID["RFC-0002"]
	if got := newer.IncludedIn; len(got) != 1 || got[0] != "database" {
		t.Errorf("included_in = %v, want [database]", got)
	}
	if !newer.Implemented {
		t.Error("RFC-0002 is included by a spec page but is not marked implemented")
	}
}

// TestContentsIsInDocumentOrderAndExclusive pins the two properties the object
// keyed by anchor could not carry. A Go map marshals its keys sorted, so a
// document arrived alphabetical with nothing recording the order it was
// written in, and a heading below level two arrived not at all.
func TestContentsIsInDocumentOrderAndExclusive(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-order.md": rfc("RFC-0001", "Order", "draft",
			"## Zulu\n\nOne.\n\n## Alpha\n\nTwo.\n\n### Deep\n\nThree.\n"),
	})
	d := export.Build(r, export.Options{}).Documents[0]

	if got, want := anchorsOf(d.Contents), []string{"rfc-0001-order", "zulu", "alpha", "deep"}; !slices.Equal(got, want) {
		t.Fatalf("contents = %v, want the headings in document order", got)
	}
	if got := d.Contents[2]; got.Level != 2 || strings.Contains(*got.Body, "Three") {
		t.Errorf("the level two entry carries its subsection's prose: %q", *got.Body)
	}
	if got := d.Contents[3]; got.Level != 3 || !strings.Contains(*got.Body, "Three") {
		t.Errorf("the level three entry is wrong: level %d, body %q", got.Level, *got.Body)
	}
}

// TestAnEmptyBodyIsNotAnAbsentOne is why Entry.Body is a pointer. A heading
// directly followed by another has a genuinely empty body, and omitempty on a
// string would make that indistinguishable from an export carrying no bodies.
func TestAnEmptyBodyIsNotAnAbsentOne(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-bare.md": rfc("RFC-0001", "Bare", "draft", "## Abstract\n## Motivation\n\nWords.\n"),
	})
	d := export.Build(r, export.Options{}).Documents[0]

	abstract, ok := entryAt(d.Contents, "abstract")
	if !ok {
		t.Fatalf("no entry anchored abstract: %v", anchorsOf(d.Contents))
	}
	if abstract.Body == nil {
		t.Fatal("an empty body was reported as absent, which is what --no-bodies means")
	}
	if *abstract.Body != "" {
		t.Errorf("body = %q, want empty", *abstract.Body)
	}
}

// TestARepeatedHeadingKeepsBothEntries pins the reason anchors are made unique
// by the walk that assigns them. Nothing forbids a repeated heading.
func TestARepeatedHeadingKeepsBothEntries(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-two.md": rfc("RFC-0001", "Two", "draft",
			"## Abstract\n\nFirst.\n\n## Abstract\n\nSecond.\n"),
	})
	d := export.Build(r, export.Options{}).Documents[0]

	first, ok := entryAt(d.Contents, "abstract")
	if !ok {
		t.Fatalf("no entry anchored abstract: %v", anchorsOf(d.Contents))
	}
	second, ok := entryAt(d.Contents, "abstract-1")
	if !ok {
		t.Fatalf("the repeated heading was dropped: %v", anchorsOf(d.Contents))
	}
	if !strings.Contains(*first.Body, "First") || !strings.Contains(*second.Body, "Second") {
		t.Errorf("the two entries carry the wrong bodies:\n%q\n%q", *first.Body, *second.Body)
	}
	if first.Text != "Abstract" || second.Text != "Abstract" {
		t.Error("the heading as written is not preserved alongside the anchor")
	}
}

// TestRequiredSectionsSurviveTheirAbsence covers the ordinary case of a draft
// part-way through being written. Lint checks the sections are there; nothing
// guarantees it, and a consumer needs to render a heading for one that is not.
func TestRequiredSectionsSurviveTheirAbsence(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-part.md": rfc("RFC-0001", "Part", "draft", "## Abstract\n\nOnly this one.\n"),
	})
	d := export.Build(r, export.Options{}).Documents[0]

	if len(d.RequiredSections) < 7 {
		t.Fatalf("expected the RFC section list, got %d", len(d.RequiredSections))
	}
	var proposal *export.RequiredSection
	for i, s := range d.RequiredSections {
		if s.Title == "Proposal" {
			proposal = &d.RequiredSections[i]
		}
	}
	if proposal == nil {
		t.Fatal("Proposal is not listed as a required section")
	}
	if proposal.Anchor != "proposal" {
		t.Errorf("a missing section has anchor %q, want the anchor it would have had", proposal.Anchor)
	}
	if _, present := entryAt(d.Contents, proposal.Anchor); present {
		t.Error("a section that does not exist was exported as present")
	}
}

// TestLinksResolveToDocuments is what spares a consumer from parsing Markdown
// to rewrite destinations into its own URL scheme.
func TestLinksResolveToDocuments(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-target.md": rfc("RFC-0001", "Target", "accepted", "## Abstract\n\nWords.\n"),
		"rfc/0002-source.md": rfc("RFC-0002", "Source", "draft",
			"## Abstract\n\nSee [RFC-0001](0001-target.md#abstract), the "+
				"[manual](https://example.invalid/docs) and [us](mailto:a@example.invalid).\n"),
	})
	var source export.Document
	for _, d := range export.Build(r, export.Options{}).Documents {
		if d.ID == "RFC-0002" {
			source = d
		}
	}
	byText := map[string]export.Link{}
	for _, l := range source.Links {
		byText[l.Text] = l
	}

	internal := byText["RFC-0001"]
	if internal.Path != "rfc/0001-target.md" {
		t.Errorf("path = %q, want it resolved against the repository root", internal.Path)
	}
	if internal.ResolvesTo != "RFC-0001" {
		t.Errorf("resolves_to = %q, want RFC-0001", internal.ResolvesTo)
	}
	if internal.Anchor != "abstract" {
		t.Errorf("anchor = %q, want abstract", internal.Anchor)
	}
	// A scheme means somewhere else. mailto: has no "//", which is why testing
	// for "://" resolved it as a filename.
	for _, text := range []string{"manual", "us"} {
		if got := byText[text].Path; got != "" {
			t.Errorf("%s was resolved to a path %q; it is not in this repository", text, got)
		}
	}
}

// TestALinkKnowsWhichFormWroteIt covers the defect the type field exists for.
// An image was emitted as an ordinary link, so a consumer rewriting
// destinations turned an img source into an anchor, and a list of a document's
// outgoing links included its diagrams.
func TestALinkKnowsWhichFormWroteIt(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-forms.md": rfc("RFC-0001", "Forms", "draft",
			"## Abstract\n\nAn ![diagram](d.png) and a [page](../spec/glossary.md) and [a][r].\n\n[r]: ../spec/glossary.md\n"),
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n",
	})
	d := export.Build(r, export.Options{}).Documents[0]

	byText := map[string]export.Link{}
	for _, l := range d.Links {
		byText[l.Text] = l
	}
	for text, want := range map[string]string{
		"diagram": "image",
		"page":    "inline",
		"r":       "definition",
	} {
		got, ok := byText[text]
		if !ok {
			t.Errorf("no link with text %q: %v", text, byText)
			continue
		}
		if got.Type != want {
			t.Errorf("%q has type %q, want %q", text, got.Type, want)
		}
	}
	// The reference use carries a label rather than a destination, and
	// rewriting the definition carries it, so it is deliberately not listed.
	if _, listed := byText["a"]; listed {
		t.Error("a reference use was extracted; only its definition should be")
	}
}

// TestNoBodiesKeepsEverythingButTheProse covers the listing-page case.
func TestNoBodiesKeepsEverythingButTheProse(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": rfc("RFC-0001", "A", "accepted", "## Abstract\n\nProse here.\n"),
	})
	d := export.Build(r, export.Options{NoBodies: true}).Documents[0]
	entry, ok := entryAt(d.Contents, "abstract")
	if !ok {
		t.Fatal("the entry was dropped along with its body")
	}
	if entry.Body != nil {
		t.Error("the body was included")
	}
	if entry.Text != "Abstract" {
		t.Error("the heading text was lost")
	}
	if d.Source != "" {
		t.Error("the file was included without --source")
	}
	if d.Title != "A" || d.Status != "accepted" {
		t.Error("metadata was dropped")
	}
}

// TestOutputIsDeterministic keeps the export cacheable, diffable and
// committable. A generated-at timestamp would break all three.
func TestOutputIsDeterministic(t *testing.T) {
	files := map[string]string{
		"rfc/0001-a.md": rfc("RFC-0001", "A", "accepted", "## Abstract\n\nWords.\n"),
		"rfc/0002-b.md": rfc("RFC-0002", "B", "draft", "## Abstract\n\nWords.\n"),
	}
	first, err := json.Marshal(export.Build(repotest.New(t, files), export.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := json.Marshal(export.Build(repotest.New(t, files), export.Options{}))
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(first) {
			t.Fatal("two exports of the same repository differ")
		}
	}
	if strings.Contains(string(first), "generated") {
		t.Error("the output records when it was generated, which defeats caching and diffing")
	}
}

// TestEveryFieldIsInThePublishedSchema is the guard that makes "a defined
// schema" mean something. Without it the schema is whatever the struct looked
// like when someone last remembered, which is exactly what a consumer pinning
// to it is trying to avoid.
//
// It compares the two sides independently rather than generating one from the
// other, the same way the release test resolves asset names from both
// .goreleaser.yaml and the workflow.
func TestEveryFieldIsInThePublishedSchema(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(export.JSONSchema, &schema); err != nil {
		t.Fatalf("the published schema is not valid JSON: %v", err)
	}
	defs, _ := schema["$defs"].(map[string]any)
	if defs == nil {
		t.Fatal("the schema declares no $defs")
	}

	for _, c := range []struct {
		def   string
		value any
	}{
		{"", export.Repository{}},
		{"document", export.Document{}},
		{"config", export.Config{}},
		{"entry", export.Entry{}},
		{"link", export.Link{}},
		{"term", export.Term{}},
	} {
		where := schema
		if c.def != "" {
			where, _ = defs[c.def].(map[string]any)
			if where == nil {
				t.Errorf("the schema has no $defs/%s", c.def)
				continue
			}
		}
		properties, _ := where["properties"].(map[string]any)
		if properties == nil {
			t.Errorf("%s declares no properties", c.def)
			continue
		}
		declared := map[string]bool{}
		for name := range properties {
			declared[name] = true
		}

		typ := reflect.TypeOf(c.value)
		for i := range typ.NumField() {
			name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			if !declared[name] {
				t.Errorf("%s.%s is exported as %q but the schema does not declare it",
					typ.Name(), typ.Field(i).Name, name)
			}
			delete(declared, name)
		}
		for name := range declared {
			t.Errorf("the schema declares %s property %q, which nothing exports", c.def, name)
		}
	}
}

// entryAt finds a contents entry by anchor.
func entryAt(entries []export.Entry, anchor string) (export.Entry, bool) {
	for _, e := range entries {
		if e.Anchor == anchor {
			return e, true
		}
	}
	return export.Entry{}, false
}

// anchorsOf lists a contents array's anchors, in order, for a failure message.
func anchorsOf(entries []export.Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Anchor)
	}
	return out
}

// TestArrayFieldsAreNeverNull keeps a consumer from having to handle both an
// array and null for the same field. A spec page has no required sections and a
// document may have no links, and the published schema declares both as arrays.
//
// The schema test above compares field names, not shapes, so it cannot catch
// this: the field is declared and populated, just with the wrong JSON type.
func TestArrayFieldsAreNeverNull(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"spec/database.md": "---\ntitle: Database\nincludes: []\n---\n\n# Database\n\nNo links here.\n",
		"rfc/0001-a.md":    rfc("RFC-0001", "A", "draft", "## Abstract\n\nWords.\n"),
	})
	encoded, err := json.Marshal(export.Build(r, export.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Documents []map[string]json.RawMessage `json:"documents"`
	}
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Documents) != 2 {
		t.Fatalf("expected two documents, got %d", len(out.Documents))
	}
	for _, doc := range out.Documents {
		for _, field := range []string{
			"depends", "updates", "obsoletes", "includes",
			"updated_by", "obsoleted_by", "depended_on_by", "included_in",
			"required_sections", "links",
		} {
			raw, present := doc[field]
			if !present {
				t.Errorf("%s is missing; the schema declares it required", field)
				continue
			}
			if string(raw) == "null" {
				t.Errorf("%s serialised as null; the schema declares it an array", field)
			}
		}
		if string(doc["sections"]) == "null" {
			t.Error("sections serialised as null; the schema declares it an object")
		}
	}
}

// updateSchema republishes the schema outside internal/. Run
// `go test ./internal/export -update-schema` after changing it.
var updateSchema = flag.Bool("update-schema", false, "rewrite schema/export/v1.json from the embedded copy")

const publishedSchema = "../../schema/export/v2.json"

// TestThePublishedSchemaMatchesTheEmbeddedOne gives the contract an address.
//
// The schema is embedded so `archdoc export --schema` can print it without a
// repository, and go:embed cannot reach outside the package, so the copy a
// consumer fetches has to be generated. Its $id names that copy's URL, and a
// $id pointing at something that does not exist is worse than an obviously
// abstract one: a tool that resolves it gets an error page.
//
// internal/ also means "you may not import this" in Go. That is true of the
// package and irrelevant to the file, but it reads as private for something
// published on purpose.
func TestThePublishedSchemaMatchesTheEmbeddedOne(t *testing.T) {
	if *updateSchema {
		if err := os.MkdirAll(filepath.Dir(publishedSchema), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(publishedSchema, export.JSONSchema, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", publishedSchema)
		return
	}
	got, err := os.ReadFile(publishedSchema)
	if err != nil {
		t.Fatalf("%s is missing: %v (run go test ./internal/export -update-schema)", publishedSchema, err)
	}
	if string(got) != string(export.JSONSchema) {
		t.Errorf("%s differs from the embedded schema; run go test ./internal/export -update-schema", publishedSchema)
	}

	// The $id has to name the published copy, or it names nothing.
	var schema struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(export.JSONSchema, &schema); err != nil {
		t.Fatal(err)
	}
	// Derived from publishedSchema rather than written out a second time, so
	// the two cannot come to name different versions of the shape.
	if want := "/" + strings.TrimPrefix(publishedSchema, "../../"); !strings.HasSuffix(schema.ID, want) {
		t.Errorf("$id is %q, which does not name the published copy at %s", schema.ID, want)
	}
	if strings.Contains(schema.ID, "/internal/") {
		t.Errorf("$id points inside internal/: %q", schema.ID)
	}
}
