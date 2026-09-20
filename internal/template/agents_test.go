package template_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/archdochq/archdoc/internal/template"
)

// guides reads every shipped agent guide, keyed by its filename.
func guides(t *testing.T) map[string]string {
	t.Helper()
	entries, err := template.Files.ReadDir("agents")
	if err != nil {
		t.Fatalf("ReadDir(agents): %v", err)
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		b, err := template.Files.ReadFile("agents/" + e.Name())
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", e.Name(), err)
		}
		out[e.Name()] = string(b)
	}
	if len(out) == 0 {
		t.Fatal("no agent guides are embedded")
	}
	return out
}

// citation matches "PROCESS.md, Some Section" in either the parenthesised or
// the inline form, which is how the guides refer to the process.
var citation = regexp.MustCompile(`PROCESS\.md, ([A-Z][A-Za-z ]+?)[,)]`)

// TestEveryCitedProcessSectionExists is the guard against the failure this
// repository has already had twice: prose derived from PROCESS.md that quietly
// stops matching it. The guides cite by section name rather than line number
// precisely so this can be checked, and so an edit to PROCESS.md that shifts
// every line does not invalidate them.
func TestEveryCitedProcessSectionExists(t *testing.T) {
	process, err := template.Files.ReadFile("PROCESS.md")
	if err != nil {
		t.Fatalf("ReadFile(PROCESS.md): %v", err)
	}
	headings := map[string]bool{}
	for _, line := range strings.Split(string(process), "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), "## "); ok {
			headings[after] = true
		}
	}
	if len(headings) == 0 {
		t.Fatal("PROCESS.md has no H2 headings, so no citation could ever resolve")
	}

	cited := 0
	for name, body := range guides(t) {
		for _, m := range citation.FindAllStringSubmatch(body, -1) {
			section := strings.TrimSpace(m[1])
			cited++
			if !headings[section] {
				t.Errorf("%s cites PROCESS.md section %q, which does not exist; headings are %v",
					name, section, keys(headings))
			}
		}
	}
	if cited == 0 {
		t.Error("no guide cites PROCESS.md, so this test is guarding nothing")
	}
	t.Logf("checked %d citations against %d headings", cited, len(headings))
}

// TestEveryGuideIsAlsoAValidSkill pins the property that lets one file serve as
// both a shipped guide and a skill: the frontmatter a skill host requires.
// Every file under agents/ is a guide, so every one of them needs it.
func TestEveryGuideIsAlsoAValidSkill(t *testing.T) {
	for name, body := range guides(t) {
		front, ok := frontMatter(body)
		if !ok {
			t.Errorf("%s has no frontmatter, so it cannot be used as a skill", name)
			continue
		}
		for _, key := range []string{"name", "description"} {
			value := field(front, key)
			if value == "" {
				t.Errorf("%s frontmatter has no %s", name, key)
			}
		}
		if got := field(front, "name"); got != "" && !strings.HasPrefix(got, "archdoc-") {
			t.Errorf("%s declares name %q; skill names are prefixed archdoc- so they do not collide", name, got)
		}
	}
}

// TestGuidesShipVerbatim asserts the guides are not run through the template
// engine, so a brace in one is literal text rather than a parse failure at
// start-up or a silent substitution.
func TestGuidesShipVerbatim(t *testing.T) {
	for name := range guides(t) {
		if _, err := template.Render("agents/"+name, template.Data{}); err == nil {
			t.Errorf("agents/%s is registered as a template; it must ship verbatim", name)
		}
	}
}

func frontMatter(body string) (string, bool) {
	rest, ok := strings.CutPrefix(body, "---\n")
	if !ok {
		return "", false
	}
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func field(front, key string) string {
	for _, line := range strings.Split(front, "\n") {
		if after, ok := strings.CutPrefix(line, key+":"); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestTheIndexIsProseAndOwnedByTheRepository pins the split that makes
// `archdoc agents` safe to re-run. Everything under agents/ belongs to ArchDoc
// and is replaced wholesale; AGENTS.md belongs to the repository, is created
// once and never rewritten, and is where a project puts its own instructions.
// It is prose rather than a skill, because it is read by convention rather than
// discovered by name.
func TestTheIndexIsProseAndOwnedByTheRepository(t *testing.T) {
	body, err := template.Files.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md): %v", err)
	}
	if _, ok := frontMatter(string(body)); ok {
		t.Error("AGENTS.md carries skill frontmatter; it is prose the repository owns")
	}
	// It is only useful if it sends the reader on to the guides.
	if !strings.Contains(string(body), "agents/working.md") {
		t.Error("AGENTS.md does not point at agents/working.md, so nothing leads to the guides")
	}
	if _, err := template.Files.ReadFile("agents/working.md"); err != nil {
		t.Errorf("AGENTS.md points at agents/working.md, which is not shipped: %v", err)
	}
}
