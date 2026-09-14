package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ollieread/archdoc/internal/export"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/spf13/cobra"
)

func newExportCommand() *cobra.Command {
	var out string
	var noBodies, schema bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Write the repository as JSON",
		Long: "Emits every document with its front matter, its parsed sections, its links and the " +
			"relationships ArchDoc derives, so a consumer does not reimplement any of it.\n\n" +
			"Bodies are raw Markdown. ArchDoc has no renderer and is not going to acquire one.\n\n" +
			"With --out, writes a tree instead of one document: index.json carrying every document " +
			"without bodies, one file per document beside it, and glossary.json when there is a " +
			"glossary. A page rendering one RFC should not have to fetch the whole specification.\n\n" +
			"The output is deterministic. Nothing in it records when it was generated, so it can be " +
			"cached, diffed and committed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if schema {
				_, err := cmd.OutOrStdout().Write(export.JSONSchema)
				return err
			}
			r, err := openRepo()
			if err != nil {
				return err
			}
			if out == "" {
				return encode(cmd.OutOrStdout(), export.Build(r, export.Options{NoBodies: noBodies}))
			}
			return writeTree(cmd, r, out, noBodies)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "write a tree of files into this directory instead of one document")
	cmd.Flags().BoolVar(&noBodies, "no-bodies", false, "omit document and section bodies")
	cmd.Flags().BoolVar(&schema, "schema", false, "print the JSON Schema the output conforms to and exit")
	return cmd
}

// writeTree splits the export so a consumer fetches only what it renders.
func writeTree(cmd *cobra.Command, r *repo.Repo, dir string, noBodies bool) error {
	full := export.Build(r, export.Options{NoBodies: noBodies})

	// index.json never carries bodies whatever was asked: it exists to be the
	// small one, and the bodies are in the per-document files beside it.
	index := full
	index.Documents = make([]export.Document, len(full.Documents))
	for i, d := range full.Documents {
		d.Body = ""
		for anchor, section := range d.Sections {
			section.Body = ""
			d.Sections[anchor] = section
		}
		index.Documents[i] = d
	}
	index.Glossary = nil

	written := []string{}
	write := func(rel string, v any) error {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := encode(f, v); err != nil {
			return err
		}
		written = append(written, rel)
		return nil
	}

	if err := write("index.json", index); err != nil {
		return err
	}
	for _, d := range full.Documents {
		if err := write(strings.TrimSuffix(d.Path, ".md")+".json", d); err != nil {
			return err
		}
	}
	if len(full.Glossary) > 0 {
		if err := write("glossary.json", full.Glossary); err != nil {
			return err
		}
	}
	for _, rel := range written {
		fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(dir, filepath.FromSlash(rel)))
	}
	return nil
}

func encode(w interface{ Write([]byte) (int, error) }, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Destinations and titles are free text and may contain < > &; escaping
	// them is for HTML, and this is not HTML.
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
