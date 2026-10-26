//go:build ignore

// Command build writes what a release attaches beside its archives: the docs
// bundle, archdoc_<version>_docs.tar.gz, and install.sh with the release set.
// goreleaser runs it as a before hook, so both are checked before anything is
// published and listed in checksums.txt.
//
//	go run ./docs/build.go [-guides <dir>] <directory> <tag> <commit>
//
// It is run from the repository root, and excluded from every build by its
// constraint, as plugin/build.go is.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/template"
)

// absolute is a link that names its scheme, such as https: or mailto:, which a
// guide may link to wherever it is displayed.
var absolute = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// releaseLine is the line install.sh carries with no release set, which the
// attached copy sets to the tag.
const releaseLine = "release="

type contents struct {
	Sections []section `json:"sections"`
}

type section struct {
	Title  string  `json:"title"`
	Guides []guide `json:"guides"`
}

type guide struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

type manifest struct {
	Bundle   int            `json:"bundle"`
	Version  string         `json:"version"`
	Commit   string         `json:"commit"`
	Schemas  map[string]int `json:"schemas"`
	Sections []section      `json:"sections"`
}

// entry is one file of the bundle, in the order it is written.
type entry struct {
	name string
	body []byte
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	guides := flags.String("guides", filepath.Join("docs", "guides"), "the guides' directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	args = flags.Args()
	if len(args) != 3 || slices.Contains(args, "") || *guides == "" {
		return errors.New("usage: go run ./docs/build.go [-guides <dir>] <directory> <tag> <commit>")
	}
	dir, tag, commit := args[0], args[1], args[2]
	if err := onlyOurs(dir); err != nil {
		return err
	}

	// Everything is read before anything is removed, so a failure leaves the
	// directory as it was.
	toc, pages, err := readGuides(*guides)
	if err != nil {
		return err
	}
	reference, err := exec.Command("go", "run", "-ldflags", "-X main.version="+tag, "./cmd/archdoc", "reference").Output()
	if exit := (*exec.ExitError)(nil); errors.As(err, &exit) {
		return fmt.Errorf("archdoc reference: %w: %s", err, bytes.TrimSpace(exit.Stderr))
	}
	if err != nil {
		return fmt.Errorf("archdoc reference: %w", err)
	}
	schemas, err := schemaNumbers(reference)
	if err != nil {
		return err
	}
	process, err := template.Files.ReadFile("PROCESS.md")
	if err != nil {
		return err
	}
	published, err := readSchemas()
	if err != nil {
		return err
	}
	script, err := os.ReadFile("install.sh")
	if err != nil {
		return err
	}
	script, err = withRelease(script, tag)
	if err != nil {
		return err
	}

	listed := manifest{Bundle: 1, Version: tag, Commit: commit, Schemas: schemas, Sections: []section{}}
	for _, s := range toc.Sections {
		in := []guide{}
		for _, g := range s.Guides {
			in = append(in, guide{Path: "guides/" + g.Path, Title: g.Title})
		}
		listed.Sections = append(listed.Sections, section{Title: s.Title, Guides: in})
	}
	encoded, err := json.MarshalIndent(listed, "", "  ")
	if err != nil {
		return err
	}
	entries := []entry{
		{"manifest.json", append(encoded, '\n')},
		{"reference.json", reference},
		{"PROCESS.md", process},
	}
	entries = append(entries, pages...)
	entries = append(entries, published...)
	bundle, err := archive(entries)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	existing, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range existing {
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	name := "archdoc_" + strings.TrimPrefix(tag, "v") + "_docs.tar.gz"
	if err := os.WriteFile(filepath.Join(dir, name), bundle, 0o644); err != nil {
		return err
	}
	installer := filepath.Join(dir, "install.sh")
	if err := os.WriteFile(installer, script, 0o755); err != nil {
		return err
	}
	// WriteFile's mode is narrowed by the umask; the attached script is 0755.
	return os.Chmod(installer, 0o755)
}

// onlyOurs refuses a directory holding anything this did not write, which
// emptying it would delete.
func onlyOurs(dir string) error {
	existing, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range existing {
		bundle, _ := filepath.Match("archdoc_*_docs.tar.gz", e.Name())
		if !e.Type().IsRegular() || (!bundle && e.Name() != "install.sh") {
			return fmt.Errorf("%s holds %s, which this did not write", dir, e.Name())
		}
	}
	return nil
}

// readGuides reads the contents and every guide they list, in their order,
// refusing anything the rules for the guides do not permit.
func readGuides(dir string) (contents, []entry, error) {
	var toc contents
	raw, err := os.ReadFile(filepath.Join(dir, "contents.json"))
	if err != nil {
		return toc, nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&toc); err != nil {
		return toc, nil, fmt.Errorf("contents.json: %w", err)
	}
	if decoder.More() {
		return toc, nil, errors.New("contents.json: more than one JSON value")
	}
	if len(toc.Sections) == 0 {
		return toc, nil, errors.New("contents.json lists no sections")
	}

	listed := map[string]bool{}
	for _, s := range toc.Sections {
		if strings.TrimSpace(s.Title) == "" {
			return toc, nil, errors.New("contents.json: a section has no title")
		}
		if len(s.Guides) == 0 {
			return toc, nil, fmt.Errorf("contents.json: section %q lists no guides", s.Title)
		}
		for _, g := range s.Guides {
			switch {
			case strings.TrimSpace(g.Title) == "":
				return toc, nil, fmt.Errorf("contents.json: %s has no title", g.Path)
			case path.Ext(g.Path) != ".md" || strings.ContainsAny(g.Path, `/\`) || g.Path == ".md":
				return toc, nil, fmt.Errorf("contents.json: %q is not a .md file directly under %s", g.Path, dir)
			case listed[g.Path]:
				return toc, nil, fmt.Errorf("contents.json: %s is listed twice", g.Path)
			}
			listed[g.Path] = true
		}
	}

	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".md" {
			return err
		}
		if rel, _ := filepath.Rel(dir, p); !listed[filepath.ToSlash(rel)] {
			return fmt.Errorf("%s is not listed in contents.json", filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return toc, nil, err
	}

	var pages []entry
	for _, s := range toc.Sections {
		for _, g := range s.Guides {
			body, err := os.ReadFile(filepath.Join(dir, g.Path))
			if err != nil {
				return toc, nil, fmt.Errorf("contents.json lists %s: %w", g.Path, err)
			}
			if err := linksResolve(g.Path, body, listed); err != nil {
				return toc, nil, err
			}
			pages = append(pages, entry{"guides/" + g.Path, body})
		}
	}
	return toc, pages, nil
}

// linksResolve refuses a relative link naming anything but a listed guide, so
// that wherever the guides are displayed together, each link still lands.
func linksResolve(name string, body []byte, listed map[string]bool) error {
	// A guide has no front matter, so it is read as a page whose body is the
	// whole file, which is all Links needs. One opening as if it had some would
	// hide those lines from the check.
	if first, _, _ := strings.Cut(string(body), "\n"); strings.TrimSpace(strings.TrimPrefix(first, "\ufeff")) == "---" {
		return fmt.Errorf("%s opens with ---; a guide has no front matter", name)
	}
	d, _ := repo.Parse(path.Join(string(repo.TypeSpec), name), body)
	for _, l := range d.Links() {
		if absolute.MatchString(l.Target) {
			continue
		}
		target, _, _ := strings.Cut(l.Target, "#")
		if target != "" && !listed[target] {
			return fmt.Errorf("%s:%d: %s is not a listed guide; link anything else by absolute URL", name, l.Line, l.Target)
		}
	}
	return nil
}

// schemaNumbers reads the export's schema number and the reference's own from
// what archdoc reference printed.
func schemaNumbers(reference []byte) (map[string]int, error) {
	var ref struct {
		Schema       int `json:"schema"`
		ExportSchema struct {
			Properties struct {
				Schema struct {
					Const int `json:"const"`
				} `json:"schema"`
			} `json:"properties"`
		} `json:"export_schema"`
	}
	if err := json.Unmarshal(reference, &ref); err != nil {
		return nil, fmt.Errorf("archdoc reference: %w", err)
	}
	export := ref.ExportSchema.Properties.Schema.Const
	if ref.Schema == 0 || export == 0 {
		return nil, errors.New("archdoc reference gives no schema number for itself or the export")
	}
	return map[string]int{"export": export, "reference": ref.Schema}, nil
}

// readSchemas reads every published schema, in path order.
func readSchemas() ([]entry, error) {
	var out []entry
	err := filepath.WalkDir("schema", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(p)
		out = append(out, entry{filepath.ToSlash(p), body})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("reading schema/ (run from the repository root): %w", err)
	}
	return out, nil
}

// withRelease sets the release in install.sh's one unset release line.
func withRelease(script []byte, tag string) ([]byte, error) {
	lines := strings.Split(string(script), "\n")
	at := -1
	for i, line := range lines {
		if line != releaseLine {
			continue
		}
		if at >= 0 {
			return nil, fmt.Errorf("install.sh holds the line %q more than once", releaseLine)
		}
		at = i
	}
	if at < 0 {
		return nil, fmt.Errorf("install.sh holds no line %q", releaseLine)
	}
	lines[at] = releaseLine + tag
	return []byte(strings.Join(lines, "\n")), nil
}

// archive writes entries into a gzipped tar at its root, files only. Every
// header carries the same time, so one commit always gives the same bundle.
func archive(entries []entry) ([]byte, error) {
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		header := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tw.Write(e.body); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
