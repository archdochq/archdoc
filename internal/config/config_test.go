package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ollieread/archdoc/internal/config"
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
