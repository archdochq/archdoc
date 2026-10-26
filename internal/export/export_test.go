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

func TestARecordOfAnExchangeCarriesItsDate(t *testing.T) {
	// A consumer computing staleness from verified and ref_stale_days has to
	// pass over a record of an exchange, so as_of is published, and absent
	// rather than empty on a ref that carries none.
	ref := func(id, frontMatter string) string {
		return "---\nid: " + id + "\ntitle: T\nverified: 2026-01-10\n" + frontMatter + "---\n\n# " + id + ": T\n\n## Sources\n\n- Somewhere.\n"
	}
	r := repotest.New(t, map[string]string{
		"ref/0001-review.md": ref("REF-0001", "as_of: 2026-01-08\n"),
		"ref/0002-survey.md": ref("REF-0002", ""),
	})

	raw, err := json.Marshal(export.Build(r, export.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Documents []map[string]any `json:"documents"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, d := range out.Documents {
		byID[d["id"].(string)] = d
	}
	if got := byID["REF-0001"]["as_of"]; got != "2026-01-08" {
		t.Errorf("REF-0001 as_of = %v, want 2026-01-08", got)
	}
	if got, ok := byID["REF-0002"]["as_of"]; ok {
		t.Errorf("REF-0002 carries no as_of, but the export has %v", got)
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

	if len(d.RequiredSections) != 5 {
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
		{"group", export.Group{}},
		{"questions", export.Questions{}},
		{"open_question", export.OpenQuestion{}},
		{"resolved_question", export.ResolvedQuestion{}},
		{"answer", export.Answer{}},
		{"changelog_entry", export.ChangelogEntry{}},
		{"rejection_rationale_section", export.RejectionRationaleSection{}},
	} {
		where := schema
		if c.def != "" {
			where, _ = defs[c.def].(map[string]any)
			if c.def == "questions" {
				// Declared in place on the document rather than in $defs.
				document, _ := defs["document"].(map[string]any)
				properties, _ := document["properties"].(map[string]any)
				where, _ = properties["questions"].(map[string]any)
			}
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
		// A field without omitempty or omitzero is always written, so the
		// schema has to require it, and one with either may be absent, so it
		// must not.
		required := map[string]bool{}
		list, _ := where["required"].([]any)
		for _, name := range list {
			required[name.(string)] = true
		}

		typ := reflect.TypeOf(c.value)
		for i := range typ.NumField() {
			name, options, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			if !declared[name] {
				t.Errorf("%s.%s is exported as %q but the schema does not declare it",
					typ.Name(), typ.Field(i).Name, name)
			}
			omitted := strings.Contains(options, "omitempty") || strings.Contains(options, "omitzero")
			if always := !omitted; always != required[name] {
				t.Errorf("%s.%s is always written: %v, but the schema requires %q: %v",
					typ.Name(), typ.Field(i).Name, always, name, required[name])
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
	var schema struct {
		Defs struct {
			Document struct {
				Required []string `json:"required"`
			} `json:"document"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(export.JSONSchema, &schema); err != nil {
		t.Fatal(err)
	}
	fields := []string{
		"depends", "updates", "obsoletes", "includes",
		"updated_by", "obsoleted_by", "depended_on_by", "included_in",
		"required_sections", "contents", "changelog", "rejection_rationale", "groups", "links",
	}
	for _, field := range fields {
		if !slices.Contains(schema.Defs.Document.Required, field) {
			t.Errorf("the schema does not declare %s required, though every document carries it", field)
		}
	}
	for _, doc := range out.Documents {
		for _, field := range fields {
			raw, present := doc[field]
			if !present {
				t.Errorf("%s is missing; the schema declares it required", field)
				continue
			}
			if string(raw) == "null" {
				t.Errorf("%s serialised as null; the schema declares it an array", field)
			}
		}
		var questions map[string]json.RawMessage
		if err := json.Unmarshal(doc["questions"], &questions); err != nil {
			t.Errorf("questions is not an object: %s", doc["questions"])
		}
		for _, field := range []string{"open", "resolved"} {
			if raw, present := questions[field]; !present || string(raw) == "null" {
				t.Errorf("questions.%s is %s; the schema declares it a required array", field, raw)
			}
		}
	}
}

// updateSchema republishes the schema outside internal/. Run
// `go test ./internal/export -update-schema` after changing it.
var updateSchema = flag.Bool("update-schema", false, "rewrite the published schema from the embedded copy")

const publishedSchema = "../../schema/export/v3.json"

// TestThePublishedSchemaMatchesTheEmbeddedOne gives the contract an address.
//
// The schema is embedded so `archdoc export --schema` can print it without a
// repository, and go:embed cannot reach outside the package, so the copy a
// consumer fetches has to be generated. Its $id is that copy's path on
// archdoc.dev, the same path it has in this repository.
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
	if want := "https://archdoc.dev/" + strings.TrimPrefix(publishedSchema, "../../"); schema.ID != want {
		t.Errorf("$id is %q, want %q", schema.ID, want)
	}
}

// questionsDoc is an RFC with every structured section, and a heading after
// them that stays in contents.
func questionsDoc() string {
	return rfc("RFC-0001", "Q", "proposed", "## Abstract\n\nWords.\n\n"+
		"## Open questions\n\n### Is it open?\n\nDetail.\n\n"+
		"## Resolved questions\n\n### Was it answered?\n\nRaised.\n\n> Yes,\n> twice.\n\n"+
		"## Changelog\n\n- 2026-01-02: Wrote it\n  down.\n- Undated.\n\n## Notes\n\nAfter.\n")
}

func TestQuestionsAndTheChangelogArePublishedAsData(t *testing.T) {
	r := repotest.New(t, map[string]string{"rfc/0001-q.md": questionsDoc()})
	d := export.Build(r, export.Options{}).Documents[0]

	if got := anchorsOf(d.Contents); strings.Join(got, " ") != "rfc-0001-q abstract notes" {
		t.Errorf("contents = %q, want the question sections and the Changelog left out", got)
	}
	if len(d.Questions.Open) != 1 || d.Questions.Open[0].Text != "Is it open?" || d.Questions.Open[0].Anchor != "is-it-open" ||
		d.Questions.Open[0].Line != 20 || *d.Questions.Open[0].Body != "\nDetail.\n" {
		t.Errorf("open questions = %+v", d.Questions.Open)
	}
	if len(d.Questions.Resolved) != 1 {
		t.Fatalf("resolved questions = %+v", d.Questions.Resolved)
	}
	resolved := d.Questions.Resolved[0]
	if resolved.Line != 26 || *resolved.Body != "\nRaised.\n" || resolved.Answer == nil ||
		resolved.Answer.Line != 30 || *resolved.Answer.Body != "Yes,\ntwice." {
		t.Errorf("resolved question = %+v, answer %+v", resolved, resolved.Answer)
	}
	if len(d.Changelog) != 2 || d.Changelog[0].Line != 35 || d.Changelog[0].Date != "2026-01-02" ||
		*d.Changelog[0].Body != "Wrote it\ndown." || d.Changelog[1].Date != "" || *d.Changelog[1].Body != "Undated." {
		t.Errorf("changelog = %+v", d.Changelog)
	}
}

func TestAllHeadingsKeepsTheStructuredSectionsInContents(t *testing.T) {
	r := repotest.New(t, map[string]string{"rfc/0001-q.md": questionsDoc()})
	d := export.Build(r, export.Options{AllHeadings: true}).Documents[0]

	want := "rfc-0001-q abstract open-questions is-it-open resolved-questions was-it-answered changelog notes"
	if got := anchorsOf(d.Contents); strings.Join(got, " ") != want {
		t.Errorf("contents = %q, want every heading", got)
	}
	if len(d.Questions.Open) != 1 || len(d.Changelog) != 2 {
		t.Error("questions and changelog are published under --all-headings too")
	}
}

func TestNoBodiesDropsQuestionAndEntryBodies(t *testing.T) {
	r := repotest.New(t, map[string]string{"rfc/0001-q.md": questionsDoc()})
	d := export.Build(r, export.Options{NoBodies: true}).Documents[0]

	if d.Questions.Open[0].Body != nil || d.Questions.Resolved[0].Body != nil ||
		d.Questions.Resolved[0].Answer.Body != nil || d.Changelog[0].Body != nil {
		t.Error("a body survived --no-bodies")
	}
	if d.Questions.Resolved[0].Answer.Line == 0 || d.Changelog[0].Line == 0 {
		t.Error("--no-bodies dropped more than the bodies")
	}
}

// rejectedDoc is a rejected RFC whose rationale has a subsection, with an H1
// after it that ends the section and stays in contents.
func rejectedDoc() string {
	return rfc("RFC-0002", "R", "rejected", "## Abstract\n\nWords.\n\n"+
		"## Rejection rationale\n\nToo costly.\n\n### Detail\n\nMore.\n\n# Appendix\n\nKept.\n")
}

func TestTheRejectionRationaleIsPublishedAsData(t *testing.T) {
	r := repotest.New(t, map[string]string{"rfc/0002-r.md": rejectedDoc()})
	d := export.Build(r, export.Options{}).Documents[0]

	if got := anchorsOf(d.Contents); strings.Join(got, " ") != "rfc-0002-r abstract appendix" {
		t.Errorf("contents = %q, want the rationale and its subsection left out", got)
	}
	if len(d.RejectionRationale) != 1 {
		t.Fatalf("rejection_rationale = %+v, want one section", d.RejectionRationale)
	}
	s := d.RejectionRationale[0]
	if s.Anchor != "rejection-rationale" || s.Line != 18 || s.Body == nil || *s.Body != "\nToo costly.\n\n### Detail\n\nMore.\n" {
		t.Errorf("rejection rationale = %+v, body %q", s, deref(s.Body))
	}
	last := d.RequiredSections[len(d.RequiredSections)-1]
	if last.Title != "Rejection rationale" || last.Anchor != s.Anchor {
		t.Errorf("required sections end with %+v, want the rationale under its own anchor", last)
	}
}

func TestEveryRejectionRationaleSectionIsPublished(t *testing.T) {
	// Not rejected, and with two: the section leaves contents whatever the
	// status, and a second is legal Markdown that would otherwise vanish.
	r := repotest.New(t, map[string]string{"rfc/0001-a.md": rfc("RFC-0001", "A", "draft",
		"## Rejection rationale\n\nOne.\n\n## Notes\n\nKept.\n\n## Rejection rationale\n\nTwo.\n")})
	d := export.Build(r, export.Options{}).Documents[0]

	if got := anchorsOf(d.Contents); strings.Join(got, " ") != "rfc-0001-a notes" {
		t.Errorf("contents = %q, want both rationale sections left out", got)
	}
	if len(d.RejectionRationale) != 2 || *d.RejectionRationale[0].Body != "\nOne.\n" || *d.RejectionRationale[1].Body != "\nTwo.\n" ||
		d.RejectionRationale[1].Anchor != "rejection-rationale-1" {
		t.Errorf("rejection_rationale = %+v, want both sections in document order", d.RejectionRationale)
	}
}

func TestAllHeadingsKeepsTheRejectionRationaleInContents(t *testing.T) {
	r := repotest.New(t, map[string]string{"rfc/0002-r.md": rejectedDoc()})
	d := export.Build(r, export.Options{AllHeadings: true}).Documents[0]

	if got := anchorsOf(d.Contents); strings.Join(got, " ") != "rfc-0002-r abstract rejection-rationale detail appendix" {
		t.Errorf("contents = %q, want every heading", got)
	}
	if len(d.RejectionRationale) != 1 {
		t.Error("the rejection rationale is published under --all-headings too")
	}
}

func TestNoBodiesDropsTheRejectionRationaleBody(t *testing.T) {
	r := repotest.New(t, map[string]string{"rfc/0002-r.md": rejectedDoc()})
	d := export.Build(r, export.Options{NoBodies: true}).Documents[0]

	if len(d.RejectionRationale) != 1 || d.RejectionRationale[0].Body != nil || d.RejectionRationale[0].Line == 0 {
		t.Errorf("rejection rationale under --no-bodies = %+v, want the section without its body", d.RejectionRationale)
	}
}

func TestGroupsArePublishedApartFromTheDocuments(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": rfc("RFC-0001", "A", "draft", "## Abstract\n\nWords.\n"),
		"spec/thing.md": "---\ntitle: Thing\nincludes: []\n---\n\n# Thing\n",
		"group/b.md":    "---\ntitle: B\ndocuments: [RFC-0001]\n---\n\n# B\n\nSee [thing](../spec/thing.md) and [a](a.md).\n",
		"group/a.md":    "---\ntitle: A\ndocuments: [thing, RFC-0001, RFC-0009]\n---\n\n# A\n",
	})
	out := export.Build(r, export.Options{})

	for _, d := range out.Documents {
		if d.Type == "group" {
			t.Errorf("%s is published among the documents", d.Path)
		}
	}
	if len(out.Groups) != 2 || out.Groups[0].Slug != "a" || out.Groups[1].Slug != "b" {
		t.Fatalf("groups = %+v, want a then b", out.Groups)
	}
	a, b := out.Groups[0], out.Groups[1]
	if a.Title != "A" || a.Path != "group/a.md" || strings.Join(a.Documents, " ") != "thing RFC-0001 RFC-0009" {
		t.Errorf("group a = %+v, want its members as written, the one that resolves to nothing included", a)
	}
	if a.Description == nil || *a.Description != "" || b.Description == nil || !strings.Contains(*b.Description, "See [thing]") {
		t.Errorf("descriptions = %q and %q", deref(a.Description), deref(b.Description))
	}
	if len(b.Links) != 2 || b.Links[0].ResolvesTo != "thing" || b.Links[1].ResolvesTo != "a" || b.Links[1].Path != "group/a.md" {
		t.Errorf("group b's links = %+v, want the spec page and the other group resolved", b.Links)
	}
	for _, d := range out.Documents {
		want := map[string]string{"rfc/0001-a.md": "a b", "spec/thing.md": "a"}[d.Path]
		if got := strings.Join(d.Groups, " "); got != want {
			t.Errorf("%s belongs to %q, want %q", d.Path, got, want)
		}
	}

	if g := export.Build(r, export.Options{NoBodies: true}).Groups[1]; g.Description != nil || len(g.Links) != 2 {
		t.Errorf("under --no-bodies group b = %+v, want no description and its links kept", g)
	}
}

func TestGroupsAreInOrderOfSlugAndNeverHoldANull(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"group/api.md":    "---\ntitle: API\n---\n\n# API\n",
		"group/api-v2.md": "---\ntitle: API v2\ndocuments: []\n---\n\n# API v2\n",
	})
	encoded, err := json.Marshal(export.Build(r, export.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Groups []map[string]json.RawMessage `json:"groups"`
	}
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Groups) != 2 || string(out.Groups[0]["slug"]) != `"api"` || string(out.Groups[1]["slug"]) != `"api-v2"` {
		t.Fatalf("groups = %s, want api then api-v2", encoded)
	}
	// api has no documents key at all, which L01 reports, and neither has links.
	for _, g := range out.Groups {
		for _, field := range []string{"documents", "links"} {
			if raw, present := g[field]; !present || string(raw) == "null" {
				t.Errorf("group %s has %s = %s; the schema declares it a required array", g["slug"], field, raw)
			}
		}
	}
}

func TestGroupsIsAnArrayWhereThereAreNone(t *testing.T) {
	encoded, err := json.Marshal(export.Build(repotest.New(t, nil), export.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"groups":[]`) {
		t.Errorf("a repository with no groups exported %s", encoded)
	}
}

// The schema requires documents as an array, including where there are none,
// as in every freshly initialised repository.
func TestDocumentsIsAnArrayWhereThereAreNone(t *testing.T) {
	encoded, err := json.Marshal(export.Build(repotest.New(t, nil), export.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"documents":[]`) {
		t.Errorf("a repository with no documents exported %s", encoded)
	}
}

// The schema declares status an enum. An invalid one is left out, as an
// invalid date is, rather than published.
func TestAnInvalidStatusIsLeftOut(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": rfc("RFC-0001", "A", "bogus", "## Abstract\n\nWords.\n"),
	})
	out := export.Build(r, export.Options{})
	if len(out.Documents) != 1 {
		t.Fatalf("expected one document, got %d", len(out.Documents))
	}
	encoded, err := json.Marshal(out.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	if raw, present := doc["status"]; present {
		t.Errorf("status %s was exported", raw)
	}
}

// Every case in testdata/lint, broken on purpose, still exports a status the
// schema allows.
func TestEveryExportedStatusIsInTheSchema(t *testing.T) {
	var schema struct {
		Defs struct {
			Document struct {
				Properties struct {
					Status struct {
						Enum []string `json:"enum"`
					} `json:"status"`
				} `json:"properties"`
			} `json:"document"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(export.JSONSchema, &schema); err != nil {
		t.Fatal(err)
	}
	allowed := schema.Defs.Document.Properties.Status.Enum
	if len(allowed) == 0 {
		t.Fatal("the schema declares no status enum")
	}
	for _, d := range export.Build(repotest.Fixture(t, "lint"), export.Options{}).Documents {
		if d.Status != "" && !slices.Contains(allowed, d.Status) {
			t.Errorf("%s exported status %q, which the schema does not allow", d.Path, d.Status)
		}
	}
}

// A term/ holding no terms is a glossary with no entries, so glossary is an
// empty array rather than absent, and agrees with has_glossary.
func TestAnEmptyGlossaryIsAnArray(t *testing.T) {
	encoded, err := json.Marshal(export.Build(repotest.New(t, map[string]string{"term/.gitkeep": ""}), export.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"has_glossary":true`) {
		t.Errorf("an empty term/ exported no glossary: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"glossary":[]`) {
		t.Errorf("an empty term/ exported glossary other than as []: %s", encoded)
	}

	encoded, err = json.Marshal(export.Build(repotest.New(t, nil), export.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"glossary"`) {
		t.Errorf("a repository with no term/ exported %s", encoded)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestAHeadingOfLevelOneEndsAStructuredSection(t *testing.T) {
	r := repotest.New(t, map[string]string{"rfc/0001-q.md": rfc("RFC-0001", "Q", "proposed",
		"## Changelog\n\n- 2026-01-02: One.\n\n# Appendix\n\nKept.\n")})
	d := export.Build(r, export.Options{}).Documents[0]
	if got := anchorsOf(d.Contents); strings.Join(got, " ") != "rfc-0001-q appendix" {
		t.Errorf("contents = %q, want the H1 after the Changelog kept", got)
	}
}
