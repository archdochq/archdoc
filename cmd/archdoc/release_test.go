package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The generated workflow downloads a named asset from a release that
// .goreleaser.yaml produces, and every scaffolded repository is pinned to the
// release it was written for. So the two names are a contract, and a rename on
// either side breaks repositories already in the wild.
//
// docs/DECISIONS.md recorded that nothing could verify this until a real release
// existed. That was wrong: both names are static text in this repository.

const (
	goreleaserConfig = "../../.goreleaser.yaml"
	workflowTemplate = "../../internal/template/archdoc-lint.yml"
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

func TestTheWorkflowAndGoreleaserAgreeOnAssetNames(t *testing.T) {
	// Both sides are resolved from their own file and then compared. The
	// previous version substituted the literal "archdoc" into the workflow's
	// filename and never read project_name at all, which made it exactly
	// backwards: renaming the project alone passed, and renaming both sides
	// together, which keeps the contract, failed.
	config, workflow := read(t, goreleaserConfig), read(t, workflowTemplate)

	project := scalar(t, config, "project_name", false)
	binary := scalar(t, config, "binary", true)

	template := regexp.MustCompile(`name_template:\s*"([^"]+)"`).FindStringSubmatch(config)
	if template == nil {
		t.Fatal("no archive name_template in .goreleaser.yaml")
	}
	format := scalar(t, config, "formats", true)
	format = strings.Trim(format, "[],")

	// The filename goreleaser would write for the platform the workflow asks
	// for, rendered from the config's own template.
	want := strings.NewReplacer(
		"{{ .ProjectName }}", project,
		"{{ .Version }}", "{Version}",
		"{{ .Os }}", "linux",
		"{{ .Arch }}", "amd64",
	).Replace(template[1]) + "." + format
	if strings.Contains(want, "{{") {
		t.Fatalf("name_template uses a field this test does not render: %q", template[1])
	}

	// The filename the workflow builds, with only its shell expansion reduced.
	archive := regexp.MustCompile(`archive="([^"]+)"`).FindStringSubmatch(workflow)
	if archive == nil {
		t.Fatal("the workflow does not build an archive name")
	}
	if got := strings.ReplaceAll(archive[1], "${tag#v}", "{Version}"); got != want {
		t.Errorf("the workflow downloads %q but goreleaser publishes %q", got, want)
	}

	// And the binary inside it, which the workflow extracts and installs by
	// name. The name is captured and compared, not matched as a prefix:
	// "archdoc" is a substring of "archdoc-cli", so a Contains check passes on
	// a rename that breaks the install step.
	for _, step := range []struct {
		what    string
		pattern *regexp.Regexp
	}{
		{"extracts", regexp.MustCompile(`tar -xzf "\$archive" (\S+)`)},
		{"installs", regexp.MustCompile(`install -m 0755 (\S+) /usr/local/bin/(\S+)`)},
	} {
		m := step.pattern.FindStringSubmatch(workflow)
		if m == nil {
			t.Errorf("the workflow no longer %s the binary", step.what)
			continue
		}
		for _, got := range m[1:] {
			if got != binary {
				t.Errorf("the workflow %s %q, but goreleaser builds %q", step.what, got, binary)
			}
		}
	}
}

func TestTheWorkflowDownloadsTheChecksumFileGoreleaserWrites(t *testing.T) {
	config, workflow := read(t, goreleaserConfig), read(t, workflowTemplate)

	checksum := regexp.MustCompile(`checksum:\s*(?:#[^\n]*\n\s*)*name_template:\s*(\S+)`).FindStringSubmatch(config)
	if checksum == nil {
		t.Fatal("no checksum name_template in .goreleaser.yaml")
	}
	name := strings.Trim(checksum[1], `"'`)
	if !strings.Contains(workflow, name) {
		t.Errorf("goreleaser writes %q but the workflow does not download it", name)
	}
	if strings.Contains(name, "{{") {
		t.Errorf("checksum name_template = %q; the workflow needs a fixed name", name)
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
