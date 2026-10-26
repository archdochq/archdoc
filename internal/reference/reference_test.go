package reference_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"archdoc.dev/internal/config"
	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/reference"
)

// updateSchema republishes the schema outside internal/. Run
// `go test ./internal/reference -update-schema` after changing it.
var updateSchema = flag.Bool("update-schema", false, "rewrite the published schema from the embedded copy")

const publishedSchema = "../../schema/reference/v1.json"

func TestThePublishedSchemaMatchesTheEmbeddedOne(t *testing.T) {
	if *updateSchema {
		if err := os.MkdirAll(filepath.Dir(publishedSchema), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(publishedSchema, reference.JSONSchema, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(publishedSchema)
	if err != nil {
		t.Fatalf("%s is missing: %v (run go test ./internal/reference -update-schema)", publishedSchema, err)
	}
	if string(got) != string(reference.JSONSchema) {
		t.Errorf("%s differs from the embedded schema; run go test ./internal/reference -update-schema", publishedSchema)
	}

	var schema struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(reference.JSONSchema, &schema); err != nil {
		t.Fatal(err)
	}
	if want := "https://archdoc.dev/schema/reference/v1.json"; schema.ID != want {
		t.Errorf("$id = %q, want %q", schema.ID, want)
	}
}

func TestEveryFieldIsInThePublishedSchema(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(reference.JSONSchema, &schema); err != nil {
		t.Fatalf("the published schema is not valid JSON: %v", err)
	}
	defs, _ := schema["$defs"].(map[string]any)

	for _, c := range []struct {
		def   string
		value any
	}{
		{"", reference.Reference{}},
		{"command", reference.Command{}},
		{"flag", reference.Flag{}},
		{"rule", reference.Rule{}},
		{"change", reference.Change{}},
		{"key", reference.Key{}},
		{"config_key", reference.ConfigKey{}},
		{"exit_code", reference.ExitCode{}},
	} {
		where := schema
		if c.def != "" {
			where, _ = defs[c.def].(map[string]any)
			if where == nil {
				t.Errorf("the schema has no $defs/%s", c.def)
				continue
			}
		}
		properties, _ := where["properties"].(map[string]any)
		declared := map[string]bool{}
		for name := range properties {
			declared[name] = true
		}
		// A field without omitempty is always written, so the schema has to
		// require it, and one with omitempty may be absent, so it must not.
		required := map[string]bool{}
		list, _ := where["required"].([]any)
		for _, name := range list {
			required[name.(string)] = true
		}

		typ := reflect.TypeOf(c.value)
		for i := range typ.NumField() {
			name, options, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			if !declared[name] {
				t.Errorf("%s.%s is written as %q but the schema does not declare it", typ.Name(), typ.Field(i).Name, name)
			}
			if always := !strings.Contains(options, "omitempty"); always != required[name] {
				t.Errorf("%s.%s is always written: %v, but the schema requires %q: %v",
					typ.Name(), typ.Field(i).Name, always, name, required[name])
			}
			delete(declared, name)
		}
		for name := range declared {
			t.Errorf("the schema declares %s property %q, which nothing writes", c.def, name)
		}
	}
}

func TestEveryRuleCodeMatchesTheSchema(t *testing.T) {
	var schema struct {
		Defs struct {
			Rule struct {
				Properties struct {
					Code struct {
						Pattern string `json:"pattern"`
					} `json:"code"`
				} `json:"properties"`
			} `json:"rule"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(reference.JSONSchema, &schema); err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(schema.Defs.Rule.Properties.Code.Pattern)
	for _, rule := range lint.Rules {
		if !pattern.MatchString(rule.Code) {
			t.Errorf("%s does not match the schema's pattern for a code, %s", rule.Code, pattern)
		}
	}
}

func TestBuildFillsEverySection(t *testing.T) {
	// The command tree is cmd/archdoc's to fill, so it is given empty here.
	command := reference.Command{Flags: []reference.Flag{}, GlobalFlags: []reference.Flag{}, Commands: []reference.Command{}}
	ref, err := reference.Build("v0.0.0", command, []reference.ExitCode{})
	if err != nil {
		t.Fatal(err)
	}

	if len(ref.Rules) != len(lint.Rules) {
		t.Errorf("%d rules, want %d", len(ref.Rules), len(lint.Rules))
	}
	for _, typ := range []string{"rfc", "adr", "ref", "spec", "term", "group"} {
		if len(ref.FrontMatter[typ]) == 0 {
			t.Errorf("no front matter keys for %s", typ)
		}
	}
	if len(ref.Config) != len(config.Keys) {
		t.Errorf("%d config keys, want %d", len(ref.Config), len(config.Keys))
	}
	for _, k := range ref.Config {
		if k.Description == "" {
			t.Errorf("config key %s has no description in the export schema", k.Key)
		}
	}
	if !json.Valid(ref.ExportSchema) {
		t.Error("export_schema is not valid JSON")
	}

	encoded, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	if i := strings.Index(string(encoded), `":null`); i >= 0 {
		t.Errorf("the reference holds a null, where every array is an array: ...%s...", encoded[max(0, i-80):i+6])
	}
	// A key with no default omits it, and one with a default carries it in
	// its own type.
	if !strings.Contains(string(encoded), `{"key":"name","type":"string","required":true,"description":`) {
		t.Error("name, which has no default, does not omit default")
	}
	if !strings.Contains(string(encoded), `"key":"ref_stale_days","type":"integer","required":false,"default":180,`) {
		t.Error("ref_stale_days does not carry its default as an integer")
	}
}
