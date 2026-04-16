package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"sse-cli/internal/config"
)

func configCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Inspect or update local CLI configuration file",
	}
	c.AddCommand(configPathCmd(), configGetCmd(), configSetCmd())
	return c
}

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved configuration file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := Global.ConfigPath
			if p == "" {
				var err error
				p, err = config.DefaultConfigPath()
				if err != nil {
					return err
				}
			}
			fmt.Println(p)
			return nil
		},
	}
}

func configGetCmd() *cobra.Command {
	var showSecrets bool
	cmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Print config values (keys: base_url, account_id, token, api_key, output)",
		Long:  "Without key, prints all entries. Secrets are masked unless --show-secrets. Credentials may be loaded from the OS secret store (see sse configure / sse config set) rather than the file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := Global.ConfigPath
			_, statErr := os.Stat(path)
			f, err := config.ReadMerged(path)
			if err != nil {
				return err
			}
			if os.IsNotExist(statErr) && !configSnapshotNonEmpty(f) {
				fmt.Fprintf(os.Stderr, "no config file at %s\n", path)
				return nil
			}
			if len(args) == 0 {
				printConfigFile(f, showSecrets)
				return nil
			}
			return printOneKey(args[0], f, showSecrets)
		},
	}
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "print token and api_key in full")
	return cmd
}

func configSnapshotNonEmpty(f config.File) bool {
	return strings.TrimSpace(f.BaseURL) != "" ||
		strings.TrimSpace(f.AccountID) != "" ||
		strings.TrimSpace(f.Token) != "" ||
		strings.TrimSpace(f.APIKey) != "" ||
		strings.TrimSpace(f.Output) != ""
}

func printConfigFile(f config.File, showSecrets bool) {
	printKV("base_url", f.BaseURL, false)
	printKV("account_id", f.AccountID, false)
	printKV("token", f.Token, !showSecrets)
	printKV("api_key", f.APIKey, !showSecrets)
	printKV("output", f.Output, false)
}

func printKV(key, val string, mask bool) {
	if val == "" {
		return
	}
	if mask {
		fmt.Printf("%s: (set)\n", key)
		return
	}
	fmt.Printf("%s: %s\n", key, val)
}

func printOneKey(key string, f config.File, showSecrets bool) error {
	switch key {
	case "base_url":
		fmt.Println(f.BaseURL)
	case "account_id":
		fmt.Println(f.AccountID)
	case "token":
		if !showSecrets && f.Token != "" {
			fmt.Println("(set)")
		} else {
			fmt.Println(f.Token)
		}
	case "api_key":
		if !showSecrets && f.APIKey != "" {
			fmt.Println("(set)")
		} else {
			fmt.Println(f.APIKey)
		}
	case "output":
		fmt.Println(f.Output)
	default:
		return fmt.Errorf("unknown key %q (use base_url, account_id, token, api_key, output)", key)
	}
	return nil
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value and save (file mode 0600; secrets prefer OS secret store)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := Global.ConfigPath
			f := config.File{}
			if existing, err := config.ReadMerged(path); err == nil {
				f = existing
			} else if !os.IsNotExist(err) {
				return err
			}
			switch args[0] {
			case "base_url":
				f.BaseURL = args[1]
			case "account_id":
				f.AccountID = args[1]
			case "token":
				f.Token = args[1]
				f.APIKey = ""
			case "api_key":
				f.APIKey = args[1]
				f.Token = ""
			case "output":
				v := strings.ToLower(strings.TrimSpace(args[1]))
				if v != "json" && v != "pretty" {
					return fmt.Errorf("output must be json or pretty")
				}
				f.Output = v
			default:
				return fmt.Errorf("unknown key %q", args[0])
			}
			if f.Token != "" && f.APIKey != "" {
				return fmt.Errorf("config cannot hold both token and api_key")
			}
			if err := config.WriteFile(path, f); err != nil {
				if errors.Is(err, config.ErrCredentialStoreFallback) {
					fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
					return nil
				}
				return err
			}
			return nil
		},
	}
}
