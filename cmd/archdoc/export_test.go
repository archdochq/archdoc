package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exported(t *testing.T, dir string, args ...string) map[string]any {
	t.Helper()
	out, code := run(t, dir, append([]string{"export"}, args...)...)
	if code != exitOK {
		t.Fatalf("export exited %d: %s", code, out)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("export did not emit valid JSON: %v\n%s", err, out)
	}
	return parsed
}

// TestExportEmitsJSONAndNothingElse keeps the output pipeable. Anything printed
// alongside the document, a progress line or a notice, makes it unparseable.
func TestExportEmitsJSONAndNothingElse(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "T"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	if out, code := run(t, dir, "new", "rfc", "A design"); code != exitOK {
		t.Fatalf("new exited %d: %s", code, out)
	}
	got := exported(t, dir)
	if got["schema"] != float64(2) {
		t.Errorf("schema = %v, want 2", got["schema"])
	}
	config, _ := got["config"].(map[string]any)
	if config["name"] != "T" {
		t.Errorf("config.name = %v, want T", config["name"])
	}
	if _, present := got["name"]; present {
		t.Error("name is still at the top level, where it was before it moved into config")
	}
	docs, _ := got["documents"].([]any)
	if len(docs) == 0 {
		t.Error("no documents were exported")
	}
}

// TestExportSchemaPrintsTheContract lets a consumer fetch the schema from the
// binary that produced the data, rather than hoping a copy elsewhere matches.
func TestExportSchemaPrintsTheContract(t *testing.T) {
	dir := t.TempDir()
	out, code := run(t, dir, "export", "--schema")
	if code != exitOK {
		t.Fatalf("export --schema exited %d: %s", code, out)
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(out), &schema); err != nil {
		t.Fatalf("--schema did not emit valid JSON: %v", err)
	}
	if _, ok := schema["$schema"]; !ok {
		t.Error("the output does not declare $schema, so it is not a JSON Schema")
	}
	// It must work without a repository: a consumer reads the contract before
	// it has anything to validate. Matched against the error's own words
	// rather than the bare filename, which the schema itself now mentions when
	// describing where the config object comes from.
	if strings.Contains(out, "no archdoc.json found") {
		t.Errorf("--schema failed for want of a repository:\n%s", out)
	}
}

// TestExportOutWritesATree covers the shape a website wants: a page rendering
// one RFC should not fetch the whole specification.
func TestExportOutWritesATree(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "T"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	if out, code := run(t, dir, "new", "rfc", "A design"); code != exitOK {
		t.Fatalf("new exited %d: %s", code, out)
	}
	target := filepath.Join(dir, "dist")
	out, code := run(t, dir, "export", "--out", target)
	if code != exitOK {
		t.Fatalf("export --out exited %d: %s", code, out)
	}

	for _, want := range []string{"index.json", filepath.Join("rfc", "0001-a-design.json")} {
		path := filepath.Join(target, want)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was not written: %v", want, err)
		}
		if !strings.Contains(out, want) {
			t.Errorf("%s was written but not reported:\n%s", want, out)
		}
	}

	// index.json is the small one whatever else was asked, because the bodies
	// are in the per-document files beside it.
	body, err := os.ReadFile(filepath.Join(target, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index struct {
		Documents []struct {
			Source   string `json:"source"`
			Contents []struct {
				Anchor string  `json:"anchor"`
				Body   *string `json:"body"`
			} `json:"contents"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(body, &index); err != nil {
		t.Fatal(err)
	}
	for _, d := range index.Documents {
		if d.Source != "" {
			t.Error("index.json carries the file itself")
		}
		if len(d.Contents) == 0 {
			t.Error("index.json dropped the outline along with the bodies")
		}
		for _, e := range d.Contents {
			if e.Body != nil {
				t.Errorf("index.json carries the body of %s", e.Anchor)
			}
		}
	}

	// The per-document file is the one that does carry them. index.json is
	// built by copying each document, and a copy shares its caller's Contents
	// backing array, so dropping the bodies for the index emptied them here
	// too until the slice was cloned.
	one, err := os.ReadFile(filepath.Join(target, "rfc", "0001-a-design.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Contents []struct {
			Anchor string  `json:"anchor"`
			Body   *string `json:"body"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(one, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Contents) == 0 {
		t.Fatal("the per-document file lists no contents, so the check below proves nothing")
	}
	carried := 0
	for _, e := range doc.Contents {
		if e.Body == nil {
			t.Errorf("the per-document file carries no body for %s", e.Anchor)
			continue
		}
		if *e.Body != "" {
			carried++
		}
	}
	if carried == 0 {
		t.Error("every body in the per-document file is empty, so nothing carries the prose")
	}
}
