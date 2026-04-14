package cmd

import (
	"github.com/spf13/cobra"

	"sse-cli/internal/api"
)

func runtimesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "runtimes",
		Short: "List runtimes and bundled libraries",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			_, err := MustGlobal()
			return err
		},
	}
	c.AddCommand(runtimesListCmd(), runtimesLibrariesCmd())
	return c
}

func runtimesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			raw, err := cl.ListRuntimes()
			if err != nil {
				return err
			}
			return PrintJSON(raw)
		},
	}
}

func runtimesLibrariesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "libraries <runtime>",
		Short: "List pre-installed libraries for a runtime",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			raw, err := cl.ListRuntimeLibraries(args[0])
			if err != nil {
				return err
			}
			return PrintJSON(raw)
		},
	}
}
