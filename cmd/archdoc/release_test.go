package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"archdoc.dev/internal/template"
)

// The archive and checksum names .goreleaser.yaml produces are a contract with
// archdochq/setup, which builds them from the runner's platform and downloads
// them. A rename here breaks that action, and through it every repository whose
// scaffolded workflow uses it.
//
// Both sides of that contract used to be static text in this repository, when
// the generated workflow reconstructed the names itself. One side now lives in
// another repository, so what follows pins the names that action expects as
// literals. The other side is checked by that repository's own CI, which
// downloads a real release on every run, so a break there is caught after a
// release rather than before one.

const goreleaserConfig = "../../.goreleaser.yaml"

// What archdochq/setup builds, for the one platform this can render.
const (
	setupArchive  = "archdoc_{Version}_linux_amd64.tar.gz"
	setupChecksum = "checksums.txt"
	setupBinary   = "archdoc"
)

func read(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(contents)
}

// scalar returns a top-level key's value from the goreleaser config, or a
// nested one when indented is true.
func scalar(t *testing.T, config, key string, indented bool) string {
	t.Helper()
	prefix := `(?m)^`
	if indented {
		prefix = `(?m)^\s+`
	}
	m := regexp.MustCompile(prefix + regexp.QuoteMeta(key) + `:\s*(\S+)`).FindStringSubmatch(config)
	if m == nil {
		t.Fatalf("no %s in .goreleaser.yaml", key)
	}
	return strings.Trim(m[1], `"'`)
}

func TestGoreleaserPublishesTheNamesSetupDownloads(t *testing.T) {
	// Rendered from the config's own template rather than spelled out again,
	// so renaming the project fails here rather than passing quietly.
	config := read(t, goreleaserConfig)

	project := scalar(t, config, "project_name", false)
	binary := scalar(t, config, "binary", true)

	template := regexp.MustCompile(`name_template:\s*"([^"]+)"`).FindStringSubmatch(config)
	if template == nil {
		t.Fatal("no archive name_template in .goreleaser.yaml")
	}
	format := strings.Trim(scalar(t, config, "formats", true), "[],")

	got := strings.NewReplacer(
		"{{ .ProjectName }}", project,
		"{{ .Version }}", "{Version}",
		"{{ .Os }}", "linux",
		"{{ .Arch }}", "amd64",
	).Replace(template[1]) + "." + format
	if strings.Contains(got, "{{") {
		t.Fatalf("name_template uses a field this test does not render: %q", template[1])
	}
	if got != setupArchive {
		t.Errorf("goreleaser publishes %q but archdochq/setup downloads %q", got, setupArchive)
	}
	if binary != setupBinary {
		t.Errorf("goreleaser builds %q but archdochq/setup extracts %q", binary, setupBinary)
	}
}

func TestGoreleaserWritesTheChecksumFileSetupDownloads(t *testing.T) {
	config := read(t, goreleaserConfig)

	checksum := regexp.MustCompile(`checksum:\s*(?:#[^\n]*\n\s*)*name_template:\s*(\S+)`).FindStringSubmatch(config)
	if checksum == nil {
		t.Fatal("no checksum name_template in .goreleaser.yaml")
	}
	name := strings.Trim(checksum[1], `"'`)
	if strings.Contains(name, "{{") {
		t.Errorf("checksum name_template = %q; archdochq/setup needs a fixed name", name)
	}
	if name != setupChecksum {
		t.Errorf("goreleaser writes %q but archdochq/setup downloads %q", name, setupChecksum)
	}
}

func TestGoreleaserArchivesOnlyListGlobs(t *testing.T) {
	// goreleaser warns on a glob that matches nothing but hard errors on a
	// literal filename, which fails the release outright.
	config := read(t, goreleaserConfig)

	block := regexp.MustCompile(`(?s)\n    files:\n(.*?)\n\n`).FindStringSubmatch(config)
	if block == nil {
		t.Skip("no archive files list")
	}
	for _, line := range strings.Split(block[1], "\n") {
		entry := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		if !strings.ContainsAny(entry, "*?[") {
			t.Errorf("archives list the literal file %q; a missing one fails the release", entry)
		}
	}
}

func TestEveryReleasePlatformIsBuilt(t *testing.T) {
	config := read(t, goreleaserConfig)

	for _, want := range []string{"linux", "darwin", "windows", "amd64", "arm64"} {
		if !strings.Contains(config, want) {
			t.Errorf(".goreleaser.yaml does not build for %s", want)
		}
	}
	if !strings.Contains(config, "main.version") {
		t.Error("the version ldflag is missing, so a release binary would report dev")
	}
}

// The generator and install.sh are run as goreleaser and a user run them, from
// the repository root.
const repositoryRoot = "../.."

// generator builds docs/build.go once, so each case below costs a run rather
// than a compile.
func generator(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "build")
	cmd := exec.Command("go", "build", "-o", bin, "./docs/build.go")
	cmd.Dir = repositoryRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building docs/build.go: %v\n%s", err, out)
	}
	return bin
}

func runGenerator(t *testing.T, bin string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repositoryRoot
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// guidesDir writes a guides directory from files, keyed by name.
func guidesDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const oneGuide = `{"sections": [{"title": "Start", "guides": [{"path": "a.md", "title": "A"}]}]}`

func TestTheGeneratorRefusesWhatTheGuidesDoNotPermit(t *testing.T) {
	bin := generator(t)
	faults := map[string]struct {
		files map[string]string
		says  string
	}{
		"a guide not listed":          {map[string]string{"contents.json": oneGuide, "a.md": "# A\n", "b.md": "# B\n"}, "b.md is not listed"},
		"a guide nested":              {map[string]string{"contents.json": oneGuide, "a.md": "# A\n", "more/b.md": "# B\n"}, "more/b.md is not listed"},
		"a guide listed twice":        {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": "A"}, {"path": "a.md", "title": "A"}]}]}`, "a.md": "# A\n"}, "a.md is listed twice"},
		"a guide listed but absent":   {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": "A"}, {"path": "b.md", "title": "B"}]}]}`, "a.md": "# A\n"}, "contents.json lists b.md"},
		"a section with no title":     {map[string]string{"contents.json": `{"sections": [{"title": "", "guides": [{"path": "a.md", "title": "A"}]}]}`, "a.md": "# A\n"}, "a section has no title"},
		"a guide with no title":       {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": " "}]}]}`, "a.md": "# A\n"}, "a.md has no title"},
		"an empty section":            {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": "A"}]}, {"title": "T", "guides": []}]}`, "a.md": "# A\n"}, `section "T" lists no guides`},
		"a path that is not markdown": {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": "A"}, {"path": "b.txt", "title": "B"}]}]}`, "a.md": "# A\n", "b.txt": "B\n"}, `"b.txt" is not a .md file`},
		"a path in a subdirectory":    {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": "A"}, {"path": "more/b.md", "title": "B"}]}]}`, "a.md": "# A\n", "more/b.md": "# B\n"}, `"more/b.md" is not a .md file`},
		"a link to no listed guide":   {map[string]string{"contents.json": oneGuide, "a.md": "# A\n\nSee [the code](../../cmd/archdoc/main.go).\n"}, "a.md:3: ../../cmd/archdoc/main.go is not a listed guide"},
		"a link to an unlisted guide": {map[string]string{"contents.json": oneGuide, "a.md": "# A\n\nSee [b](b.md#top).\n"}, "a.md:3: b.md#top is not a listed guide"},
		"an unknown key":              {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": "A", "order": 1}]}]}`, "a.md": "# A\n"}, `unknown field "order"`},
		"a second JSON value":         {map[string]string{"contents.json": oneGuide + ` {}`, "a.md": "# A\n"}, "more than one JSON value"},
		"malformed contents":          {map[string]string{"contents.json": `{"sections": [`, "a.md": "# A\n"}, "contents.json: unexpected EOF"},
		"no sections":                 {map[string]string{"contents.json": `null`}, "contents.json lists no sections"},
		"a blank section title":       {map[string]string{"contents.json": `{"sections": [{"title": " ", "guides": [{"path": "a.md", "title": "A"}]}]}`, "a.md": "# A\n"}, "a section has no title"},
		"a backslash in a path":       {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": "A"}, {"path": "more\\b.md", "title": "B"}]}]}`, "a.md": "# A\n"}, `is not a .md file`},
		"a path of .md alone":         {map[string]string{"contents.json": `{"sections": [{"title": "S", "guides": [{"path": ".md", "title": "A"}]}]}`, ".md": "# A\n"}, `".md" is not a .md file`},
		"a guide opening with a BOM":  {map[string]string{"contents.json": oneGuide, "a.md": "\ufeff---\n[x](../../secret.md)\n---\n# A\n"}, "a.md opens with ---"},
		"a link carrying a scheme":    {map[string]string{"contents.json": oneGuide, "a.md": "# A\n\n[x](../x.md?u=http://a)\n"}, "a.md:3: ../x.md?u=http://a is not a listed guide"},
		"a guide opening with ---":    {map[string]string{"contents.json": oneGuide, "a.md": "---\n[x](../../secret.md)\n---\n# A\n"}, "a.md opens with ---"},
	}
	for name, fault := range faults {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			before := filepath.Join(out, "install.sh")
			if err := os.WriteFile(before, []byte("left as it was"), 0o644); err != nil {
				t.Fatal(err)
			}
			said, err := runGenerator(t, bin, "-guides", guidesDir(t, fault.files), out, "v1.2.3", "abc")
			if err == nil || !strings.Contains(said, fault.says) {
				t.Fatalf("the generator gave %v, saying %q; want a failure saying %q", err, said, fault.says)
			}
			if got := read(t, before); got != "left as it was" {
				t.Errorf("the output directory was changed: install.sh holds %q", got)
			}
		})
	}

	t.Run("a missing argument", func(t *testing.T) {
		out := t.TempDir()
		if said, err := runGenerator(t, bin, out, "v1.2.3"); err == nil || !strings.Contains(said, "usage:") {
			t.Errorf("the generator gave %v with two arguments, saying %q", err, said)
		}
		if left, _ := os.ReadDir(out); len(left) != 0 {
			t.Errorf("the generator wrote %v", left)
		}
	})
	t.Run("an empty argument", func(t *testing.T) {
		if said, err := runGenerator(t, bin, t.TempDir(), "", "abc"); err == nil || !strings.Contains(said, "usage:") {
			t.Errorf("the generator gave %v with an empty tag, saying %q", err, said)
		}
	})
	t.Run("an empty guides directory", func(t *testing.T) {
		if said, err := runGenerator(t, bin, "-guides", "", t.TempDir(), "v1.2.3", "abc"); err == nil || !strings.Contains(said, "usage:") {
			t.Errorf("the generator gave %v with -guides empty, saying %q", err, said)
		}
	})
	t.Run("a directory where it writes", func(t *testing.T) {
		out := t.TempDir()
		if err := os.Mkdir(filepath.Join(out, "install.sh"), 0o755); err != nil {
			t.Fatal(err)
		}
		if said, err := runGenerator(t, bin, out, "v1.2.3", "abc"); err == nil || !strings.Contains(said, "holds install.sh") {
			t.Errorf("the generator gave %v for a directory named install.sh, saying %q", err, said)
		}
	})
	t.Run("a file it did not write", func(t *testing.T) {
		out := t.TempDir()
		if err := os.WriteFile(filepath.Join(out, "notes.txt"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if said, err := runGenerator(t, bin, out, "v1.2.3", "abc"); err == nil || !strings.Contains(said, "holds notes.txt") {
			t.Errorf("the generator gave %v for a directory holding a file it did not write, saying %q", err, said)
		}
	})
}

func TestTheGeneratorWritesTheScriptExecutableWhateverTheUmask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no umask")
	}
	bin, _ := filepath.Abs(generator(t))
	out := t.TempDir()
	cmd := exec.Command("sh", "-c", `umask 077 && exec "$0" "$@"`, bin, out, "v1.2.3", "abc")
	cmd.Dir = repositoryRoot
	if said, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, said)
	}
	if info, err := os.Stat(filepath.Join(out, "install.sh")); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("install.sh is %v, %v; want 0755", info.Mode().Perm(), err)
	}
}

func TestTheGeneratorAcceptsLinksBetweenListedGuides(t *testing.T) {
	bin := generator(t)
	guides := guidesDir(t, map[string]string{
		"contents.json": `{"sections": [{"title": "S", "guides": [{"path": "a.md", "title": "A"}, {"path": "b.md", "title": "B"}]}]}`,
		"a.md":          "# A\n\n[b](b.md), [part of b](b.md#part), [here](#a), [site](https://archdoc.dev/), `[code](x.md)`.\n",
		"b.md":          "# B\n\n[a](a.md).\n",
	})
	if said, err := runGenerator(t, bin, "-guides", guides, t.TempDir(), "v1.2.3", "abc"); err != nil {
		t.Errorf("the generator refused links it permits: %v: %s", err, said)
	}
}

func TestTheGeneratorWritesTheBundleAndTheScript(t *testing.T) {
	out := t.TempDir()
	if said, err := runGenerator(t, generator(t), out, "v1.2.3", "0123456789abcdef"); err != nil {
		t.Fatalf("%v: %s", err, said)
	}

	script := read(t, filepath.Join(out, "install.sh"))
	if !strings.Contains(script, "\nrelease=v1.2.3\n") {
		t.Error("install.sh does not set release to the tag")
	}
	if info, err := os.Stat(filepath.Join(out, "install.sh")); err == nil && runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Errorf("install.sh is mode %v, want 0755", info.Mode().Perm())
	}

	entries := untar(t, filepath.Join(out, "archdoc_1.2.3_docs.tar.gz"))
	var toc struct {
		Sections []struct {
			Title  string
			Guides []struct{ Path, Title string }
		}
	}
	if err := json.Unmarshal([]byte(read(t, filepath.Join(repositoryRoot, "docs", "guides", "contents.json"))), &toc); err != nil {
		t.Fatal(err)
	}
	want := []string{"manifest.json", "reference.json", "PROCESS.md"}
	for _, s := range toc.Sections {
		for _, g := range s.Guides {
			want = append(want, "guides/"+g.Path)
		}
	}
	err := filepath.WalkDir(filepath.Join(repositoryRoot, "schema"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(repositoryRoot, p)
			want = append(want, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.name)
	}
	if !slices.Equal(names, want) {
		t.Fatalf("the bundle holds\n%q\nwant\n%q", names, want)
	}

	files := map[string][]byte{}
	for _, e := range entries {
		files[e.name] = e.body
	}
	process, err := template.Files.ReadFile("PROCESS.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["PROCESS.md"], process) {
		t.Error("PROCESS.md is not the embedded copy")
	}
	for name, body := range files {
		if source, ok := strings.CutPrefix(name, "guides/"); ok {
			if !bytes.Equal(body, []byte(read(t, filepath.Join(repositoryRoot, "docs", "guides", source)))) {
				t.Errorf("%s is not the guide byte for byte", name)
			}
		}
	}

	var reference struct {
		Version      string `json:"version"`
		Schema       int    `json:"schema"`
		ExportSchema struct {
			Properties struct {
				Schema struct {
					Const int `json:"const"`
				} `json:"schema"`
			} `json:"properties"`
		} `json:"export_schema"`
	}
	if err := json.Unmarshal(files["reference.json"], &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Version != "v1.2.3" {
		t.Errorf("reference.json is for %q, want the tag", reference.Version)
	}

	var manifest struct {
		Bundle   int            `json:"bundle"`
		Version  string         `json:"version"`
		Commit   string         `json:"commit"`
		Schemas  map[string]int `json:"schemas"`
		Sections []struct {
			Title  string
			Guides []struct{ Path, Title string }
		} `json:"sections"`
	}
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Bundle != 1 || manifest.Version != "v1.2.3" || manifest.Commit != "0123456789abcdef" {
		t.Errorf("manifest = bundle %d, version %q, commit %q", manifest.Bundle, manifest.Version, manifest.Commit)
	}
	export := reference.ExportSchema.Properties.Schema.Const
	if manifest.Schemas["reference"] != reference.Schema || manifest.Schemas["export"] != export || len(manifest.Schemas) != 2 {
		t.Errorf("manifest schemas = %v, want reference %d and export %d", manifest.Schemas, reference.Schema, export)
	}
	for i := range toc.Sections {
		toc.Sections[i].Guides = slices.Clone(toc.Sections[i].Guides)
		for j := range toc.Sections[i].Guides {
			toc.Sections[i].Guides[j].Path = "guides/" + toc.Sections[i].Guides[j].Path
		}
	}
	if fmt.Sprint(manifest.Sections) != fmt.Sprint(toc.Sections) {
		t.Errorf("manifest sections = %+v\nwant the contents under guides/: %+v", manifest.Sections, toc.Sections)
	}
}

type tarEntry struct {
	name string
	body []byte
}

func untar(t *testing.T, path string) []tarEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var out []tarEntry
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag != tar.TypeReg {
			t.Errorf("%s is not a regular file", h.Name)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, tarEntry{h.Name, body})
	}
}

func TestGoreleaserAttachesTheGeneratorsOutput(t *testing.T) {
	config := read(t, goreleaserConfig)
	hook := regexp.MustCompile(`(?m)^\s+- go run \./docs/build\.go (\S+) \{\{ \.Tag \}\} \{\{ \.FullCommit \}\}$`).FindStringSubmatch(config)
	if hook == nil {
		t.Fatal("no before hook runs docs/build.go with the tag and the commit")
	}
	glob := "- glob: " + hook[1] + "/*"
	for _, block := range []string{"release", "checksum"} {
		section := regexp.MustCompile(`(?ms)^` + block + `:\n(.*?)\n\S`).FindStringSubmatch(config + "\nend")
		if section == nil || !strings.Contains(section[1], "extra_files:") || !strings.Contains(section[1], glob) {
			t.Errorf("%s does not attach %s in extra_files", block, hook[1])
		}
	}
}

// installFixture serves an archive for this platform, named from
// .goreleaser.yaml, beside a checksums.txt that checksums builds from its
// digest, and returns install.sh rendered for the release.
func installFixture(t *testing.T, checksums func(name, digest string) string) (script, url string) {
	t.Helper()
	return installFixtureHolding(t, "archdoc", checksums)
}

// installFixtureHolding is installFixture with the archive's one file named
// member.
func installFixtureHolding(t *testing.T, member string, checksums func(name, digest string) string) (script, url string) {
	t.Helper()
	arch := map[string]string{"amd64": "amd64", "arm64": "arm64"}[runtime.GOARCH]
	if (runtime.GOOS != "linux" && runtime.GOOS != "darwin") || arch == "" {
		t.Skipf("install.sh does not support %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	for _, tool := range []string{"sh", "tar", "awk"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}

	config := read(t, goreleaserConfig)
	template := regexp.MustCompile(`name_template:\s*"([^"]+)"`).FindStringSubmatch(config)
	name := strings.NewReplacer(
		"{{ .ProjectName }}", scalar(t, config, "project_name", false),
		"{{ .Version }}", "1.2.3",
		"{{ .Os }}", runtime.GOOS,
		"{{ .Arch }}", arch,
	).Replace(template[1]) + ".tar.gz"

	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	binary := []byte("#!/bin/sh\necho installed\n")
	if err := tw.WriteHeader(&tar.Header{Name: member, Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	tw.Write(binary)
	tw.Close()
	gz.Close()

	assets := t.TempDir()
	sum := sha256.Sum256(archive.Bytes())
	files := map[string]string{name: archive.String(), "checksums.txt": checksums(name, hex.EncodeToString(sum[:]))}
	for file, body := range files {
		if err := os.WriteFile(filepath.Join(assets, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.FileServer(http.Dir(assets)))
	t.Cleanup(server.Close)

	rendered := strings.Replace(read(t, filepath.Join(repositoryRoot, "install.sh")), "\nrelease=\n", "\nrelease=v1.2.3\n", 1)
	script = filepath.Join(t.TempDir(), "install.sh")
	if err := os.WriteFile(script, []byte(rendered), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, server.URL
}

func runInstall(t *testing.T, script, url, dir string, env ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", script)
	// The caller's own ARCHDOC_ settings would override the fixture's.
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "ARCHDOC_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "ARCHDOC_DOWNLOAD_URL="+url, "ARCHDOC_INSTALL_DIR="+dir)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// fakeTool puts a command named name, running body, ahead of the real ones on
// PATH.
func fakeTool(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

// toolsWithout is a PATH directory linking every tool install.sh runs but the
// ones named.
func toolsWithout(t *testing.T, missing ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, tool := range []string{"sh", "uname", "curl", "wget", "sha256sum", "shasum", "cut", "mktemp", "awk", "tar", "gzip", "mkdir", "cp", "chmod", "mv", "rm", "perl"} {
		real, err := exec.LookPath(tool)
		if err != nil || slices.Contains(missing, tool) {
			continue
		}
		if err := os.Symlink(real, filepath.Join(dir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func goodChecksums(name, digest string) string { return digest + "  " + name + "\n" }

func TestInstallScriptRefusesWhatItCannotInstall(t *testing.T) {
	cases := map[string]struct {
		member string
		env    func(t *testing.T, dir string) []string
		says   string
	}{
		"an unsupported system": {"archdoc", func(t *testing.T, _ string) []string {
			return []string{fakeTool(t, "uname", `if [ "$1" = -s ]; then echo Plan9; else echo x86_64; fi`)}
		}, "Plan9 is not supported"},
		"an unsupported architecture": {"archdoc", func(t *testing.T, _ string) []string {
			return []string{fakeTool(t, "uname", `if [ "$1" = -s ]; then echo Linux; else echo mips; fi`)}
		}, "mips is not supported"},
		"an archive without the binary": {"something-else", nil, "holds no archdoc"},
		"an install directory that cannot be created": {"archdoc", func(t *testing.T, dir string) []string {
			blocker := filepath.Join(dir, "file")
			if err := os.WriteFile(blocker, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			return []string{"ARCHDOC_INSTALL_DIR=" + filepath.Join(blocker, "bin")}
		}, "could not create"},
		"a copy that fails": {"archdoc", func(t *testing.T, _ string) []string {
			return []string{fakeTool(t, "cp", "exit 1")}
		}, "could not write to"},
		"no curl or wget": {"archdoc", func(t *testing.T, _ string) []string {
			return []string{"PATH=" + toolsWithout(t, "curl", "wget")}
		}, "neither curl nor wget is installed"},
		"no sha256sum or shasum": {"archdoc", func(t *testing.T, _ string) []string {
			return []string{"PATH=" + toolsWithout(t, "sha256sum", "shasum")}
		}, "neither sha256sum nor shasum is installed"},
		"a release that is not there": {"archdoc", func(t *testing.T, _ string) []string {
			return []string{"ARCHDOC_VERSION=v9.9.9"}
		}, "could not download"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			script, url := installFixtureHolding(t, c.member, goodChecksums)
			dir, temp := t.TempDir(), t.TempDir()
			env := []string{"TMPDIR=" + temp}
			if c.env != nil {
				env = append(env, c.env(t, dir)...)
			}
			said, err := runInstall(t, script, url, dir, env...)
			if err == nil || !strings.Contains(said, c.says) {
				t.Fatalf("install.sh gave %v, saying %q; want a failure saying %q", err, said, c.says)
			}
			for _, d := range []string{dir, temp} {
				left, _ := os.ReadDir(d)
				for _, e := range left {
					if e.Name() != "file" {
						t.Errorf("install.sh left %s in %s", e.Name(), d)
					}
				}
			}
		})
	}
}

func TestInstallScriptNeedsAReleaseUnlessOneIsSet(t *testing.T) {
	_, url := installFixture(t, goodChecksums)
	unset := filepath.Join(repositoryRoot, "install.sh")
	if said, err := runInstall(t, unset, url, t.TempDir()); err == nil || !strings.Contains(said, "set ARCHDOC_VERSION") {
		t.Fatalf("the repository's install.sh gave %v, saying %q", err, said)
	}
	dir := t.TempDir()
	if said, err := runInstall(t, unset, url, dir, "ARCHDOC_VERSION=v1.2.3"); err != nil {
		t.Fatalf("the repository's install.sh with ARCHDOC_VERSION gave %v: %s", err, said)
	}
}

func TestATruncatedInstallScriptRunsNothing(t *testing.T) {
	script, url := installFixture(t, goodChecksums)
	whole := read(t, script)
	cut := strings.TrimSuffix(strings.TrimRight(whole, "\n"), "main")
	if cut == strings.TrimRight(whole, "\n") {
		t.Fatal("install.sh does not end by calling main")
	}
	if err := os.WriteFile(script, []byte(cut), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	runInstall(t, script, url, dir)
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("a truncated install.sh installed %v", left)
	}
}

func TestInstallScriptInstallsTheRelease(t *testing.T) {
	script, url := installFixture(t, func(name, digest string) string {
		return "0000  archdoc_1.2.3_windows_amd64.zip\n" + digest + "  " + name + "\n"
	})
	dir := filepath.Join(t.TempDir(), "bin")
	if said, err := runInstall(t, script, url, dir); err != nil {
		t.Fatalf("%v: %s", err, said)
	}
	out, err := exec.Command(filepath.Join(dir, "archdoc")).Output()
	if err != nil || string(out) != "installed\n" {
		t.Errorf("the installed binary printed %q, %v", out, err)
	}
}

func TestInstallScriptRefusesAnArchiveItCannotVerify(t *testing.T) {
	for name, fault := range map[string]struct {
		checksums func(name, digest string) string
		says      string
	}{
		"a wrong digest":          {func(name, _ string) string { return strings.Repeat("0", 64) + "  " + name + "\n" }, "does not match checksums.txt"},
		"no line for the archive": {func(_, digest string) string { return digest + "  archdoc_1.2.3_other.tar.gz\n" }, "checksums.txt has no line for"},
	} {
		t.Run(name, func(t *testing.T) {
			script, url := installFixture(t, fault.checksums)
			dir := t.TempDir()
			if said, err := runInstall(t, script, url, dir); err == nil || !strings.Contains(said, fault.says) {
				t.Fatalf("install.sh gave %v, saying %q; want a failure saying %q", err, said, fault.says)
			}
			if left, _ := os.ReadDir(dir); len(left) != 0 {
				t.Errorf("install.sh left %v in the install directory", left)
			}
		})
	}
}

func TestInstallScriptLeavesAPackageManagersLinkAlone(t *testing.T) {
	script, url := installFixture(t, func(name, digest string) string { return digest + "  " + name + "\n" })
	dir := t.TempDir()
	link := filepath.Join(dir, "archdoc")
	if err := os.Symlink("/opt/homebrew/bin/archdoc", link); err != nil {
		t.Fatal(err)
	}
	if said, err := runInstall(t, script, url, dir); err == nil || !strings.Contains(said, "is a symbolic link") {
		t.Fatalf("install.sh gave %v for a symbolic link, saying %q", err, said)
	}
	if target, err := os.Readlink(link); err != nil || target != "/opt/homebrew/bin/archdoc" {
		t.Errorf("the link now reads %q, %v", target, err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 1 {
		t.Errorf("install.sh left %v in the install directory", left)
	}
}

func TestInstallScriptRefusesADirectoryInTheBinarysPlace(t *testing.T) {
	script, url := installFixture(t, func(name, digest string) string { return digest + "  " + name + "\n" })
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "archdoc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if said, err := runInstall(t, script, url, dir); err == nil || !strings.Contains(said, "is a directory") {
		t.Fatalf("install.sh gave %v for a directory, saying %q", err, said)
	}
	if left, _ := os.ReadDir(filepath.Join(dir, "archdoc")); len(left) != 0 {
		t.Errorf("install.sh wrote %v into the directory", left)
	}
}
