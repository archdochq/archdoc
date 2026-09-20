package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/archdochq/archdoc/internal/lint"
)

// Exit codes, as specified: a usage or validation problem is distinct from a
// repository that is simply not clean, so a script can tell them apart.
const (
	exitOK       = 0
	exitUsage    = 1
	exitFindings = 2
)

// jsonFinding is the wire shape shared by lint, index --check and link. Line is
// a pointer so that a finding with no line omits the field rather than
// reporting line zero.
type jsonFinding struct {
	Path     string `json:"path"`
	Line     *int   `json:"line,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Rule     string `json:"rule,omitempty"`
}

// printFindings writes findings one per line, or as a JSON array.
func printFindings(w io.Writer, findings []lint.Finding, asJSON bool) {
	if asJSON {
		encoded := make([]jsonFinding, 0, len(findings))
		for _, f := range findings {
			entry := jsonFinding{Path: f.Path, Severity: string(f.Severity), Message: f.Message, Rule: f.Rule}
			if f.Line > 0 {
				line := f.Line
				entry.Line = &line
			}
			encoded = append(encoded, entry)
		}
		// Encode, not Marshal: an empty slice must render as [] rather than
		// null, which a consumer expecting a list would choke on.
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(encoded)
		return
	}

	for _, f := range findings {
		if f.Line > 0 {
			fmt.Fprintf(w, "%s:%d: %s: %s\n", f.Path, f.Line, f.Severity, f.Message)
			continue
		}
		fmt.Fprintf(w, "%s: %s: %s\n", f.Path, f.Severity, f.Message)
	}
}

// exitFor is the status a run of findings should end with. Warnings alone are
// not a failure unless the caller asked for that.
func exitFor(findings []lint.Finding, strictWarnings bool) int {
	for _, f := range findings {
		if f.Severity == lint.Error || strictWarnings {
			return exitFindings
		}
	}
	return exitOK
}
