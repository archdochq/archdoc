package main

import (
	"encoding/json"
	"errors"
	"io/fs"
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

// TestOutPrunesWhatTheRepositoryNoLongerHolds covers the reason --out reads the
// index it is about to overwrite. Without it a document deleted from the
// repository is served from the exported tree for as long as that tree lives.
func TestOutPrunesWhatTheRepositoryNoLongerHolds(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "T"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	for _, title := range []string{"First design", "Second design"} {
		if out, code := run(t, dir, "new", "rfc", title); code != exitOK {
			t.Fatalf("new exited %d: %s", code, out)
		}
	}
	target := filepath.Join(dir, "dist")
	if out, code := run(t, dir, "export", "--out", target); code != exitOK {
		t.Fatalf("export exited %d: %s", code, out)
	}

	second := filepath.Join(target, "rfc", "0002-second-design.json")
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("the second document was not exported: %v", err)
	}
	// Something the export did not write, which it must not touch.
	bystander := filepath.Join(target, "notes.json")
	if err := os.WriteFile(bystander, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(dir, "rfc", "0002-second-design.md")); err != nil {
		t.Fatal(err)
	}
	out, code := run(t, dir, "export", "--out", target)
	if code != exitOK {
		t.Fatalf("second export exited %d: %s", code, out)
	}

	if _, err := os.Stat(second); !errors.Is(err, fs.ErrNotExist) {
		t.Error("a document removed from the repository is still in the tree")
	}
	if !strings.Contains(out, "0002-second-design.json") {
		t.Errorf("the removal was not reported:\n%s", out)
	}
	if _, err := os.Stat(bystander); err != nil {
		t.Errorf("a file the export never wrote was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "rfc", "0001-first-design.json")); err != nil {
		t.Errorf("a document still in the repository was removed: %v", err)
	}
}

// TestOutPrunesNothingWithoutAPreviousIndex is the other half. A directory this
// command has not written to holds nothing it is entitled to delete, so a
// mistyped destination costs nothing.
func TestOutPrunesNothingWithoutAPreviousIndex(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "T"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	target := filepath.Join(dir, "somewhere-else")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	theirs := filepath.Join(target, "important.json")
	if err := os.WriteFile(theirs, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := run(t, dir, "export", "--out", target)
	if code != exitOK {
		t.Fatalf("export exited %d: %s", code, out)
	}
	if _, err := os.Stat(theirs); err != nil {
		t.Errorf("a file in a directory with no previous export was removed: %v", err)
	}
	if !strings.Contains(out, "nothing was pruned") {
		t.Errorf("the command did not say that it pruned nothing:\n%s", out)
	}
}

// TestOutIgnoresAPathLeavingTheDirectory pins the guard on the index being
// input. It is read from disk and its paths reach os.Remove, so one naming
// somewhere else must not be followed.
func TestOutIgnoresAPathLeavingTheDirectory(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "T"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	target := filepath.Join(dir, "dist")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(dir, "victim.json")
	if err := os.WriteFile(victim, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An index claiming a document lives outside the directory it sits in.
	forged := `{"schema":2,"has_glossary":false,"documents":[{"path":"../victim.md"}]}`
	if err := os.WriteFile(filepath.Join(target, "index.json"), []byte(forged), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, code := run(t, dir, "export", "--out", target); code != exitOK {
		t.Fatalf("export exited %d: %s", code, out)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("a path leaving the directory was followed and the file removed: %v", err)
	}
}

// TestOutRefusesToWriteThroughASymbolicLink covers the defect archdoc index
// already had. An export tree is a directory a repository commonly commits, so
// a symlink committed into it followed the write out of the tree and onto any
// file the user could write, while the command reported the path inside it.
func TestOutRefusesToWriteThroughASymbolicLink(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "T"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	target := filepath.Join(dir, "dist")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(dir, "precious.txt")
	const content = "content the user wrote\n"
	if err := os.WriteFile(precious, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(precious, filepath.Join(target, "index.json")); err != nil {
		t.Fatal(err)
	}

	out, code := run(t, dir, "export", "--out", target)
	if code == exitOK {
		t.Errorf("export wrote through a symbolic link and reported success:\n%s", out)
	}
	got, err := os.ReadFile(precious)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("the file the link pointed at was overwritten:\n%s", got)
	}
}
