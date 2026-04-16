package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadMerged_missingFileIsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.yaml")
	f, err := ReadMerged(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.BaseURL != "" || f.Token != "" {
		t.Fatalf("expected empty, got %+v", f)
	}
}

func TestReadMerged_filePlusKeyring(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(p, []byte("base_url: https://x/v1/scripting\naccount_id: a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveSecretsKeyring(abs, "secret-tok", ""); err != nil {
		t.Skipf("keyring unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = saveSecretsKeyring(abs, "", "")
	})
	f, err := ReadMerged(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.Token != "secret-tok" || f.APIKey != "" {
		t.Fatalf("token=%q api_key=%q", f.Token, f.APIKey)
	}
	if f.BaseURL == "" {
		t.Fatal("expected base_url from file")
	}
}

func TestWriteFile_prefersKeyringOverPlaintext(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.yaml")
	f := File{
		BaseURL:   "https://example.com/v1/scripting",
		AccountID: "acct",
		Token:     "bear-me",
		Output:    "json",
	}
	if err := WriteFile(p, f); err != nil {
		if errors.Is(err, ErrCredentialStoreFallback) {
			t.Skipf("keyring unavailable: %v", err)
		}
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(p)
	t.Cleanup(func() { _ = saveSecretsKeyring(abs, "", "") })
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "bear-me") {
		t.Fatalf("token should not appear in config file:\n%s", raw)
	}
	got, err := ReadMerged(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "bear-me" {
		t.Fatalf("ReadMerged token: %q", got.Token)
	}
}
