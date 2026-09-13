package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ollieread/archdoc/internal/repo"
	"github.com/ollieread/archdoc/internal/template"
	"github.com/spf13/cobra"
)

const (
	// agentsDir is both the directory inside the embedded templates and the
	// directory written under root, so the two cannot drift apart.
	agentsDir = "agents"
	// agentsIndex sits at the top of the repository because that is where the
	// convention puts it, and it is the one file here the repository owns.
	agentsIndex = "AGENTS.md"
)

func newAgentsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "Write or refresh the guides for coding agents",
		Long: "Writes the guides under " + agentsDir + "/, replacing what is there, and creates " + agentsIndex +
			" only when it is absent.\n\n" +
			"The guides ship with the binary and change as ArchDoc changes, so a repository " +
			"scaffolded by an older version carries older guidance. Run this after upgrading.\n\n" +
			agentsIndex + " is never rewritten: it is the file a project adds its own instructions to.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			return refreshAgents(cmd.OutOrStdout(), r)
		},
	}
}

// agentGuides reads every embedded guide, in the order ReadDir sorts them so
// the output does not depend on the filesystem.
func agentGuides() (paths []string, contents [][]byte, err error) {
	entries, err := template.Files.ReadDir(agentsDir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		body, err := template.Files.ReadFile(agentsDir + "/" + e.Name())
		if err != nil {
			return nil, nil, err
		}
		paths = append(paths, agentsDir+"/"+e.Name())
		contents = append(contents, body)
	}
	return paths, contents, nil
}

// refreshAgents replaces the guides and leaves the index alone.
//
// The two halves are owned by different parties: ArchDoc owns everything under
// agents/ and rewrites it freely, and the repository owns AGENTS.md, which is
// where a project's own instructions go. Rewriting that would discard them.
func refreshAgents(out io.Writer, r *repo.Repo) error {
	paths, contents, err := agentGuides()
	if err != nil {
		return err
	}
	for i, path := range paths {
		full := r.File(path)
		verb := "written"
		if _, err := os.Lstat(full); err == nil {
			verb = "updated"
		}
		// A repository scaffolded before --agents existed, or one that declined
		// it, has no agents/ directory. The other writers create their parent
		// the same way.
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := r.WriteFile(path, contents[i]); err != nil {
			return err
		}
		fmt.Fprintf(out, "%-28s %s\n", path, verb)
	}

	index, err := template.Files.ReadFile(agentsIndex)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(r.File(agentsIndex)); err == nil {
		fmt.Fprintf(out, "%-28s kept; it is yours to edit\n", agentsIndex)
		return nil
	}
	if err := r.WriteFile(agentsIndex, index); err != nil {
		return err
	}
	fmt.Fprintf(out, "%-28s written\n", agentsIndex)
	return nil
}
