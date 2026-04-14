package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sse-cli/internal/api"
)

func runsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "runs",
		Short: "List runs, get run details, fetch logs",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			_, err := MustGlobal()
			return err
		},
	}
	c.AddCommand(runsListCmd(), runsGetCmd(), runsLogsCmd())
	return c
}

func runsListCmd() *cobra.Command {
	var cursor string
	var pageSize int
	cmd := &cobra.Command{
		Use:   "list <scriptId>",
		Short: "List runs for a script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			raw, err := cl.ListRuns(args[0], cursor, pageSize)
			if err != nil {
				return err
			}
			return PrintJSON(raw)
		},
	}
	cmd.Flags().StringVar(&cursor, "cursor", "", "opaque pagination cursor (next_cursor from prior page)")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "items per page (1–100; omit for API default)")
	return cmd
}

func runsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <scriptId> <runId>",
		Short: "Get a single run",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			raw, err := cl.GetRun(args[0], args[1])
			if err != nil {
				return err
			}
			return PrintJSON(raw)
		},
	}
}

func runsLogsCmd() *cobra.Command {
	var urlOnly bool
	var writePath string
	cmd := &cobra.Command{
		Use:   "logs <scriptId> <runId>",
		Short: "Fetch run logs (resolves presigned URL from API)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			meta, err := cl.GetRunLogsMetadata(args[0], args[1])
			if err != nil {
				return err
			}
			logURL, err := api.ExtractLogURL(meta)
			if err != nil {
				return err
			}
			if urlOnly {
				fmt.Println(logURL)
				return nil
			}
			logBytes, err := api.FetchURL(cl.HTTP(), logURL)
			if err != nil {
				return err
			}
			if writePath != "" {
				return os.WriteFile(writePath, logBytes, 0o644)
			}
			return PrintRaw(logBytes)
		},
	}
	cmd.Flags().BoolVar(&urlOnly, "url-only", false, "print presigned URL only, do not download")
	cmd.Flags().StringVarP(&writePath, "write", "w", "", "write log bytes to file instead of stdout")
	return cmd
}
