package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/archdochq/archdoc/internal/export"
	"github.com/archdochq/archdoc/internal/repo"
	"github.com/spf13/cobra"
)

func newExportCommand() *cobra.Command {
	var out, commit string
	var noBodies, schema, source bool

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
			opts := export.Options{NoBodies: noBodies, Source: source, Commit: commit}
			if out == "" {
				return encode(cmd.OutOrStdout(), export.Build(r, opts))
			}
			return writeTree(cmd, r, out, opts)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "write a tree of files into this directory instead of one document")
	cmd.Flags().BoolVar(&noBodies, "no-bodies", false, "omit every contents body, keeping the outline")
	cmd.Flags().BoolVar(&source, "source", false, "include each document's file as it is on disk, front matter included")
	cmd.Flags().StringVar(&commit, "commit", "", "record this revision as the one the export describes")
	cmd.Flags().BoolVar(&schema, "schema", false, "print the JSON Schema the output conforms to and exit")
	return cmd
}

// writeTree splits the export so a consumer fetches only what it renders.
func writeTree(cmd *cobra.Command, r *repo.Repo, dir string, opts export.Options) error {
	full := export.Build(r, opts)

	// What the last run left, read before this one overwrites the index it
	// was read from.
	previous, managed := previousFiles(dir)

	// index.json never carries bodies whatever was asked: it exists to be the
	// small one, and the bodies are in the per-document files beside it.
	index := full
	index.Documents = make([]export.Document, len(full.Documents))
	for i, d := range full.Documents {
		d.Source = ""
		// Into a new slice. A Document copied by value shares its caller's
		// Contents backing array, so dropping the bodies here would write
		// through to the documents the per-document files are rendered from
		// below, which is how --out came to emit every section body empty
		// however it was invoked.
		contents := make([]export.Entry, len(d.Contents))
		for j, e := range d.Contents {
			e.Body = nil
			contents[j] = e
		}
		d.Contents = contents
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

	if !managed {
		// Nothing here was written by an export, so nothing here is this
		// command's to delete. A mistyped destination prunes nothing.
		fmt.Fprintln(cmd.ErrOrStderr(), "no previous export found in this directory, so nothing was pruned")
		return nil
	}
	current := make(map[string]bool, len(written))
	for _, rel := range written {
		current[rel] = true
	}
	// Removals go to stderr, leaving stdout the list of files that now exist.
	for _, rel := range slices.Sorted(maps.Keys(previous)) {
		if current[rel] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", rel, err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: removed, no longer in the repository\n", rel)
	}
	return nil
}

// previousFiles is the set of files an earlier export wrote into dir, derived
// from the index.json it left behind. managed is false when there is none, or
// when what is there is not an ArchDoc export.
//
// The index is the manifest, so no second file is needed and the command only
// ever removes what it wrote. A directory it has not written to before holds
// no index to read, so a mistyped --out deletes nothing.
//
// A path is taken from that file only when it is lexically local: the index is
// input, and input naming ../../something must not reach os.Remove.
func previousFiles(dir string) (map[string]bool, bool) {
	body, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return nil, false
	}
	var index struct {
		Schema      *int `json:"schema"`
		HasGlossary bool `json:"has_glossary"`
		Documents   []struct {
			Path string `json:"path"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(body, &index); err != nil || index.Schema == nil {
		return nil, false
	}

	files := map[string]bool{"index.json": true}
	for _, d := range index.Documents {
		if d.Path == "" {
			continue
		}
		rel := strings.TrimSuffix(d.Path, ".md") + ".json"
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			continue
		}
		files[rel] = true
	}
	if index.HasGlossary {
		files["glossary.json"] = true
	}
	return files, true
}

func encode(w interface{ Write([]byte) (int, error) }, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Destinations and titles are free text and may contain < > &; escaping
	// them is for HTML, and this is not HTML.
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
