package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/archdochq/archdoc/internal/lint"
)

var sample = []lint.Finding{
	{Path: "rfc/0001-a.md", Line: 4, Severity: lint.Error, Message: "something is wrong", Rule: "L07"},
	{Path: "spec/b.md", Severity: lint.Warning, Message: "something is suspicious", Rule: "L12"},
}

func TestFindingsPrintOnePerLine(t *testing.T) {
	var out bytes.Buffer
	printFindings(&out, sample, false)

	want := "rfc/0001-a.md:4: error: something is wrong\n" +
		"spec/b.md: warning: something is suspicious\n"
	if out.String() != want {
		t.Errorf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

func TestFindingsAsJSONAreABareArray(t *testing.T) {
	var out bytes.Buffer
	printFindings(&out, sample, true)

	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, out.String())
	}
	if len(decoded) != 2 {
		t.Fatalf("got %d findings, want 2", len(decoded))
	}
	if decoded[0]["line"] != float64(4) || decoded[0]["rule"] != "L07" {
		t.Errorf("first finding = %v", decoded[0])
	}
	if _, present := decoded[1]["line"]; present {
		t.Errorf("a finding with no line carries one: %v", decoded[1])
	}
}

func TestFindingsAsJSONAreEmptyArrayNotNull(t *testing.T) {
	var out bytes.Buffer
	printFindings(&out, nil, true)

	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Errorf("output = %q, want %q: null would break a consumer expecting a list", got, "[]")
	}
}

func TestExitCodeFollowsSeverity(t *testing.T) {
	errors := sample
	warnings := []lint.Finding{{Path: "a", Severity: lint.Warning, Message: "m"}}

	for _, tc := range []struct {
		name     string
		findings []lint.Finding
		strict   bool
		want     int
	}{
		{"nothing found", nil, false, exitOK},
		{"a warning alone", warnings, false, exitOK},
		{"a warning under strict-warnings", warnings, true, exitFindings},
		{"an error", errors, false, exitFindings},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitFor(tc.findings, tc.strict); got != tc.want {
				t.Errorf("exitFor = %d, want %d", got, tc.want)
			}
		})
	}
}
