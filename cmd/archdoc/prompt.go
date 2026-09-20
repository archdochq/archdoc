package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/archdochq/archdoc/internal/config"
	"github.com/archdochq/archdoc/internal/repo"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// Prompting substitutes for values the command was not given, and for nothing
// else. When stdin is not a terminal, or --no-interaction was passed, a missing
// required value is a usage error instead.

// askForNew fills in the type and title of a new document.
func askForNew(cmd *cobra.Command, args []string) (kind, title string, err error) {
	if len(args) > 0 {
		kind = args[0]
	}
	if len(args) > 1 {
		title = args[1]
	}
	if kind != "" && title != "" {
		return kind, title, nil
	}
	if !interactive(cmd) {
		return "", "", fmt.Errorf("new needs a type and a title: archdoc new <rfc|adr|ref> <title>")
	}

	var fields []huh.Field
	if kind == "" {
		fields = append(fields, huh.NewSelect[string]().
			Title("Which kind of document?").
			Options(
				huh.NewOption("RFC: a design the spec will have a page for", string(repo.TypeRFC)),
				huh.NewOption("ADR: a decision that constrains designs", string(repo.TypeADR)),
				huh.NewOption("Ref: research or background, never normative", string(repo.TypeRef)),
			).
			Value(&kind))
	}
	if title == "" {
		fields = append(fields, huh.NewInput().Title("Title").Value(&title).
			Validate(func(s string) error {
				if repo.Slug(s) == "" {
					return fmt.Errorf("a title needs something that can go in a filename")
				}
				return nil
			}))
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return "", "", err
	}
	return kind, title, nil
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

// askForTerm fills in a glossary term and, when adding, its definition.
func askForTerm(cmd *cobra.Command, args []string, needDefinition bool, action string) (term, definition string, err error) {
	if len(args) > 0 {
		term = args[0]
	}
	if len(args) > 1 {
		definition = args[1]
	}
	if term != "" && (!needDefinition || definition != "") {
		return term, definition, nil
	}
	if !interactive(cmd) {
		return "", "", fmt.Errorf("term %s needs its arguments", action)
	}

	var fields []huh.Field
	if term == "" {
		fields = append(fields, huh.NewInput().Title("Term").Value(&term))
	}
	if needDefinition && definition == "" {
		fields = append(fields, huh.NewText().Title("Definition").Value(&definition))
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
