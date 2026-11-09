package config_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"archdoc.dev/internal/config"
)

// writeConfig puts an archdoc.json in a fresh directory and returns its path.
func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archdoc.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadReadsEveryField(t *testing.T) {
	path := writeConfig(t, `{
  "name": "TheGamePanel",
  "branch": "trunk",
  "root": "docs",
  "strict": false,
  "ref_stale_days": 90
}`)

	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.Name != "TheGamePanel" {
		t.Errorf("Name = %q, want %q", c.Name, "TheGamePanel")
	}
	if c.Branch != "trunk" {
		t.Errorf("Branch = %q, want %q", c.Branch, "trunk")
	}
	if c.Root != "docs" {
		t.Errorf("Root = %q, want %q", c.Root, "docs")
	}
	if c.Strict {
		t.Error("Strict = true, want false")
	}
	if c.RefStaleDays != 90 {
		t.Errorf("RefStaleDays = %d, want 90", c.RefStaleDays)
	}
}

func TestLoadAppliesDefaultsForMissingKeys(t *testing.T) {
	path := writeConfig(t, `{"name": "Minimal"}`)

	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.Branch != "main" {
		t.Errorf("Branch = %q, want %q", c.Branch, "main")
	}
	if c.Root != "." {
		t.Errorf("Root = %q, want %q", c.Root, ".")
	}
	if !c.Strict {
		t.Error("Strict = false, want true: an absent key must not take Go's zero value")
	}
	if c.RefStaleDays != 180 {
		t.Errorf("RefStaleDays = %d, want 180", c.RefStaleDays)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name     string
		contents string
		want     string // substring the error must mention
	}{
		{"absent name", `{"branch": "main"}`, "name"},
		{"empty name", `{"name": ""}`, "name"},
		{"unknown key", `{"name": "X", "colour": "red"}`, "colour"},
		{"negative ref_stale_days", `{"name": "X", "ref_stale_days": -1}`, "ref_stale_days"},
		{"empty branch", `{"name": "X", "branch": ""}`, "branch"},
		{"branch git refuses", `{"name": "X", "branch": "a: b"}`, "branch"},
		{"key in another case", `{"NAME": "X"}`, "NAME"},
		{"null for a typed key", `{"name": "X", "strict": null}`, "strict"},
		{"acknowledged not an object", `{"name": "X", "acknowledged": ["L07"]}`, "acknowledged"},
		{"acknowledged rules not a list", `{"name": "X", "acknowledged": {"adr/0001-a.md": "L07"}}`, "acknowledged"},
		{"acknowledged rule not a string", `{"name": "X", "acknowledged": {"adr/0001-a.md": [7]}}`, "acknowledged"},
		{"acknowledged rule null", `{"name": "X", "acknowledged": {"adr/0001-a.md": ["L07", null]}}`, "adr/0001-a.md"},
		{"acknowledged rules null", `{"name": "X", "acknowledged": {"adr/0001-a.md": null}}`, "adr/0001-a.md"},
		{"acknowledged rules empty", `{"name": "X", "acknowledged": {"adr/0001-a.md": []}}`, "adr/0001-a.md"},
		{"acknowledged rule repeated", `{"name": "X", "acknowledged": {"adr/0001-a.md": ["L07", "L10", "L07"]}}`, "L07"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(writeConfig(t, tc.contents))
			if err == nil {
				t.Fatalf("Load returned no error, want one mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestLoadReadsAcknowledgedWarnings(t *testing.T) {
	c, err := config.Load(writeConfig(t, `{
  "name": "X",
  "acknowledged": {
    "adr/0003-storage-engine.md": ["L07", "L10"],
    "rfc/0012-retry-policy.md": ["L17"]
  }
}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string][]string{
		"adr/0003-storage-engine.md": {"L07", "L10"},
		"rfc/0012-retry-policy.md":   {"L17"},
	}
	if !reflect.DeepEqual(c.Acknowledged, want) {
		t.Errorf("Acknowledged = %v, want %v", c.Acknowledged, want)
	}
}

func TestLoadLeavesAcknowledgedEmptyWhenAbsent(t *testing.T) {
	c, err := config.Load(writeConfig(t, `{"name": "X"}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Acknowledged == nil || len(c.Acknowledged) != 0 {
		t.Errorf("Acknowledged = %#v, want an empty map", c.Acknowledged)
	}
}

func TestLoadAcceptsValidBranchNames(t *testing.T) {
	for _, branch := range []string{"main", "release/1.0", "it's", "a#b", "feature-x_y", "@", "FETCH_HEAD"} {
		encoded, _ := json.Marshal(branch)
		if _, err := config.Load(writeConfig(t, `{"name": "X", "branch": `+string(encoded)+`}`)); err != nil {
			t.Errorf("branch %q: %v", branch, err)
		}
	}
}

func TestValidateRefusesWhatGitRefusesAsABranch(t *testing.T) {
	for _, branch := range []string{
		"", "HEAD", "-x", "a..b", "a b", "a~b", "a^b", "a:b", "a?b", "a*b", "a[b", `a\b`,
		"a\nb", "/a", "a/", "a//b", ".a", "a/.b", "a.", "a.lock", "a/b.lock/c", "a@{b", "a\x7fb",
	} {
		c := &config.Config{Name: "X", Branch: branch, Root: "."}
		if err := config.Validate(c); err == nil || !strings.Contains(err.Error(), "branch") {
			t.Errorf("branch %q: err = %v, want a refusal naming branch", branch, err)
		}
	}
}

func TestFindWalksUpToLocateConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "archdoc.json"), []byte(`{"name":"Deep"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "spec", "http")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	c, err := config.Find(deep)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if c.Name != "Deep" {
		t.Errorf("Name = %q, want %q", c.Name, "Deep")
	}
	if got, want := c.Dir(), root; got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestFindReportsNotFoundDistinguishably(t *testing.T) {
	_, err := config.Find(t.TempDir())
	if !errors.Is(err, config.ErrNotFound) {
		t.Errorf("error = %v, want it to match config.ErrNotFound", err)
	}
}

func TestRootDirResolvesAgainstTheConfigDirectory(t *testing.T) {
	path := writeConfig(t, `{"name":"Sub","root":"docs"}`)

	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(filepath.Dir(path), "docs")
	if got := c.RootDir(); got != want {
		t.Errorf("RootDir() = %q, want %q", got, want)
	}
}

func TestLoadRejectsARootOutsideTheRepository(t *testing.T) {
	// root exists so the specification can live in a subdirectory. Escaping it
	// makes archdoc write INDEX.md and glossary entries into a directory the
	// user never named.
	for _, root := range []string{"../victim", "docs/../../victim", ".."} {
		path := writeConfig(t, `{"name":"Escape","root":"`+root+`"}`)

		c, err := config.Load(path)
		if err == nil {
			t.Errorf("root %q loaded and resolved to %s, want an error", root, c.RootDir())
			continue
		}
		if !strings.Contains(err.Error(), "root") {
			t.Errorf("root %q: error = %q, want it to name the key", root, err)
		}
	}
}

func TestLoadRejectsAnAbsoluteRoot(t *testing.T) {
	path := writeConfig(t, `{"name":"Absolute","root":"/etc"}`)

	if c, err := config.Load(path); err == nil {
		t.Errorf("an absolute root loaded and resolved to %s, want an error", c.RootDir())
	}
}

func TestLoadAcceptsARootBelowTheRepository(t *testing.T) {
	// The documented use, which the check must not break.
	for _, root := range []string{".", "docs", "a/b", "./docs"} {
		path := writeConfig(t, `{"name":"Nested","root":"`+root+`"}`)

		if _, err := config.Load(path); err != nil {
			t.Errorf("root %q: %v", root, err)
		}
	}
}

func TestLoadRejectsAnythingAfterTheObject(t *testing.T) {
	// Decoder.Decode reads one JSON value and leaves the rest of the stream.
	// Unmarshal would refuse the tail, but it cannot offer DisallowUnknownFields,
	// which is why the decoder is here. So a second object, or plain junk, was
	// read and ignored: a duplicated or appended object silently took effect in
	// part and its unknown keys escaped the check entirely.
	for _, contents := range []string{
		`{"name":"T"} trailing junk`,
		`{"name":"T"}{"name":"U","root":"../victim"}`,
		`{"name":"T"}` + "\n" + `{"unknown":true}`,
	} {
		if _, err := config.Load(writeConfig(t, contents)); err == nil {
			t.Errorf("loaded a file with trailing content: %s", contents)
		}
	}
}

func TestLoadStillAcceptsTrailingWhitespace(t *testing.T) {
	// A trailing newline is what every editor writes.
	for _, contents := range []string{`{"name":"T"}`, `{"name":"T"}` + "\n", `{"name":"T"}` + "\n\n  \t\n"} {
		if _, err := config.Load(writeConfig(t, contents)); err != nil {
			t.Errorf("rejected %q: %v", contents, err)
		}
	}
}

func TestLoadRejectsARootThatIsASymlinkOutOfTheTree(t *testing.T) {
	// The containment check is lexical, and a symlink is not. With root
	// pointing at one, archdoc index wrote INDEX.md into a directory nobody
	// named, which is the guarantee the lexical check was added to make.
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "docs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	path := filepath.Join(dir, "archdoc.json")
	if err := os.WriteFile(path, []byte(`{"name":"Escape","root":"docs"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if c, err := config.Load(path); err == nil {
		t.Errorf("root resolved to %s and loaded, want an error", c.RootDir())
	}
}

func TestLoadAcceptsARootThatIsASymlinkWithinTheTree(t *testing.T) {
	// A symlink is only a problem when it leaves.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "docs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	path := filepath.Join(dir, "archdoc.json")
	if err := os.WriteFile(path, []byte(`{"name":"Inside","root":"docs"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := config.Load(path); err != nil {
		t.Errorf("a symlink pointing inside the tree was refused: %v", err)
	}
}

func TestLoadAppliesTheDefaultsTheTableGives(t *testing.T) {
	c, err := config.Load(writeConfig(t, `{"name": "x"}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Through JSON, so each value is compared as archdoc.json spells it and
	// every key the file can hold is seen.
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var loaded map[string]any
	if err := json.Unmarshal(b, &loaded); err != nil {
		t.Fatal(err)
	}

	jsonType := map[string]string{"string": "string", "bool": "boolean", "float64": "integer", "map[string]interface {}": "object"}
	for _, key := range config.Keys {
		value, ok := loaded[key.Name]
		if !ok && key.Name == "acknowledged" {
			// An empty object is left out of the file init writes, and
			// TestLoadLeavesAcknowledgedEmptyWhenAbsent holds its default.
			continue
		}
		if !ok {
			t.Errorf("%s is in the table but not a key of archdoc.json", key.Name)
			continue
		}
		delete(loaded, key.Name)
		if got := jsonType[fmt.Sprintf("%T", value)]; got != key.Type {
			t.Errorf("%s loads as %s, but the table gives %s", key.Name, got, key.Type)
		}
		if key.Required {
			if key.Default != nil {
				t.Errorf("%s is required and has a default, %v", key.Name, key.Default)
			}
			continue
		}
		var want any
		b, _ := json.Marshal(key.Default)
		_ = json.Unmarshal(b, &want)
		if !reflect.DeepEqual(value, want) {
			t.Errorf("%s loads as %v when absent, but the table gives %v", key.Name, value, want)
		}
	}
	for name := range loaded {
		t.Errorf("%s is a key of archdoc.json missing from the table", name)
	}
}
