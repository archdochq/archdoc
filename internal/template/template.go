// Package template holds the files archdoc writes: the document templates, the
// process description, and the scaffolding init produces. They are embedded so
// that the binary carries everything a repository needs.
package template

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Files is every embedded file, exposed so that init can write out the ones it
// scaffolds without rendering them.
//
//go:embed *.md *.yml LICENSE.mit agents
var Files embed.FS

// Data is the substitution set. A field left empty renders as absent rather
// than as an empty value, so the same template serves an ordinary draft and a
// backfilled document.
type Data struct {
	ID      string
	Title   string
	Date    string
	Name    string
	Version string

	// Status is the lifecycle status to create the document in. Backfilled
	// documents arrive terminal; everything else starts as a draft.
	Status string
	// Decided is the date a terminal document reached its status.
	Decided string
	// Backfilled is the date the document was written, when it records a
	// decision taken earlier. Empty for an ordinary document.
	Backfilled string

	// Year is the copyright year, for the licence.
	Year string
	// WorkingDirectory is where the generated workflow runs archdoc: the path
	// from the git repository root to the directory holding archdoc.json.
	WorkingDirectory string
}

// funcs are the functions available to every template.
var funcs = template.FuncMap{
	// yaml encodes a value as a YAML scalar, quoting and escaping only where
	// the encoding requires it. Without it a title containing a colon would
	// produce front matter that does not parse.
	"yaml": func(v any) (string, error) {
		encoded, err := yaml.Marshal(v)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(encoded), "\n"), nil
	},
}

// templates are parsed once. They are compiled into the binary, so one that
// does not parse is a build fault rather than a user's problem, and panicking
// at start turns "this fails for someone, one day, on one subcommand" into
// "this fails in CI, always". PROCESS.md is parsed too; it
// contains no template syntax, and a future file that does must either escape
// its braces or move out of the glob. The agents/ guides are embedded but not
// in the glob: they ship verbatim, so a brace in one is literal text.
var templates = template.Must(template.New("archdoc").Funcs(funcs).ParseFS(Files, "*.md", "*.yml", "LICENSE.mit"))

// Render substitutes data into the named embedded template. A document is
// created as a draft unless the caller says otherwise, so a zero Data never
// writes an empty status.
func Render(name string, data Data) ([]byte, error) {
	if data.Status == "" {
		data.Status = "draft"
	}
	var out bytes.Buffer
	if err := templates.ExecuteTemplate(&out, name, data); err != nil {
		return nil, fmt.Errorf("rendering template %q: %w", name, err)
	}
	return out.Bytes(), nil
}
