package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"archdoc.dev/internal/config"
	"archdoc.dev/internal/repo"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// Prompting substitutes for values the command was not given, and for nothing
// else. When stdin is not a terminal, or --no-interaction was passed, a missing
// required value is a usage error instead.

// runForm is a variable so that a test can assert no form was opened. Every
// test runs with a non-terminal stdin, where a form cannot be driven.
var runForm = (*huh.Form).Run

// askForNew fills in the type and title of a new document and, under
// --backfill, the status and dates it was not given, in one form. Where the
// type or title is asked for, as_of is too, once the type is a ref; it opens
// no form of its own, since most refs are not records of an exchange.
func askForNew(cmd *cobra.Command, args []string, backfill *backfillOptions, asOf *string) (kind, title string, err error) {
	if len(args) > 0 {
		kind = args[0]
	}
	if len(args) > 1 {
		title = args[1]
	}
	if !interactive(cmd) {
		if kind == "" || title == "" {
			return "", "", fmt.Errorf("new needs a type and a title: archdoc new <rfc|adr|ref> <title>")
		}
		return kind, title, nil
	}

	// Decided before any field is built: binding a select writes its first
	// option into kind at once, which would read as an RFC given.
	offerAsOf := !cmd.Flags().Changed("as-of") && !backfill.On && (kind == "" || kind == string(repo.TypeRef))

	var fields []huh.Field
	if kind == "" {
		options := []huh.Option[string]{
			huh.NewOption("RFC: a design the spec will have a page for", string(repo.TypeRFC)),
			huh.NewOption("ADR: a decision that constrains designs", string(repo.TypeADR)),
		}
		if *asOf != "" {
			// Only a ref carries as_of.
			options = nil
		}
		if !backfill.On {
			// A ref has no status, so it cannot be backfilled.
			options = append(options, huh.NewOption("Ref: research, background or the record of an exchange, never normative", string(repo.TypeRef)))
		}
		fields = append(fields, huh.NewSelect[string]().Title("Which kind of document?").Options(options...).Value(&kind))
	}
	if title == "" {
		fields = append(fields, huh.NewInput().Title("Title").Value(&title).Validate(checkTitle))
	}
	if backfill.On {
		fields = append(fields, backfillFields(cmd, backfill)...)
	}
	if len(fields) == 0 {
		return kind, title, nil
	}
	groups := []*huh.Group{huh.NewGroup(fields...)}
	if offerAsOf {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().Title("As of").
				Description("For a record of an exchange, the date it took place, YYYY-MM-DD. Leave empty otherwise.").
				Value(asOf).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					return checkAsOf(s, time.Now())
				}),
		).WithHideFunc(func() bool { return kind != string(repo.TypeRef) }))
	}
	if err := runForm(huh.NewForm(groups...)); err != nil {
		return "", "", err
	}
	return kind, title, nil
}

// backfillFields offers the status and dates of a backfilled document that the
// command line did not give. Each is written to the front matter, so the
// interaction rule covers them. decided is pre-filled with --created where that
// was given; in the same form it cannot follow a created date being typed, so
// it is left empty to mean the same date, and shows that date as it is typed.
// Each answer is judged with the rest as it is given.
func backfillFields(cmd *cobra.Command, opts *backfillOptions) []huh.Field {
	var fields []huh.Field
	if !cmd.Flags().Changed("status") {
		opts.Status = string(repo.StatusAccepted)
		fields = append(fields, huh.NewSelect[string]().Title("Status").
			Options(
				huh.NewOption("Accepted", string(repo.StatusAccepted)),
				huh.NewOption("Rejected", string(repo.StatusRejected)),
				huh.NewOption("Withdrawn", string(repo.StatusWithdrawn)),
			).Value(&opts.Status))
	}
	if !cmd.Flags().Changed("created") {
		fields = append(fields, huh.NewInput().Title("Created").Description("The date the work began, YYYY-MM-DD.").
			Value(&opts.Created).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("the date the work began is required")
				}
				judged := *opts
				judged.Created = s
				return checkBackfill(judged, time.Now())
			}))
	}
	if !cmd.Flags().Changed("decided") {
		opts.Decided = opts.Created
		fields = append(fields, huh.NewInput().Title("Decided").Description("The date the decision was taken, YYYY-MM-DD.").
			PlaceholderFunc(func() string {
				if opts.Created == "" {
					return "the created date"
				}
				return opts.Created
			}, &opts.Created).
			Value(&opts.Decided).
			Validate(func(s string) error {
				judged := *opts
				judged.Decided = s
				return checkBackfill(judged, time.Now())
			}))
	}
	return fields
}

// askForDocument chooses among the documents a transition is available for,
// which is the filtered choice the interaction rule asks for.
func askForDocument(cmd *cobra.Command, args []string, eligible []*repo.Document, action string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if !interactive(cmd) {
		return "", fmt.Errorf("%s needs an identifier: archdoc %s <id>", action, action)
	}
	if len(eligible) == 0 {
		return "", fmt.Errorf("no document is eligible for %s", action)
	}

	options := make([]huh.Option[string], len(eligible))
	for i, d := range eligible {
		options[i] = huh.NewOption(
			fmt.Sprintf("%s  %s  (%s)", d.ID, d.FrontMatter.Title, d.FrontMatter.Status), d.ID)
	}
	var chosen string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Which document?").Options(options...).Value(&chosen),
	)).Run()
	return chosen, err
}

// askForTerm fills in a glossary term and, when adding, its definition. Given
// namedBy, which only adding does, it also offers the numbered documents for
// --named-by unless the flag was given, with none as the first choice.
func askForTerm(cmd *cobra.Command, args []string, needDefinition bool, action string, namedBy *string, documents []*repo.Document) (term, definition string, err error) {
	if len(args) > 0 {
		term = args[0]
	}
	if len(args) > 1 {
		definition = args[1]
	}
	complete := term != "" && (!needDefinition || definition != "")
	if !interactive(cmd) {
		if complete {
			return term, definition, nil
		}
		return "", "", fmt.Errorf("term %s needs its arguments", action)
	}
	askNamedBy := namedBy != nil && !cmd.Flags().Changed("named-by") && len(documents) > 0
	if complete && !askNamedBy {
		return term, definition, nil
	}

	var fields []huh.Field
	if term == "" {
		fields = append(fields, huh.NewInput().Title("Term").Value(&term))
	}
	if needDefinition && definition == "" {
		fields = append(fields, huh.NewText().Title("Definition").Value(&definition))
	}
	if askNamedBy {
		options := []huh.Option[string]{huh.NewOption("None", "")}
		for _, d := range documents {
			options = append(options, huh.NewOption(fmt.Sprintf("%s  %s", d.ID, d.FrontMatter.Title), d.ID))
		}
		fields = append(fields, huh.NewSelect[string]().Title("Which document introduced this term?").
			Options(options...).Value(namedBy))
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return "", "", err
	}
	return term, definition, nil
}

// askForRename fills in the term being renamed and its new name.
func askForRename(cmd *cobra.Command, args []string, terms []string) (from, to string, err error) {
	if len(args) > 0 {
		from = args[0]
	}
	if len(args) > 1 {
		to = args[1]
	}
	if from != "" && to != "" {
		return from, to, nil
	}
	if !interactive(cmd) {
		return "", "", fmt.Errorf("term rename needs both names: archdoc term rename <old> <new>")
	}
	if len(terms) == 0 {
		return "", "", fmt.Errorf("the glossary has no terms to rename")
	}

	var fields []huh.Field
	if from == "" {
		options := make([]huh.Option[string], len(terms))
		for i, term := range terms {
			options[i] = huh.NewOption(term, term)
		}
		fields = append(fields, huh.NewSelect[string]().Title("Which term?").Options(options...).Value(&from))
	}
	if to == "" {
		fields = append(fields, huh.NewInput().Title("New name").Value(&to))
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return "", "", err
	}
	return from, to, nil
}

// askForScaffold confirms the values init records in archdoc.json. They are
// optional flags, but every one of them is written to a file, so the
// interaction rule offers them.
func askForScaffold(cmd *cobra.Command, settings *config.Config, licence *string, agents *bool) error {
	if !interactive(cmd) {
		return nil
	}
	refStaleDays := strconv.Itoa(settings.RefStaleDays)

	// A field is offered only when the command line did not settle it.
	// Prompting substitutes for values the command was not given and for
	// nothing else, and a fully specified invocation from a terminal, which is
	// what a Makefile target or a wrapper script produces, used to render the
	// whole form and block until someone answered it.
	var fields []huh.Field
	unless := func(flag string, field huh.Field) {
		if !cmd.Flags().Changed(flag) {
			fields = append(fields, field)
		}
	}

	unless("name", huh.NewInput().Title("Project name").Value(&settings.Name).
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("a name is required")
			}
			return nil
		}))
	unless("branch", huh.NewInput().Title("Branch frozen documents are compared against").Value(&settings.Branch))
	unless("root", huh.NewInput().Title("Directory holding the document directories").Value(&settings.Root))
	if !cmd.Flags().Changed("strict") && !cmd.Flags().Changed("no-strict") {
		fields = append(fields, huh.NewConfirm().Title("Keep the strict transition graph?").
			Description("Off also permits draft to accepted and draft to rejected.").
			Value(&settings.Strict))
	}
	unless("ref-stale-days", huh.NewInput().Title("Days before a ref's verified date is stale").Value(&refStaleDays).
		Validate(func(s string) error {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				return fmt.Errorf("a number of days, or 0 to disable the check")
			}
			return nil
		}))
	// A licence is written to a file, so the interaction rule covers it.
	unless("license", huh.NewSelect[string]().Title("Licence for the new repository").
		Options(
			huh.NewOption("None", "none"),
			huh.NewOption("MIT", "mit"),
		).Value(licence))

	// Also a file, so the same rule covers it.
	unless("agents", huh.NewConfirm().Title("Write guidance for coding agents?").
		Description("AGENTS.md and agents/, describing the process to an agent working here.").
		Value(agents))

	if len(fields) == 0 {
		return nil
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return err
	}
	settings.RefStaleDays, _ = strconv.Atoi(refStaleDays)
	return nil
}

// askForIncludes offers the accepted documents a glossary edit may cite. It is
// an optional flag, so it is offered rather than required: an empty selection
// is a complete answer.
func askForIncludes(cmd *cobra.Command, given []string, accepted []*repo.Document) ([]string, error) {
	if len(given) > 0 || !interactive(cmd) || len(accepted) == 0 {
		return given, nil
	}
	options := make([]huh.Option[string], len(accepted))
	for i, d := range accepted {
		options[i] = huh.NewOption(fmt.Sprintf("%s  %s", d.ID, d.FrontMatter.Title), d.ID)
	}
	var chosen []string
	err := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Which accepted documents does this term come from?").
			Description("Optional; leave empty to record none.").
			Options(options...).Value(&chosen),
	)).Run()
	return chosen, err
}

// askForRenumber chooses the document to renumber among the numbered ones that
// are not frozen. The choice is a path, because the case this command exists
// for is two documents carrying one identifier.
func askForRenumber(cmd *cobra.Command, r *repo.Repo, frozen func(*repo.Document) bool) (string, error) {
	if !interactive(cmd) {
		return "", fmt.Errorf("renumber needs a document: archdoc renumber <id|path> [new-id]")
	}
	var options []huh.Option[string]
	for _, d := range r.Documents() {
		if d.Type.Numbered() && !frozen(d) {
			options = append(options, huh.NewOption(fmt.Sprintf("%s  %s  (%s)", d.ID, d.FrontMatter.Title, d.Path), d.Path))
		}
	}
	if len(options) == 0 {
		return "", fmt.Errorf("no document can be renumbered")
	}
	var chosen string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Which document?").Options(options...).Value(&chosen),
	)).Run()
	return chosen, err
}
