package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"sse-cli/internal/config"
)

func configureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Interactively set API base URL, account, credentials, and default output (writes config file)",
		Long: `Walks through prompts similar to "aws configure": each question shows the current
value in brackets; press Enter to keep it. API keys and tokens are read with hidden input
when the terminal supports it.`,
		RunE: runConfigure,
	}
}

func runConfigure(cmd *cobra.Command, args []string) error {
	if os.Getenv("CI") == "true" {
		return fmt.Errorf("sse configure is interactive only; use 'sse config set' in CI, or unset CI=true")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return fmt.Errorf("sse configure needs an interactive terminal (stdin and stderr must be TTYs); use 'sse config set' instead")
	}

	path := Global.ConfigPath
	if path == "" {
		var err error
		path, err = config.DefaultConfigPath()
		if err != nil {
			return err
		}
	}

	f, err := config.ReadMerged(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintln(os.Stderr, "SmartSuite Script Engine CLI — configuration")
	fmt.Fprintln(os.Stderr, "Press Enter at any prompt to keep the current value shown in brackets.")
	fmt.Fprintln(os.Stderr)

	pick := func(a, b string) string {
		if strings.TrimSpace(a) != "" {
			return strings.TrimSpace(a)
		}
		return strings.TrimSpace(b)
	}

	base, err := promptString(os.Stdin, os.Stderr, "API base URL (include /v1/scripting)", pick(Global.BaseURL, f.BaseURL))
	if err != nil {
		return err
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return fmt.Errorf("base URL is required")
	}
	f.BaseURL = config.NormalizeScriptAPIBaseURL(base)

	acct, err := promptString(os.Stdin, os.Stderr, "Account-Id (workspace id)", pick(Global.AccountID, f.AccountID))
	if err != nil {
		return err
	}
	acct = strings.TrimSpace(acct)
	if acct == "" {
		return fmt.Errorf("Account-Id is required")
	}
	f.AccountID = acct

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "How should sse authenticate?")
	fmt.Fprintln(os.Stderr, "  1) API key — secret value only (recommended; sent as Authorization: ApiKey …)")
	fmt.Fprintln(os.Stderr, "  2) Token — full value, or a string starting with Bearer or ApiKey")

	authDefault := "1"
	if strings.TrimSpace(f.Token) != "" {
		authDefault = "2"
	}

	choice, err := promptString(os.Stdin, os.Stderr, "Enter 1 or 2", authDefault)
	if err != nil {
		return err
	}
	switch strings.TrimSpace(choice) {
	case "", "1":
		tok, key, err := promptCredentialSwap(true, f.Token, f.APIKey)
		if err != nil {
			return err
		}
		f.Token, f.APIKey = tok, key
	case "2":
		tok, key, err := promptCredentialSwap(false, f.Token, f.APIKey)
		if err != nil {
			return err
		}
		f.Token, f.APIKey = tok, key
	default:
		return fmt.Errorf("enter 1 or 2")
	}

	if f.Token != "" && f.APIKey != "" {
		return fmt.Errorf("internal error: both token and api_key set")
	}

	fmt.Fprintln(os.Stderr)
	outDefault := pick(Global.Output, f.Output)
	if outDefault == "" {
		outDefault = flagOutput
	}
	out, err := promptString(os.Stdin, os.Stderr, "Default output format (json or pretty)", outDefault)
	if err != nil {
		return err
	}
	out = strings.TrimSpace(strings.ToLower(out))
	if out == "" {
		out = outDefault
	}
	if out != "json" && out != "pretty" {
		return fmt.Errorf("output must be json or pretty")
	}
	f.Output = out

	if err := config.WriteFile(path, f); err != nil {
		if errors.Is(err, config.ErrCredentialStoreFallback) {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		} else {
			return err
		}
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Wrote configuration to %s\n", path)
	fmt.Fprintln(os.Stderr, "Try: sse scripts list")
	return nil
}

func promptString(in *os.File, out *os.File, label, current string) (string, error) {
	show := current
	if show == "" {
		show = "none"
	}
	fmt.Fprintf(out, "%s [%s]: ", label, show)
	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return "", err
		}
		return current, nil
	}
	line := strings.TrimSpace(sc.Text())
	if line == "" {
		return current, nil
	}
	return line, nil
}

// promptCredentialSwap asks for a new secret; empty Enter keeps existing. isAPIKey chooses which field to set.
func promptCredentialSwap(isAPIKey bool, curToken, curAPI string) (token, apiKey string, err error) {
	var existing string
	if isAPIKey {
		existing = curAPI
	} else {
		existing = curToken
	}
	hasExisting := strings.TrimSpace(existing) != ""

	label := "Token (hidden)"
	if isAPIKey {
		label = "API key (hidden)"
	}
	if hasExisting {
		label += " — Enter to keep current"
	}
	fmt.Fprintf(os.Stderr, "%s\n", label)
	secret, err := readSecretLine()
	if err != nil {
		return "", "", err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		if !hasExisting {
			return "", "", fmt.Errorf("a non-empty secret is required on first configure")
		}
		if isAPIKey {
			return "", curAPI, nil
		}
		return curToken, "", nil
	}
	if isAPIKey {
		return "", secret, nil
	}
	return secret, "", nil
}

func readSecretLine() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		fmt.Fprintln(os.Stderr)
		return string(b), nil
	}
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return "", sc.Err()
	}
	return sc.Text(), nil
}
