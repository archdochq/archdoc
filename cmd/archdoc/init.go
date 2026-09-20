package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/archdochq/archdoc/internal/config"
	"github.com/archdochq/archdoc/internal/git"
	"github.com/archdochq/archdoc/internal/index"
	"github.com/archdochq/archdoc/internal/repo"
	"github.com/archdochq/archdoc/internal/template"
	"github.com/spf13/cobra"
)

// workflowPath is where GitHub looks for a workflow. It is relative to the git
// repository root, never to archdoc's root, because GitHub only runs workflows
// from the top of the repository.
const workflowPath = ".github/workflows/archdoc-lint.yml"

// pinnable reports whether a version names a release goreleaser could have
// published. The archive name and the version ldflag are both derived from the
// git tag, so anything that is not a tag names an asset that cannot exist and
// the workflow is better left on "latest".
//
// A pseudo-version has to be excluded by name: it is a valid semantic version
// with a prerelease field, so it passes the tag test, and `go install ...@main`
// produces one. Local patterns rather than golang.org/x/mod, because docs/ARCHDOC.md
// closes the dependency list.
func pinnable(version string) bool {
	return releaseTag.MatchString(version) && !pseudoVersion.MatchString(version)
}

var (
	releaseTag = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
	// The trailing timestamp and commit prefix Go appends to a pseudo-version.
	pseudoVersion = regexp.MustCompile(`-\d{14}-[0-9a-f]{12}$`)
)

// scaffold is one file init writes, with the directory it is written relative
// to. Collecting them before writing anything is what lets init refuse the
// whole operation when one file is in the way.
type scaffold struct {
	dir      string
	rel      string
	contents []byte
}

func newInitCommand() *cobra.Command {
	settings := config.Config{
		Branch:       config.DefaultBranch,
		Root:         config.DefaultRoot,
		Strict:       config.DefaultStrict,
		RefStaleDays: config.DefaultRefStaleDays,
	}
	var licence string
	var agents bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a specification repository in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(dir, config.Filename)); err == nil {
				return fmt.Errorf("%s already exists here", config.Filename)
			}

			if err := applyDefaults(cmd, &settings, dir, &licence, &agents); err != nil {
				return err
			}
			// After the prompts, not before: a root typed at the prompt was
			// checked by nothing, and every check here was otherwise first
			// reached by loading the archdoc.json that had already been
			// written, which left a dead configuration behind.
			settings.Path = filepath.Join(dir, config.Filename)
			if err := config.Validate(&settings); err != nil {
				return err
			}
			files, unpinned, err := plan(dir, settings, licence, agents)
			if err != nil {
				return err
			}
			if unpinned {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"%s is not a published release, so the workflow is set to ARCHDOC_VERSION: latest rather than pinned; pin it before relying on it\n", resolveVersion())
			}
			return write(cmd, files)
		},
	}
	cmd.Flags().StringVar(&settings.Name, "name", "", "project name; defaults to the directory name")
	cmd.Flags().StringVar(&settings.Branch, "branch", "", "branch frozen documents are compared against")
	cmd.Flags().StringVar(&settings.Root, "root", config.DefaultRoot, "directory holding the document directories")
	cmd.Flags().BoolVar(&settings.Strict, "strict", config.DefaultStrict, "keep the transition graph as PROCESS.md describes it")
	// pflag has no notion of a negative flag, so the spelling docs/ARCHDOC.md
	// documents has to be registered as one of its own.
	cmd.Flags().Bool("no-strict", false, "permit any transition between statuses; the opposite of --strict")
	cmd.Flags().IntVar(&settings.RefStaleDays, "ref-stale-days", config.DefaultRefStaleDays, "how old a ref's verified date may be")
	cmd.Flags().StringVar(&licence, "license", "none", "licence to write: none or mit")
	cmd.Flags().BoolVar(&agents, "agents", false, "write AGENTS.md and the agents/ guides, for coding agents working in the repository")
	return cmd
}

// applyDefaults fills in what the user did not supply, prompting on a terminal.
func applyDefaults(cmd *cobra.Command, settings *config.Config, dir string, licence *string, agents *bool) error {
	if *licence != "none" && *licence != "mit" {
		return fmt.Errorf("--license %q is not one of none, mit", *licence)
	}
	if settings.RefStaleDays < 0 {
		return fmt.Errorf("--ref-stale-days must not be negative")
	}
	if cmd.Flags().Changed("root") {
		// Named as a flag problem when it came from the flag. A root that
		// arrives from the prompt is caught by config.Validate afterwards,
		// which is the check that covers every field.
		if err := config.ValidateRoot(settings.Root); err != nil {
			return fmt.Errorf("--root: %w", err)
		}
	}
	if cmd.Flags().Changed("no-strict") {
		if cmd.Flags().Changed("strict") {
			return fmt.Errorf("--strict and --no-strict contradict each other; pass one")
		}
		off, _ := cmd.Flags().GetBool("no-strict")
		settings.Strict = !off
	}
	if settings.Name == "" {
		// A directory called thing-spec is documenting "thing".
		settings.Name = strings.TrimSuffix(filepath.Base(dir), "-spec")
	}
	if settings.Branch == "" {
		settings.Branch = config.DefaultBranch
		if g, err := git.Open(dir); err == nil {
			// CurrentBranch asks symbolic-ref, which fails on a detached head
			// rather than reporting the literal "HEAD" that rev-parse used to,
			// so the default below covers that case and no test for "HEAD" is
			// needed or reachable.
			if branch, err := g.CurrentBranch(); err == nil {
				settings.Branch = branch
			}
		}
	}
	return askForScaffold(cmd, settings, licence, agents)
}

// plan assembles every file init would write, so that nothing is written when
// any of them is already there.
func plan(dir string, settings config.Config, licence string, agents bool) (files []scaffold, unpinned bool, err error) {
	data := template.Data{
		Name:    settings.Name,
		Version: resolveVersion(),
		Year:    time.Now().Format("2006"),
	}
	if !pinnable(data.Version) {
		// A workflow pinned to a release that does not exist fails days later
		// with "release not found"; one pinned to latest works. The caller
		// says so, because the generated file carries a comment claiming it is
		// pinned and silently writing "latest" under it makes the artefact
		// assert something untrue.
		//
		// Asked as "is this a release tag", not "is this the string dev".
		// `go install ...@main` reports a pseudo-version, which is neither dev
		// nor a tag that was ever published, and pinning to it wrote a workflow
		// whose very first CI run could only fail.
		data.Version = "latest"
		unpinned = true
	}

	root := filepath.Join(dir, filepath.FromSlash(settings.Root))
	settings.Path = filepath.Join(dir, config.Filename)

	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, false, err
	}

	for _, name := range []string{"README.md", "PROCESS.md"} {
		rendered, err := renderOrCopy(name, data)
		if err != nil {
			return nil, false, err
		}
		files = append(files, scaffold{root, name, rendered})
	}
	glossary, err := template.Render("glossary.md", template.Data{})
	if err != nil {
		return nil, false, err
	}
	files = append(files,
		scaffold{root, "spec/glossary.md", glossary},
		scaffold{root, "rfc/.gitkeep", nil},
		scaffold{root, "adr/.gitkeep", nil},
		scaffold{root, "ref/.gitkeep", nil},
	)
	if licence == "mit" {
		rendered, err := template.Render("LICENSE.mit", data)
		if err != nil {
			return nil, false, err
		}
		files = append(files, scaffold{root, "LICENSE", rendered})
	}

	if agents {
		// Shipped verbatim, like PROCESS.md: they describe the process rather
		// than this repository, so they carry no substitutions.
		paths, contents, err := agentGuides()
		if err != nil {
			return nil, false, err
		}
		index, err := template.Files.ReadFile(agentsIndex)
		if err != nil {
			return nil, false, err
		}
		files = append(files, scaffold{root, agentsIndex, index})
		for i, path := range paths {
			files = append(files, scaffold{root, path, contents[i]})
		}
	}

	// The workflow goes at the git repository root, wherever that is, and is
	// told where to run archdoc from.
	workflowDir, workingDirectory := workflowLocation(dir)
	data.WorkingDirectory = workingDirectory
	workflow, err := template.Render("archdoc-lint.yml", data)
	if err != nil {
		return nil, false, err
	}
	files = append(files, scaffold{workflowDir, workflowPath, workflow})

	// The index is generated after everything else, but its path is knowable
	// now, so it takes part in the collision check like every other file. Nil
	// contents mark it as written later.
	files = append(files, scaffold{root, indexFile, nil})

	// archdoc.json goes last. A write that fails part-way leaves what came
	// before it, and if the configuration were among those every later attempt
	// would refuse, leaving the user to clean up by hand.
	files = append(files, scaffold{dir, config.Filename, append(encoded, '\n')})

	return files, unpinned, nil
}

// workflowLocation is where the workflow belongs and where it should run
// archdoc from: the git repository root, and the path back to the directory
// holding archdoc.json. Outside a repository the current directory serves as
// both.
func workflowLocation(dir string) (workflowDir, workingDirectory string) {
	g, err := git.Open(dir)
	if err != nil {
		return dir, "."
	}
	top, err := g.RepoRoot()
	if err != nil {
		return dir, "."
	}
	// git is asked where this directory sits inside the repository rather than
	// the answer being computed from two paths obtained different ways. The
	// workflow runs on a checkout, so the value has to be relative to the
	// repository root and nothing else; a local path could never resolve there.
	prefix, err := g.Prefix()
	if err != nil || prefix == "" {
		return top, "."
	}
	return top, filepath.ToSlash(prefix)
}

// renderOrCopy renders a template, or copies it verbatim when it carries no
// substitutions. PROCESS.md is shipped as written and becomes the repository's
// own copy.
func renderOrCopy(name string, data template.Data) ([]byte, error) {
	if name == "PROCESS.md" {
		return template.Files.ReadFile(name)
	}
	return template.Render(name, data)
}

// write creates the scaffold, refusing if anything is already in the way, then
// generates the index so that `archdoc index --check` passes immediately.
func write(cmd *cobra.Command, files []scaffold) error {
	for _, f := range files {
		full := filepath.Join(f.dir, filepath.FromSlash(f.rel))
		// Lstat, not Stat: a dangling symlink is something in the way, and
		// Stat reports it absent, after which the write followed it and
		// created the file outside the tree.
		if _, err := os.Lstat(full); err == nil {
			return fmt.Errorf("%s already exists; nothing has been written", full)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	out := cmd.OutOrStdout()
	for _, f := range files {
		if f.rel == indexFile {
			continue // generated below, once the rest exists
		}
		full := filepath.Join(f.dir, filepath.FromSlash(f.rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		// O_EXCL, as new and ensureGlossary use, so the kernel closes the
		// window between the check above and this write and refuses a symlink
		// that appeared in between.
		if err := writeNew(full, f.contents); err != nil {
			return err
		}
		// Every path is printed, because init can write outside the current
		// directory and that is surprising without a record of it.
		fmt.Fprintln(out, printable(f.dir, f.rel))
	}

	// The index is generated rather than templated: a scaffolded repository
	// must pass the check its own workflow runs.
	written, err := generateIndex(configDir(files))
	if err != nil {
		return err
	}
	fmt.Fprintln(out, written)
	return nil
}

// configDir is where archdoc.json was written, which is where the index
// generator has to look for it.
func configDir(files []scaffold) string {
	for _, f := range files {
		if f.rel == config.Filename {
			return f.dir
		}
	}
	return "."
}

// generateIndex writes INDEX.md for the repository just scaffolded.
func generateIndex(dir string) (string, error) {
	c, err := config.Load(filepath.Join(dir, config.Filename))
	if err != nil {
		return "", err
	}
	r, err := repo.Open(c)
	if err != nil {
		return "", err
	}
	if err := repo.WriteFile(filepath.Join(c.RootDir(), indexFile), index.Generate(r), indexFile); err != nil {
		return "", err
	}
	return path.Join(c.Root, indexFile), nil
}

// printable renders a written path for a human: relative to the current
// directory where it is inside it, absolute where it is not.
func printable(dir, rel string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Join(dir, rel)
	}
	// Getwd returns the logical path while git reports the resolved one, so
	// under a symlinked directory the two never matched and a file plainly
	// inside the current directory was printed as an absolute path.
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	full := filepath.Join(dir, filepath.FromSlash(rel))
	// Both sides resolved, or neither: git reports a resolved path and Getwd a
	// logical one, so comparing one of each printed a file plainly inside the
	// current directory as an absolute path.
	compare := full
	if resolved, err := filepath.EvalSymlinks(full); err == nil {
		compare = resolved
	}
	if within, err := filepath.Rel(cwd, compare); err == nil && !strings.HasPrefix(within, "..") {
		return filepath.ToSlash(within)
	}
	return full
}
