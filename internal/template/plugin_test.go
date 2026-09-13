package template_test

import (
	"encoding/json"
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

// TestTheMarketplaceManifestResolves guards the file that makes the repository
// installable at all. A plugin is not installed directly: a host adds the
// repository as a marketplace, reads .claude-plugin/marketplace.json at its
// root, and follows each entry's source. A wrong path or a mismatched name
// fails at install time with nothing here to have caught it.
func TestTheMarketplaceManifestResolves(t *testing.T) {
	const manifestPath = "../../.claude-plugin/marketplace.json"
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("the marketplace manifest is missing: %v", err)
	}
	var market struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &market); err != nil {
		t.Fatalf("the marketplace manifest is not valid JSON: %v", err)
	}
	if market.Name == "" {
		t.Error("the marketplace declares no name; it is half of the plugin@marketplace argument")
	}
	if len(market.Plugins) == 0 {
		t.Fatal("the marketplace lists no plugins")
	}

	for _, p := range market.Plugins {
		// source is relative to the repository root, which is two levels up.
		dir := filepath.Join("../..", p.Source)
		own, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
		if err != nil {
			t.Errorf("plugin %q names source %q, which has no .claude-plugin/plugin.json: %v", p.Name, p.Source, err)
			continue
		}
		var plugin struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(own, &plugin); err != nil {
			t.Errorf("the plugin.json at %s is not valid JSON: %v", p.Source, err)
			continue
		}
		if plugin.Name != p.Name {
			t.Errorf("the marketplace calls it %q and its own plugin.json calls it %q; the install argument is built from the marketplace entry",
				p.Name, plugin.Name)
		}
		// A plugin that packages no skills installs cleanly and does nothing.
		entries, err := os.ReadDir(filepath.Join(dir, "skills"))
		if err != nil || len(entries) == 0 {
			t.Errorf("plugin %q packages no skills", p.Name)
		}
	}
}
