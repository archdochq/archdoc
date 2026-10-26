package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"archdoc.dev/internal/reference"
)

// commandPaths lists every command in the tree, depth first.
func commandPaths(c reference.Command) []string {
	paths := []string{c.Path}
	for _, sub := range c.Commands {
		paths = append(paths, commandPaths(sub)...)
	}
	return paths
}

// flagNames lists every flag and global flag in the tree.
func flagNames(c reference.Command) []string {
	var names []string
	for _, f := range append(c.Flags, c.GlobalFlags...) {
		names = append(names, c.Path+" --"+f.Name)
	}
	for _, sub := range c.Commands {
		names = append(names, flagNames(sub)...)
	}
	return names
}

func TestReferenceDescribesTheCommandTree(t *testing.T) {
	// Outside any repository, since it needs none.
	out, code := run(t, t.TempDir(), "reference")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d:\n%s", code, exitOK, out)
	}
	if strings.Contains(out, `": null`) {
		t.Error("the reference holds a null, where every array is an array")
	}
	var ref reference.Reference
	if err := json.Unmarshal([]byte(out), &ref); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	if ref.Schema != reference.Schema || ref.Version != resolveVersion() {
		t.Errorf("schema %d, version %q; want %d, %q", ref.Schema, ref.Version, reference.Schema, resolveVersion())
	}
	if got := ref.Command.Commands[slices.IndexFunc(ref.Command.Commands, func(c reference.Command) bool {
		return c.Path == "archdoc new"
	})].Use; got != "new <rfc|adr|ref> <title>" {
		t.Errorf("archdoc new's use = %q, want the command's own usage", got)
	}
	paths := strings.Join(commandPaths(ref.Command), "\n") + "\n"
	for _, want := range []string{"archdoc\n", "archdoc new\n", "archdoc term add\n", "archdoc accept\n"} {
		if !strings.Contains(paths, want) {
			t.Errorf("no command %q among:\n%s", strings.TrimSpace(want), paths)
		}
	}
	for _, unwanted := range []string{"archdoc reference\n", "archdoc help\n", "archdoc completion"} {
		if strings.Contains(paths, unwanted) {
			t.Errorf("%q is listed, but hidden commands, help and completion are left out", strings.TrimSpace(unwanted))
		}
	}

	flags := strings.Join(flagNames(ref.Command), "\n") + "\n"
	for _, want := range []string{"archdoc --version\n", "archdoc --chdir\n", "archdoc new --backfill\n"} {
		if !strings.Contains(flags, want) {
			t.Errorf("no flag %q among:\n%s", strings.TrimSpace(want), flags)
		}
	}
	if strings.Contains(flags, "--help\n") {
		t.Error("--help is listed, but every command has it")
	}
	// A persistent flag is the root's, not repeated on every command.
	if strings.Count(flags, "--chdir\n") != 1 {
		t.Errorf("--chdir is listed %d times, want once on the root", strings.Count(flags, "--chdir\n"))
	}

	if len(ref.ExitCodes) != 3 {
		t.Errorf("%d exit codes, want 0, 1 and 2", len(ref.ExitCodes))
	}
	for i, c := range ref.ExitCodes {
		if c.Code != i || c.Meaning == "" {
			t.Errorf("exit code %d = %+v, want code %d with a meaning", i, c, i)
		}
	}
}

func TestReferenceSchemaPrintsTheEmbeddedSchema(t *testing.T) {
	out, code := run(t, t.TempDir(), "reference", "--schema")

	if code != exitOK || out != string(reference.JSONSchema) {
		t.Errorf("exit %d, and the output is not the embedded schema", code)
	}
}

func TestReferenceIsHiddenFromHelp(t *testing.T) {
	out, _ := run(t, t.TempDir(), "--help")

	// renumber's description says "reference", so the command's own row.
	if strings.Contains(out, "  reference ") {
		t.Errorf("archdoc --help lists reference:\n%s", out)
	}
}
