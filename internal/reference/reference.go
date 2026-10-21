// Package reference describes ArchDoc itself as data: its commands, its lint
// rules, the front matter of each type, its configuration, its exit codes and
// the export schema, so a site documenting a release can be generated from the
// release rather than written again by hand.
//
// The shape is a contract held to the rule ADR-0019 sets for the export: before
// ArchDoc 1.0, a change that removes a field, or changes a field's type or
// meaning, moves Schema to the next integer, at most once between two tagged
// releases. From 1.0 it only gains optional fields.
//
// The command tree is walked in cmd/archdoc, the only package that imports
// cobra, which fills Command.
package reference

import (
	_ "embed"
	"encoding/json"

	"archdoc.dev/internal/config"
	"archdoc.dev/internal/export"
	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/repo"
)

// Schema is the version of the shape below. It changes only for a break.
const Schema = 1

// JSONSchema is the published schema document.
//
//go:embed schema.json
var JSONSchema []byte

// Reference is the whole document `archdoc reference` prints.
type Reference struct {
	Schema       int              `json:"schema"`
	Version      string           `json:"version"`
	Command      Command          `json:"command"`
	Rules        []Rule           `json:"rules"`
	FrontMatter  map[string][]Key `json:"front_matter"`
	Config       []ConfigKey      `json:"config"`
	ExitCodes    []ExitCode       `json:"exit_codes"`
	ExportSchema json.RawMessage  `json:"export_schema"`
}

// Command is one command, with its subcommands in the order help lists them.
type Command struct {
	Path  string `json:"path"`
	Use   string `json:"use"`
	Short string `json:"short"`
	Long  string `json:"long"`
	Flags []Flag `json:"flags"`
	// GlobalFlags are the persistent flags this command defines, which its
	// subcommands inherit.
	GlobalFlags []Flag    `json:"global_flags"`
	Commands    []Command `json:"commands"`
}

// Flag is one flag, its type and default as the flag library gives them.
type Flag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand"`
	Type      string `json:"type"`
	Default   string `json:"default"`
	Usage     string `json:"usage"`
}

// Rule is one lint rule's entry in the registry.
type Rule struct {
	Code    string   `json:"code"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Remedy  string   `json:"remedy"`
	Since   string   `json:"since"`
	Changes []Change `json:"changes"`
}

// Change is a release that changed what a rule reports.
type Change struct {
	Release string `json:"release"`
	Summary string `json:"summary"`
}

// Key is one front matter key of a type.
type Key struct {
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Required   bool     `json:"required"`
	MayBeEmpty bool     `json:"may_be_empty"`
	Values     []string `json:"values"`
}

// ConfigKey is one key of archdoc.json. Default is absent for a key with none.
type ConfigKey struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
	Description string `json:"description"`
}

// ExitCode is one status a command exits with.
type ExitCode struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

// Build assembles the reference from the registries the rest of the binary
// already reads, around the command tree and exit codes cmd/archdoc supplies.
func Build(version string, command Command, exitCodes []ExitCode) (Reference, error) {
	// Each key's description is the export schema's, so the two cannot say
	// different things about the same setting.
	var schema struct {
		Defs struct {
			Config struct {
				Properties map[string]struct {
					Description string `json:"description"`
				} `json:"properties"`
			} `json:"config"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(export.JSONSchema, &schema); err != nil {
		return Reference{}, err
	}

	ref := Reference{
		Schema:       Schema,
		Version:      version,
		Command:      command,
		FrontMatter:  map[string][]Key{},
		ExitCodes:    exitCodes,
		ExportSchema: export.JSONSchema,
	}
	for _, r := range lint.Rules {
		changes := []Change{}
		for _, c := range r.Changes {
			changes = append(changes, Change{Release: c.Release, Summary: c.Summary})
		}
		ref.Rules = append(ref.Rules, Rule{Code: r.Code, Title: r.Title, Summary: r.Summary, Remedy: r.Remedy, Since: r.Since, Changes: changes})
	}
	for _, t := range repo.Types {
		keys := []Key{}
		for _, k := range repo.Keys(t) {
			values := k.Values
			if values == nil {
				values = []string{}
			}
			keys = append(keys, Key{Key: k.Name, Type: string(k.Type), Required: k.Required, MayBeEmpty: k.MayBeEmpty, Values: values})
		}
		ref.FrontMatter[string(t)] = keys
	}
	for _, k := range config.Keys {
		ref.Config = append(ref.Config, ConfigKey{
			Key:         k.Name,
			Type:        k.Type,
			Required:    k.Required,
			Default:     k.Default,
			Description: schema.Defs.Config.Properties[k.Name].Description,
		})
	}
	return ref, nil
}
