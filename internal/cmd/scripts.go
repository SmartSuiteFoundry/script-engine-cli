package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"sse-cli/internal/api"
)

func scriptsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "scripts",
		Short: "Manage scripts (CRUD + execute)",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			_, err := MustGlobal()
			return err
		},
	}
	c.AddCommand(scriptListCmd(), scriptGetCmd(), scriptCreateCmd(), scriptUpdateCmd(), scriptDeleteCmd(), scriptExecuteCmd())
	return c
}

func scriptListCmd() *cobra.Command {
	var cursor string
	var pageSize int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List scripts (cursor pagination)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			raw, err := cl.ListScripts(cursor, pageSize)
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

func scriptGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a script by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			raw, err := cl.GetScript(args[0])
			if err != nil {
				return err
			}
			return PrintJSON(raw)
		},
	}
}

func scriptCreateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a script (JSON body via --file or stdin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := ReadJSONPayload(file, nil)
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(body)) == "" {
				return fmt.Errorf("empty body: provide --file or JSON on stdin")
			}
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			raw, err := cl.CreateScript(body)
			if err != nil {
				return err
			}
			return PrintJSON(raw)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "JSON body file path (use - for stdin)")
	return cmd
}

func scriptUpdateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a script (JSON body via --file or stdin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := ReadJSONPayload(file, nil)
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(body)) == "" {
				return fmt.Errorf("empty body: provide --file or JSON on stdin")
			}
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			raw, err := cl.UpdateScript(args[0], body)
			if err != nil {
				return err
			}
			return PrintJSON(raw)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "JSON body file path (use - for stdin)")
	return cmd
}

func scriptDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			if err := cl.DeleteScript(args[0]); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "deleted")
			return nil
		},
	}
}

func scriptExecuteCmd() *cobra.Command {
	var file string
	var mode string
	var triggerType string
	var callerIP string
	cmd := &cobra.Command{
		Use:   "execute <id>",
		Short: "Execute a script (ExecuteRequest per docs/openapi.yaml)",
		Long: "Builds a JSON body with required fields mode and trigger_type. " +
			"Use -f to supply a partial or full ExecuteRequest; flags fill any missing required keys.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := ReadJSONPayload(file, []byte("{}"))
			if err != nil {
				return err
			}
			body, err := mergeExecuteRequest(raw, mode, triggerType, callerIP)
			if err != nil {
				return err
			}
			cfg, err := MustGlobal()
			if err != nil {
				return err
			}
			cl := api.New(cfg)
			st, out, err := cl.ExecuteScript(args[0], body)
			if err != nil {
				return err
			}
			switch st {
			case 200:
				fmt.Fprintln(os.Stderr, "execution: synchronous (200 OK)")
			case 202:
				fmt.Fprintln(os.Stderr, "execution: async accepted (202 Accepted)")
			default:
				fmt.Fprintf(os.Stderr, "execution: HTTP %d\n", st)
			}
			if len(out) == 0 {
				return nil
			}
			return PrintJSON(out)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "JSON file or stdin (-); merged with mode/trigger_type defaults")
	cmd.Flags().StringVar(&mode, "mode", "sync", `execution mode: "sync" or "async"`)
	cmd.Flags().StringVar(&triggerType, "trigger-type", "manual", `trigger_type: "http", "scheduled", or "manual"`)
	cmd.Flags().StringVar(&callerIP, "caller-ip", "", "optional caller_ip (sent only if non-empty)")
	return cmd
}

func mergeExecuteRequest(user []byte, mode, triggerType, callerIP string) ([]byte, error) {
	var m map[string]any
	if strings.TrimSpace(string(user)) == "" {
		m = map[string]any{}
	} else {
		if err := json.Unmarshal(user, &m); err != nil {
			return nil, fmt.Errorf("payload must be a JSON object: %w", err)
		}
	}
	if _, ok := m["mode"]; !ok {
		if mode == "" {
			m["mode"] = "sync"
		} else {
			m["mode"] = mode
		}
	}
	if _, ok := m["trigger_type"]; !ok {
		if triggerType == "" {
			m["trigger_type"] = "manual"
		} else {
			m["trigger_type"] = triggerType
		}
	}
	if callerIP != "" {
		if _, ok := m["caller_ip"]; !ok {
			m["caller_ip"] = callerIP
		}
	}
	return json.Marshal(m)
}
