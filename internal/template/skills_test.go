package template_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The skeleton is everything the skills repository carries except the skills,
// the versions and the licence, which plugin/build.go adds from a release.
const skeletonDir = "../../plugin"

// versionedManifests are the manifests the generator writes the release's
// version into. The marketplace carries none.
var versionedManifests = []string{
	".claude-plugin/plugin.json",
	".cursor-plugin/plugin.json",
	"plugin.json",
	"gemini-extension.json",
}

// TestTheSkeletonHoldsTheTree pins the skeleton to the files the skills
// repository is published with. A file added here is published to every agent,
// and a version written here would be raised by hand again, which is what the
// generator replaces.
func TestTheSkeletonHoldsTheTree(t *testing.T) {
	want := map[string]bool{"README.md": true, ".claude-plugin/marketplace.json": true}
	for _, path := range versionedManifests {
		want[path] = true
	}

	got := map[string]bool{}
	err := filepath.WalkDir(skeletonDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(skeletonDir, path)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); rel != "build.go" {
			got[rel] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path := range want {
		if !got[path] {
			t.Errorf("plugin/%s is missing from the skeleton", path)
		}
	}
	for path := range got {
		if !want[path] {
			t.Errorf("plugin/%s is in the skeleton but not in the skills repository's tree", path)
		}
	}

	for path := range want {
		if !strings.HasSuffix(path, ".json") {
			continue
		}
		var manifest map[string]any
		raw, err := os.ReadFile(filepath.Join(skeletonDir, path))
		if err == nil {
			err = json.Unmarshal(raw, &manifest)
		}
		if err != nil {
			t.Errorf("plugin/%s: %v", path, err)
			continue
		}
		if manifest["name"] != "archdoc" {
			t.Errorf("plugin/%s names the plugin %v, not archdoc", path, manifest["name"])
		}
		if _, ok := manifest["version"]; ok {
			t.Errorf("plugin/%s declares a version; the generator writes it from the release", path)
		}
	}
}

// TestTheMarketplaceNamesTheRootAsItsSource guards the layout every agent
// relies on: the plugin is the root of the skills repository, which is where
// Gemini CLI reads its manifest and Copilot CLI installs from.
func TestTheMarketplaceNamesTheRootAsItsSource(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(skeletonDir, ".claude-plugin/marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	var market struct {
		Plugins []struct {
			Name   string `json:"name"`
			Source any    `json:"source"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &market); err != nil {
		t.Fatal(err)
	}
	if len(market.Plugins) != 1 || market.Plugins[0].Name != "archdoc" || market.Plugins[0].Source != "." {
		t.Errorf("the marketplace lists %+v; it lists archdoc alone, with source \".\"", market.Plugins)
	}
}

// TestTheGeneratorWritesTheSkillsRepository runs plugin/build.go as the
// release workflow does. Its build constraint keeps it out of go vet ./... and
// every build, so this is also the only thing that compiles it.
func TestTheGeneratorWritesTheSkillsRepository(t *testing.T) {
	out := t.TempDir()
	// Left from an earlier release, so emptying the directory is checked.
	if err := os.WriteFile(filepath.Join(out, "stale.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./plugin/build.go", out, "1.2.3")
	cmd.Dir = "../.."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go run ./plugin/build.go: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(out, "stale.md")); err == nil {
		t.Error("a file from an earlier run survived; the generator empties the directory first")
	}
	if _, err := os.Stat(filepath.Join(out, "LICENSE")); err != nil {
		t.Errorf("LICENSE is missing: %v", err)
	}

	want := map[string]bool{}
	for name, body := range guides(t) {
		front, _ := frontMatter(body)
		skill := field(front, "name")
		want[skill] = true
		got, err := os.ReadFile(filepath.Join(out, "skills", skill, "SKILL.md"))
		if err != nil {
			t.Errorf("%s was not written as skills/%s/SKILL.md: %v", name, skill, err)
			continue
		}
		if string(got) != body {
			t.Errorf("skills/%s/SKILL.md differs from %s", skill, name)
		}
	}
	entries, err := os.ReadDir(filepath.Join(out, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !want[e.Name()] {
			t.Errorf("skills/%s is written but no guide declares it", e.Name())
		}
	}

	for _, path := range versionedManifests {
		var manifest map[string]any
		raw, err := os.ReadFile(filepath.Join(out, path))
		if err == nil {
			err = json.Unmarshal(raw, &manifest)
		}
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if manifest["version"] != "1.2.3" {
			t.Errorf("%s carries version %v, not 1.2.3", path, manifest["version"])
		}
	}
}
