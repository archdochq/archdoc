package main

import (
	"encoding/json"

	"archdoc.dev/internal/reference"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newReferenceCommand() *cobra.Command {
	var schema bool

	cmd := &cobra.Command{
		Use:   "reference",
		Short: "Print the commands, rules, front matter, configuration and exit codes as JSON",
		Args:  cobra.NoArgs,
		// Hidden, which also keeps it out of completion: it is for the tools
		// that document a release, not for working in a repository.
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if schema {
				_, err := out.Write(reference.JSONSchema)
				return err
			}
			// Cobra adds --version only when the root command runs, and it is
			// the root's to describe whichever command is running.
			root := cmd.Root()
			root.InitDefaultVersionFlag()
			ref, err := reference.Build(resolveVersion(), describe(root), exitCodes)
			if err != nil {
				return err
			}
			encoded, err := json.MarshalIndent(ref, "", "  ")
			if err != nil {
				return err
			}
			_, err = out.Write(append(encoded, '\n'))
			return err
		},
	}
	cmd.Flags().BoolVar(&schema, "schema", false, "print the JSON Schema the output conforms to and exit")
	return cmd
}

// describe walks a command and its subcommands into the reference's types.
func describe(c *cobra.Command) reference.Command {
	d := reference.Command{
		Path:        c.CommandPath(),
		Use:         c.Use,
		Short:       c.Short,
		Long:        c.Long,
		Flags:       flags(c.LocalNonPersistentFlags()),
		GlobalFlags: flags(c.PersistentFlags()),
		Commands:    []reference.Command{},
	}
	for _, sub := range c.Commands() {
		if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}
		d.Commands = append(d.Commands, describe(sub))
	}
	return d
}

// flags lists a flag set in the order help prints it. --help is not among
// them: cobra adds it only to the command running, which is this hidden one.
func flags(set *pflag.FlagSet) []reference.Flag {
	out := []reference.Flag{}
	set.VisitAll(func(f *pflag.Flag) {
		out = append(out, reference.Flag{
			Name: f.Name, Shorthand: f.Shorthand, Type: f.Value.Type(), Default: f.DefValue, Usage: f.Usage,
		})
	})
	return out
}
