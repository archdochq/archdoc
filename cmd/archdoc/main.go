// Command archdoc manages a spec repository. Everything this package does is
// argument handling: it parses flags, prompts for what it was not given, calls
// the packages under internal/, prints what they return and chooses an exit
// code. No rule, no format and no edit is decided here.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/charmbracelet/x/term"
	"github.com/ollieread/archdoc/internal/config"
	"github.com/ollieread/archdoc/internal/git"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/spf13/cobra"
)

// version is set by the linker on a release build. Failing that it is taken
// from the module's build information, which go install fills in, and only then
// does it fall back to dev.
var version = ""

func main() {
	if err := newRoot(os.Stdout, os.Stderr).Execute(); err != nil {
		var quiet silentExit
		if errors.As(err, &quiet) {
			os.Exit(int(quiet))
		}
		fmt.Fprintln(os.Stderr, "archdoc:", err)
		os.Exit(exitUsage)
	}
}

// silentExit ends the process with a status and prints nothing further. A
// command that has already written its findings returns it: the findings are
// the message, and the status is the only thing left to say.
type silentExit int

func (e silentExit) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// newRoot builds the command tree. Output is a parameter so the tests can read
// what a command wrote.
func newRoot(out, errOut io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:   "archdoc",
		Short: "Manage a specification repository",
		// Errors are printed by main in one format, and a failing command has
		// already said what was wrong, so cobra should not add a usage dump.
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       resolveVersion(),
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetVersionTemplate("{{.Version}}\n")
	root.PersistentFlags().Bool("no-interaction", false, "never prompt; use defaults and fail if a required value is missing")
	root.PersistentFlags().StringP("chdir", "C", "", "run as if archdoc had been started in this directory")
	// Applied before anything reads the working directory, because config.Find
	// walks up from it and every command's notion of "here" follows. A spec
	// repository is commonly a sibling of the code repository it documents, so
	// without this an agent working in the code has to wrap every invocation in
	// a cd, which a fresh shell per command makes unreliable.
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		dir, err := cmd.Flags().GetString("chdir")
		if err != nil || dir == "" {
			return err
		}
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("--chdir %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("--chdir %s: not a directory", dir)
		}
		return os.Chdir(dir)
	}

	root.AddCommand(
		newInitCommand(),
		newLintCommand(),
		newIndexCommand(),
		newLinkCommand(),
		newNewCommand(),
		newTermCommand(),
		newAgentsCommand(),
	)
	root.AddCommand(transitionCommands()...)
	return root
}

// resolveVersion prefers the linker's value, then the module version that go
// install records, and says dev when it has neither.
func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// openRepo finds archdoc.json by walking up and opens the repository around it.
func openRepo() (*repo.Repo, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	c, err := config.Find(dir)
	if errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("%w; run archdoc init to create one", err)
	}
	if err != nil {
		return nil, err
	}
	return repo.Open(c)
}

// openGit returns the repository's git, or nil when there is not one. Only
// git's own "this is not a repository" means nil: any other failure, git
// missing from PATH above all, is an error, because treating it as "no
// repository" would skip every frozen-document check and still succeed.
func openGit(r *repo.Repo) (git.Repository, error) {
	g, err := git.Open(r.Config().Dir())
	if errors.Is(err, git.ErrNotARepository) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return g, nil
}

// interactive reports whether the command may prompt: stdin must be a terminal
// and the user must not have asked to be left alone.
//
// The test is IsTerminal rather than "is a character device". /dev/null is a
// character device, and redirecting from it is how a shell, Makefile or job
// runner says there is no input; prompting there hangs. Stdin is read from the
// command so that a test can supply it.
func interactive(cmd *cobra.Command) bool {
	if noInteraction, _ := cmd.Flags().GetBool("no-interaction"); noInteraction {
		return false
	}
	return onTerminal(cmd)
}

// onTerminal is a variable so that a test can exercise the flag check above it.
// Every test runs with a non-terminal stdin, which made that check unreachable:
// it could be deleted with the whole suite green, and --no-interaction would
// then do nothing on a terminal, blocking a wrapper script on a form instead of
// failing as documented.
var onTerminal = func(cmd *cobra.Command) bool {
	in, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(in.Fd())
}
