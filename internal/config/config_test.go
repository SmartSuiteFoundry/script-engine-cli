package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	if got := NormalizeBaseURL("https://x.com/stage/"); got != "https://x.com/stage" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeScriptAPIBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://hotfix.ss-stage.com/scripting", "https://hotfix.ss-stage.com/v1/scripting"},
		{"https://hotfix.ss-stage.com/scripting/", "https://hotfix.ss-stage.com/v1/scripting"},
		{"https://hotfix.ss-stage.com/v1/scripting", "https://hotfix.ss-stage.com/v1/scripting"},
		{"https://hotfix.ss-stage.com/v1/scripting/", "https://hotfix.ss-stage.com/v1/scripting"},
		{"https://hotfix.ss-stage.com/api/scripting", "https://hotfix.ss-stage.com/api/scripting"},
	}
	for _, tc := range cases {
		if got := NormalizeScriptAPIBaseURL(tc.in); got != tc.want {
			t.Fatalf("NormalizeScriptAPIBaseURL(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolve_outputFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("base_url: https://x/v1/scripting\naccount_id: a\ntoken: t\noutput: json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(Input{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if r.Output != "json" {
		t.Fatalf("output: %q", r.Output)
	}
}

func TestResolve_precedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("base_url: https://from-file\naccount_id: f1\ntoken: t1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSE_BASE_URL", "https://from-env")
	t.Setenv("SSE_ACCOUNT_ID", "e1")
	t.Setenv("SSE_TOKEN", "envtok")
	t.Setenv("SSE_API_KEY", "")
	r, err := Resolve(Input{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if r.BaseURL != "https://from-env" {
		t.Fatalf("base_url: got %q", r.BaseURL)
	}
	if r.AccountID != "e1" {
		t.Fatalf("account_id: got %q", r.AccountID)
	}
	if r.Token != "envtok" {
		t.Fatalf("token: got %q", r.Token)
	}
	// flags win over env
	r2, err := Resolve(Input{ConfigPath: path, BaseURL: "https://from-flag"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.BaseURL != "https://from-flag" {
		t.Fatalf("flag: got %q", r2.BaseURL)
	}
}
