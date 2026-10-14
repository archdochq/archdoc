//go:build ignore

// Command build writes the contents of the archdochq/skills repository: the
// skeleton under plugin/, each embedded guide as a skill, the licence, and the
// release's version in every manifest that carries one. The release workflow
// runs it from the tag; nothing else writes that repository.
//
//	go run ./plugin/build.go <directory> <version>
//
// It is run from the repository root, and excluded from every build by its
// constraint, so go install ./... never produces it.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"archdoc.dev/internal/template"
)

// versioned are the manifests that carry the release's version. The
// marketplace does not: Claude Code reads the version from plugin.json over a
// marketplace entry's.
var versioned = []string{
	".claude-plugin/plugin.json",
	".cursor-plugin/plugin.json",
	"plugin.json",
	"gemini-extension.json",
}

// skillName is the Agent Skills rule for a name: lowercase letters, digits and
// single hyphens, neither leading nor trailing. It is also a directory name
// here, so it is what stops a guide writing outside skills/.
var skillName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 || args[0] == "" || args[1] == "" {
		return errors.New("usage: go run ./plugin/build.go <directory> <version>")
	}
	dir, version := args[0], args[1]
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if err := notThisCheckout(dir); err != nil {
		return err
	}

	// Everything is read before anything is removed, so a failure leaves the
	// directory as it was.
	files, err := skeleton()
	if err != nil {
		return err
	}
	skills, err := guides()
	if err != nil {
		return err
	}
	for name, body := range skills {
		files[filepath.Join("skills", name, "SKILL.md")] = body
	}
	if files["LICENSE"], err = os.ReadFile("LICENSE"); err != nil {
		return err
	}
	for _, path := range versioned {
		if files[path], err = withVersion(files[path], version); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}

	// Emptied first, so a removed guide or a manifest the skeleton no longer
	// carries does not survive into the next release.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	for path, body := range files {
		target := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// notThisCheckout refuses a directory that holds the working directory, which
// emptying it would delete along with any uncommitted work.
func notThisCheckout(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if rel, err := filepath.Rel(abs, wd); err == nil && !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%s holds the working directory; write into a checkout of archdochq/skills", dir)
	}
	return nil
}

// skeleton reads plugin/ without this file, keyed by its path within it.
func skeleton() (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir("plugin", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path == filepath.Join("plugin", "build.go") {
			return err
		}
		rel, err := filepath.Rel("plugin", path)
		if err != nil {
			return err
		}
		files[rel], err = os.ReadFile(path)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("reading the skeleton (run from the repository root): %w", err)
	}
	for _, path := range versioned {
		if files[path] == nil {
			return nil, fmt.Errorf("the skeleton has no %s", path)
		}
	}
	return files, nil
}

// guides returns every embedded guide keyed by the skill name its front matter
// declares.
func guides() (map[string][]byte, error) {
	entries, err := template.Files.ReadDir("agents")
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, e := range entries {
		body, err := template.Files.ReadFile("agents/" + e.Name())
		if err != nil {
			return nil, err
		}
		name := nameOf(string(body))
		switch {
		case name == "":
			return nil, fmt.Errorf("agents/%s declares no name", e.Name())
		case len(name) > 64 || !skillName.MatchString(name):
			return nil, fmt.Errorf("agents/%s declares name %q, which is not a valid skill name", e.Name(), name)
		case out[name] != nil:
			return nil, fmt.Errorf("agents/%s declares name %q, which another guide declares", e.Name(), name)
		}
		out[name] = body
	}
	return out, nil
}

// nameOf reads the name field from a guide's front matter.
func nameOf(body string) string {
	rest, ok := strings.CutPrefix(body, "---\n")
	if !ok {
		return ""
	}
	front, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(front, "\n") {
		if after, ok := strings.CutPrefix(line, "name:"); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// withVersion re-encodes a manifest with version set.
func withVersion(raw []byte, version string) ([]byte, error) {
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	manifest["version"] = version
	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
