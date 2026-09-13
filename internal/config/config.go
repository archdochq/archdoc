// Package config loads and validates archdoc.json.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound reports that no archdoc.json exists in the starting directory or
// any of its ancestors.
var ErrNotFound = errors.New("no archdoc.json found")

// Filename is the name of the configuration file, searched for by Find.
const Filename = "archdoc.json"

// Defaults are exported so that init can scaffold a file holding them rather
// than keeping a second copy of the same numbers.

// Defaults for keys absent from archdoc.json. Name has no default and is
// required.
const (
	DefaultBranch       = "main"
	DefaultRoot         = "."
	DefaultStrict       = true
	DefaultRefStaleDays = 180
)

// Config is the contents of archdoc.json.
type Config struct {
	Name         string `json:"name"`
	Branch       string `json:"branch"`
	Root         string `json:"root"`
	Strict       bool   `json:"strict"`
	RefStaleDays int    `json:"ref_stale_days"`

	// Path is the absolute location of the archdoc.json this was loaded from.
	// It is not part of the file, and naming it here keeps a document that
	// mentions "path" an unknown-field error rather than a way to set it.
	Path string `json:"-"`
}

// Dir is the directory holding archdoc.json. Root is relative to it.
func (c *Config) Dir() string { return filepath.Dir(c.Path) }

// RootDir is the absolute directory holding the four document directories, and
// everything else archdoc generates. ValidateRoot, which Load applies, is what
// keeps it inside the repository.
func (c *Config) RootDir() string { return filepath.Join(c.Dir(), c.Root) }

// ValidateRoot reports whether root names a directory inside the repository.
//
// root exists so that a specification can live in a subdirectory of the project
// it documents, and nothing else: an unchecked "../victim" made archdoc write
// INDEX.md, and create a glossary, in a directory nobody named. The check is
// lexical, which is the same shape as the paths it guards.
func ValidateRoot(root string) error {
	if filepath.IsAbs(root) {
		return fmt.Errorf("root must be relative to the directory holding %s, got %q", Filename, root)
	}
	if clean := filepath.Clean(root); clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("root must not lead outside the directory holding %s, got %q", Filename, root)
	}
	return nil
}

// rootStaysInside checks the resolved root, which ValidateRoot cannot: that
// test is lexical and a symlink is not. A root of "docs" pointing at a
// directory elsewhere passed the spelling check and then had INDEX.md and a
// glossary written into it.
//
// A root that does not exist yet is not an error, because `archdoc init`
// creates it.
func rootStaysInside(c *Config) error {
	root, err := filepath.EvalSymlinks(c.RootDir())
	if err != nil {
		return nil
	}
	base, err := filepath.EvalSymlinks(c.Dir())
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(base, root)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("root resolves to %s, which is outside the directory holding %s", root, Filename)
	}
	return nil
}

// Find walks up from dir looking for archdoc.json and loads the first one it
// finds. It returns ErrNotFound if it reaches the filesystem root first.
func Find(dir string) (*Config, error) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		candidate := filepath.Join(current, Filename)
		switch _, err := os.Stat(candidate); {
		case err == nil:
			return Load(candidate)
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil, fmt.Errorf("%w in %s or any parent directory", ErrNotFound, dir)
		}
		current = parent
	}
}

// Load reads and validates the archdoc.json at path.
func Load(path string) (*Config, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	// Decoding on top of a struct already holding the defaults is what makes an
	// absent key differ from one written as the zero value: encoding/json only
	// assigns the fields the document actually mentions.
	c := &Config{
		Path:         abs,
		Branch:       DefaultBranch,
		Root:         DefaultRoot,
		Strict:       DefaultStrict,
		RefStaleDays: DefaultRefStaleDays,
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// Decode reads one value and leaves the rest of the stream, so a second
	// object or any trailing text was silently ignored. A second decode that
	// must reach EOF is the check; dec.More() is not, because it means "another
	// element of the current array or object" and reports false for a stray
	// brace.
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: unexpected content after the closing brace", path)
	}

	if err := Validate(c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Validate is every check Load applies to a configuration, exported so that
// `archdoc init` can apply them to the settings it is about to write rather
// than discovering them when it reads the file back.
//
// It used to discover them there: init wrote the whole scaffold and then failed
// loading its own archdoc.json, leaving a dead configuration behind that made
// init refuse to run again and every other command fail. The checks it did
// mirror ran before the interactive prompts, so a root typed at the prompt was
// checked by nothing at all.
func Validate(c *Config) error {
	if c.Name == "" {
		return errors.New("name is required and must not be empty")
	}
	if c.RefStaleDays < 0 {
		return fmt.Errorf("ref_stale_days must not be negative, got %d", c.RefStaleDays)
	}
	if err := ValidateRoot(c.Root); err != nil {
		return err
	}
	return rootStaysInside(c)
}
