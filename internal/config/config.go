package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// File represents on-disk config (YAML or JSON).
type File struct {
	BaseURL   string `yaml:"base_url" json:"base_url"`
	AccountID string `yaml:"account_id" json:"account_id"`
	Token     string `yaml:"token" json:"token"`
	APIKey    string `yaml:"api_key" json:"api_key"`
	Output    string `yaml:"output" json:"output"` // json | pretty; optional default when --output not passed
}

// Input is flag/env-driven input before merge.
type Input struct {
	BaseURL    string
	AccountID  string
	Token      string
	APIKey     string
	ConfigPath string
}

// Resolved is the merged configuration.
type Resolved struct {
	BaseURL    string
	AccountID  string
	Token      string
	APIKey     string
	Output     string // from config file only; applied when --output flag unchanged
	ConfigPath string // path used when loading file (for config subcommands)
}

// DefaultConfigPath returns XDG path or ~/.config/sse/config.yaml (ignores SSE_CONFIG; use effectiveConfigPath for full resolution).
func DefaultConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "sse")
	return filepath.Join(dir, "config.yaml"), nil
}

// Resolve merges defaults < file < environment < flags (last wins).
func Resolve(in Input) (Resolved, error) {
	path, err := effectiveConfigPath(in.ConfigPath)
	if err != nil {
		return Resolved{}, err
	}
	fileVals := File{}
	if data, err := os.ReadFile(path); err == nil {
		if err := unmarshalConfig(data, &fileVals); err != nil {
			return Resolved{}, fmt.Errorf("parse config %q: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return Resolved{}, fmt.Errorf("read config %q: %w", path, err)
	}

	r := Resolved{ConfigPath: path}
	r.BaseURL = NormalizeScriptAPIBaseURL(firstNonEmpty(in.BaseURL, os.Getenv("SSE_BASE_URL"), fileVals.BaseURL))
	r.AccountID = firstNonEmpty(in.AccountID, os.Getenv("SSE_ACCOUNT_ID"), fileVals.AccountID)
	r.Token = firstNonEmpty(in.Token, os.Getenv("SSE_TOKEN"), fileVals.Token)
	r.APIKey = firstNonEmpty(in.APIKey, os.Getenv("SSE_API_KEY"), fileVals.APIKey)
	r.Output = strings.TrimSpace(fileVals.Output)

	if r.Token != "" && r.APIKey != "" {
		return Resolved{}, errors.New("set only one of bearer token (--token / SSE_TOKEN) or API key (--api-key / SSE_API_KEY)")
	}
	return r, nil
}

func effectiveConfigPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if env := os.Getenv("SSE_CONFIG"); env != "" {
		return env, nil
	}
	return DefaultConfigPath()
}

func unmarshalConfig(data []byte, dst *File) error {
	trim := strings.TrimSpace(string(data))
	if trim == "" {
		return nil
	}
	if trim[0] == '{' {
		return json.Unmarshal(data, dst)
	}
	return yaml.Unmarshal(data, dst)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ValidateForAPI ensures settings required for HTTP calls are present.
func (r Resolved) ValidateForAPI() error {
	if r.BaseURL == "" {
		return errors.New("base URL is required (--base-url or SSE_BASE_URL or config base_url)")
	}
	if r.AccountID == "" {
		return errors.New("account id is required (--account-id or SSE_ACCOUNT_ID or config account_id)")
	}
	if r.Token == "" && r.APIKey == "" {
		return errors.New("authentication required: set --token or --api-key (or SSE_TOKEN / SSE_API_KEY / config); sent as Authorization per docs/openapi.yaml")
	}
	return nil
}

// NormalizeBaseURL trims trailing slashes.
func NormalizeBaseURL(base string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/")
}

// NormalizeScriptAPIBaseURL trims the base URL and fixes a common mistake: using
// https://host/.../scripting without the /v1 segment hits the web app (HTML), not the
// Script Management API. Per docs/openapi.yaml, servers use .../v1/scripting.
func NormalizeScriptAPIBaseURL(base string) string {
	base = NormalizeBaseURL(base)
	if base == "" {
		return base
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return base
	}
	path := strings.Trim(u.Path, "/")
	// Only rewrite a single path segment "scripting" -> "v1/scripting".
	if path == "scripting" {
		u.Path = "/v1/scripting"
		return strings.TrimRight(u.String(), "/")
	}
	return base
}

// WriteFile writes config to path with mode 0600. Only non-empty fields are written.
func WriteFile(path string, f File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out := File{}
	if f.BaseURL != "" {
		out.BaseURL = f.BaseURL
	}
	if f.AccountID != "" {
		out.AccountID = f.AccountID
	}
	if f.Token != "" {
		out.Token = f.Token
	}
	if f.APIKey != "" {
		out.APIKey = f.APIKey
	}
	if f.Output != "" {
		out.Output = f.Output
	}
	data, err := yaml.Marshal(&out)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ReadFile loads a config file from path (ignores merge).
func ReadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := unmarshalConfig(data, &f); err != nil {
		return File{}, err
	}
	return f, nil
}
