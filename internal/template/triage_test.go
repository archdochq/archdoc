package template_test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/template"
)

// updateTriage rewrites the triage guide's table from the rule registry. Run
// `go test ./internal/template -update-triage` after changing a rule entry.
var updateTriage = flag.Bool("update-triage", false, "rewrite the triage guide's table of rules from the registry")

const (
	triagePath   = "agents/triage.md"
	triageHeader = "| Rule | What it means | Remedy |\n| --- | --- | --- |\n"
)

// triageTable renders the registry as the rows under the guide's header.
func triageTable() string {
	var b strings.Builder
	for _, rule := range lint.Rules {
		b.WriteString("| " + rule.Code + " | " + rule.Summary + " | " + rule.Remedy + " |\n")
	}
	return b.String()
}

func TestTheTriageTableIsTheRegistry(t *testing.T) {
	b, err := template.Files.ReadFile(triagePath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", triagePath, err)
	}
	guide := string(b)
	start := strings.Index(guide, triageHeader)
	if start < 0 {
		t.Fatalf("%s has no table headed %q", triagePath, triageHeader)
	}
	start += len(triageHeader)
	end := strings.Index(guide[start:], "\n\n")
	if end < 0 {
		t.Fatalf("%s's table is not followed by a blank line", triagePath)
	}
	end += start + 1

	want := triageTable()
	if *updateTriage {
		if err := os.WriteFile(triagePath, []byte(guide[:start]+want+guide[end:]), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if got := guide[start:end]; got != want {
		t.Errorf("%s's table differs from the registry; run go test ./internal/template -update-triage\ngot:\n%s\nwant:\n%s", triagePath, got, want)
	}
}
