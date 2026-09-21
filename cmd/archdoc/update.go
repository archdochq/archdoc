package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/template"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// refresh is one file archdoc generated and would generate differently now.
type refresh struct {
	dir  string // absolute directory rel is resolved against
	rel  string // slash-separated, for display and for writing
	want []byte
}

func (f refresh) path() string { return filepath.Join(f.dir, filepath.FromSlash(f.rel)) }

func newUpdateCommand() *cobra.Command {
	var check, yes bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Refresh the files archdoc generates",
		Long: "Regenerates PROCESS.md, the workflow, and the agents/ guides when that directory is " +
			"present, then shows what would change and asks before writing.\n\n" +
			"These files ship with the binary and change as ArchDoc changes, so a repository " +
			"scaffolded by an older version carries older copies, including any that were wrong.\n\n" +
			"Nothing the repository owns is touched: AGENTS.md, README.md, archdoc.json, LICENSE and " +
			"every document. agents/ is refreshed but never created; archdoc agents installs it.\n\n" +
			"Regenerating the workflow moves the version pin to the binary running now, which " +
			"changes the ArchDoc your CI uses. The diff shows it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			planned, notes, err := plannedRefresh(r)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			var changed []refresh
			for _, f := range planned {
				// A missing file reads as empty, so the diff shows it arriving.
				have, err := os.ReadFile(f.path())
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				if bytes.Equal(have, f.want) {
					continue
				}
				changed = append(changed, f)
				fmt.Fprint(out, unifiedDiff(f.rel, string(have), string(f.want)))
			}
			for _, note := range notes {
				fmt.Fprintln(out, note)
			}
			if len(changed) == 0 {
				fmt.Fprintln(out, "everything ArchDoc generates is already current")
				return nil
			}
			if check {
				return silentExit(exitFindings)
			}
			if !yes {
				// Refused rather than assumed: the realistic way this loses
				// work is a script running it over a customised workflow.
				if !interactive(cmd) {
					return fmt.Errorf("update would overwrite %d file(s); pass --yes to confirm, or --check to see the diff",
						len(changed))
				}
				agreed := false
				if err := huh.NewConfirm().
					Title(fmt.Sprintf("Overwrite %d file(s) with the versions above?", len(changed))).
					Description("Anything you edited in them is replaced.").
					Value(&agreed).Run(); err != nil {
					return err
				}
				if !agreed {
					fmt.Fprintln(out, "nothing written")
					return nil
				}
			}
			for _, f := range changed {
				if err := os.MkdirAll(filepath.Dir(f.path()), 0o755); err != nil {
					return err
				}
				if err := repo.WriteFile(f.path(), f.want, f.rel); err != nil {
					return err
				}
				fmt.Fprintln(out, f.rel)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "show what would change and exit without writing")
	cmd.Flags().BoolVar(&yes, "yes", false, "apply without asking")
	return cmd
}

// plannedRefresh is every file archdoc would generate for this repository as it
// is now, plus notes about what it deliberately left alone.
func plannedRefresh(r *repo.Repo) (files []refresh, notes []string, err error) {
	c := r.Config()
	root, configDir := c.RootDir(), c.Dir()

	process, err := template.Files.ReadFile("PROCESS.md")
	if err != nil {
		return nil, nil, err
	}
	files = append(files, refresh{root, "PROCESS.md", process})

	workflowDir, workingDirectory := workflowLocation(configDir)
	data := template.Data{Name: c.Name, WorkingDirectory: workingDirectory}
	// The pin says which ArchDoc this repository's CI runs. Raising it is the
	// point: this command also rewrites PROCESS.md, which describes what lint
	// enforces, so leaving CI on an older ArchDoc would leave the repository
	// documenting one set of rules while enforcing another.
	//
	// It is only held back when this binary has nothing better to offer. An
	// unreleased build resolves to a version that names no downloadable asset,
	// and writing "latest" over a real version unpins the repository rather
	// than updating it.
	data.Version = resolveVersion()
	if !pinnable(data.Version) {
		data.Version = pinnedVersion(filepath.Join(workflowDir, filepath.FromSlash(workflowPath)))
		if data.Version == "" {
			data.Version = "latest"
		}
	}
	workflow, err := template.Render("archdoc-lint.yml", data)
	if err != nil {
		return nil, nil, err
	}
	files = append(files, refresh{workflowDir, workflowPath, workflow})

	// Refreshed, never created: installing them is opting in, which is what
	// `archdoc agents` is for.
	if _, statErr := os.Stat(filepath.Join(root, agentsDir)); statErr == nil {
		paths, contents, err := agentGuides()
		if err != nil {
			return nil, nil, err
		}
		for i, path := range paths {
			files = append(files, refresh{root, path, contents[i]})
		}
	} else {
		notes = append(notes, "agents/ is not present and was not created; archdoc agents installs it")
	}
	notes = append(notes, "AGENTS.md, README.md and archdoc.json are yours and were not read")
	return files, notes, nil
}

// pinnedVersion reads the version a workflow already names, so regenerating it
// does not move the pin. Empty when there is no workflow to read or no pin in
// it, which is when the running binary's own version is the best available
// answer.
func pinnedVersion(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// The pin is the version input of the action the workflow uses. It was an
	// ARCHDOC_VERSION env var while the workflow installed archdoc itself, and
	// both names are read: a repository scaffolded before the workflow changed
	// still carries the old one, and reading only the new name would find
	// nothing there and quietly rewrite a pinned repository to latest.
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		for _, key := range []string{"version:", "ARCHDOC_VERSION:"} {
			if after, ok := strings.CutPrefix(line, key); ok {
				return strings.TrimSpace(after)
			}
		}
	}
	return ""
}
