package template_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updatePlugin rewrites the packaged skills from the shipped guides. Run
// `go test ./internal/template -update-plugin` after changing a guide.
var updatePlugin = flag.Bool("update-plugin", false, "rewrite plugin/skills from the embedded guides")

const pluginSkills = "../../plugin/skills"

// TestThePluginSkillsMatchTheShippedGuides keeps one copy of the guidance
// authoritative.
//
// The same Markdown is delivered three ways: written into a repository by
// `archdoc init --agents`, refreshed there by `archdoc agents`, and packaged as
// skills for hosts that discover them. Three copies of prose that must agree is
// the drift this repository has already been bitten by twice, so the packaged
// copies are generated from the embedded ones and compared byte for byte.
func TestThePluginSkillsMatchTheShippedGuides(t *testing.T) {
	want := map[string][]byte{}
	for name, body := range guides(t) {
		front, ok := frontMatter(body)
		if !ok {
			t.Fatalf("%s has no frontmatter, so it cannot be packaged as a skill", name)
		}
		skill := field(front, "name")
		if skill == "" {
			t.Fatalf("%s declares no skill name", name)
		}
		want[filepath.Join(pluginSkills, skill, "SKILL.md")] = []byte(body)
	}

	if *updatePlugin {
		// Removed first, so a renamed or deleted guide does not leave a skill
		// behind that nothing generates any more.
		if err := os.RemoveAll(pluginSkills); err != nil {
			t.Fatal(err)
		}
		for path, body := range want {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("wrote %d skills into %s", len(want), pluginSkills)
		return
	}

	for path, body := range want {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s is missing: %v (run go test ./internal/template -update-plugin)", path, err)
			continue
		}
		if string(got) != string(body) {
			t.Errorf("%s differs from the guide it is generated from; run go test ./internal/template -update-plugin", path)
		}
	}

	// Nothing packaged that no guide produces.
	entries, err := os.ReadDir(pluginSkills)
	if err != nil {
		t.Fatalf("reading %s: %v", pluginSkills, err)
	}
	for _, e := range entries {
		path := filepath.Join(pluginSkills, e.Name(), "SKILL.md")
		if _, ok := want[path]; !ok {
			t.Errorf("%s is packaged but no guide generates it; run go test ./internal/template -update-plugin", path)
		}
	}
}

// TestThePluginManifestIsPresentAndNamesTheRightSkills stops the plugin from
// shipping skills a host cannot discover.
func TestThePluginManifestIsPresentAndNamesTheRightSkills(t *testing.T) {
	manifest, err := os.ReadFile("../../plugin/.claude-plugin/plugin.json")
	if err != nil {
		t.Fatalf("the plugin manifest is missing: %v", err)
	}
	if !strings.Contains(string(manifest), `"name"`) {
		t.Error("the manifest declares no name")
	}
	// Every skill directory has to hold a SKILL.md, which is what a host looks
	// for; a directory without one is silently ignored.
	entries, err := os.ReadDir(pluginSkills)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("the plugin packages no skills")
	}
	for _, e := range entries {
		if !e.IsDir() {
			t.Errorf("%s is not a directory; a skill is a directory holding SKILL.md", e.Name())
			continue
		}
		if _, err := os.Stat(filepath.Join(pluginSkills, e.Name(), "SKILL.md")); err != nil {
			t.Errorf("%s has no SKILL.md, so a host will ignore it", e.Name())
		}
	}
}
