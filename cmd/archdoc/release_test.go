package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
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
