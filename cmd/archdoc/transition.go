package main

import (
	"fmt"
	"slices"
	"time"

	"github.com/ollieread/archdoc/internal/lifecycle"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/spf13/cobra"
)

// transitionCommands builds one command per status a document can be moved to.
// They differ only in the status they name, so they are generated rather than
// written four times.
func transitionCommands() []*cobra.Command {
	moves := []struct {
		verb  string
		to    repo.Status
		short string
	}{
		{"propose", repo.StatusProposed, "Open a draft for comment"},
		{"accept", repo.StatusAccepted, "Record that a proposal stands"},
		{"reject", repo.StatusRejected, "Record that a proposal was turned down"},
		{"withdraw", repo.StatusWithdrawn, "Pull a document before a verdict"},
	}

	commands := make([]*cobra.Command, len(moves))
	for i, move := range moves {
		commands[i] = &cobra.Command{
			Use:   move.verb + " <id>",
			Short: move.short,
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return transition(cmd, args, move.verb, move.to)
			},
		}
	}
	return commands
}

func transition(cmd *cobra.Command, args []string, verb string, to repo.Status) error {
	r, err := openRepo()
	if err != nil {
		return err
	}

	id, err := askForDocument(cmd, args, eligibleFor(r, to), verb)
	if err != nil {
		return err
	}
	d := r.ByID(id)
	if d == nil {
		return fmt.Errorf("no document has the identifier %s", id)
	}

	rewritten, err := lifecycle.Apply(d, to, time.Now())
	if err != nil {
		return err
	}
	if err := r.WriteFile(d.Path, rewritten); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s is now %s\n", d.ID, to)
	if to == repo.StatusRejected {
		fmt.Fprintf(cmd.OutOrStdout(), "write the rejection rationale in %s before committing\n", d.Path)
	}
	return nil
}

// eligibleFor is every document this transition is available for, which is what
// the chooser offers rather than the whole repository.
func eligibleFor(r *repo.Repo, to repo.Status) []*repo.Document {
	strict := r.Config().Strict
	var eligible []*repo.Document
	for _, d := range r.Documents() {
		if !d.Type.HasLifecycle() {
			continue
		}
		if slices.Contains(lifecycle.Permitted(d.FrontMatter.Status, strict), to) {
			eligible = append(eligible, d)
		}
	}
	return eligible
}
