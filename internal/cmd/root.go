package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"sse-cli/internal/config"
)

// Version is set by main via ldflags.
var Version = "dev"

// Global holds CLI-wide options after PersistentPreRun.
var Global config.Resolved

var (
	flagBaseURL    string
	flagAccountID  string
	flagToken      string
	flagAPIKey     string
	flagConfigPath string
	flagOutput     string
)

var rootCmd = &cobra.Command{
	Use:           "sse",
	Short:         "SmartSuite Script Engine CLI",
	Long:          "Interact with the SmartSuite Script Management API (scripts, execution, runs, logs).",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		resolved, err := config.Resolve(config.Input{
			BaseURL:    flagBaseURL,
			AccountID:  flagAccountID,
			Token:      flagToken,
			APIKey:     flagAPIKey,
			ConfigPath: flagConfigPath,
		})
		if err != nil {
			return err
		}
		Global = resolved
		// Default output from config file when the user did not pass --output on the CLI.
		if pf := cmd.Root().PersistentFlags().Lookup("output"); pf != nil && !pf.Changed && strings.TrimSpace(Global.Output) != "" {
			flagOutput = Global.Output
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
	Version: Version,
}

// Execute runs the root command.
func Execute() error {
	// rootCmd.Version was copied at package init; refresh after main() sets Version via ldflags.
	rootCmd.Version = Version
	return rootCmd.Execute()
}

func init() {
	// Without this, Cobra only runs the first ancestor PersistentPreRun (e.g. `scripts`),
	// not `sse`, so global flags are never merged into config before MustGlobal().
	cobra.EnableTraverseRunHooks = true

	rootCmd.PersistentFlags().StringVar(&flagBaseURL, "base-url", "", "API base URL (env SSE_BASE_URL)")
	rootCmd.PersistentFlags().StringVar(&flagAccountID, "account-id", "", "Workspace Account-Id header (env SSE_ACCOUNT_ID)")
	rootCmd.PersistentFlags().StringVar(&flagToken, "token", "", "API credential (env SSE_TOKEN): sent as Authorization ApiKey <value> unless value already starts with Bearer or ApiKey (see docs/openapi.yaml)")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "Same as --token (env SSE_API_KEY); mutually exclusive; use one or the other")
	rootCmd.PersistentFlags().StringVar(&flagConfigPath, "config", "", "Config file path (env SSE_CONFIG)")
	rootCmd.PersistentFlags().StringVar(&flagOutput, "output", "pretty", "Output format: json | pretty")

	rootCmd.AddCommand(scriptsCmd())
	rootCmd.AddCommand(runsCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(runtimesCmd())
	rootCmd.AddCommand(configureCmd())
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

// MustGlobal returns merged config after validating API requirements.
func MustGlobal() (config.Resolved, error) {
	if err := Global.ValidateForAPI(); err != nil {
		return config.Resolved{}, err
	}
	return Global, nil
}

// OutputFormat returns json or pretty.
func OutputFormat() string {
	if flagOutput == "json" {
		return "json"
	}
	return "pretty"
}
