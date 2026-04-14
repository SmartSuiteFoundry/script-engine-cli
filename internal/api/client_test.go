package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sse-cli/internal/config"
)

func TestFormatAuthorization(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"secret", "ApiKey secret"},
		{"  tok  ", "ApiKey tok"},
		{"ApiKey abc", "ApiKey abc"},
		{"apikey  xyz", "ApiKey xyz"},
		{"Bearer eyJ", "Bearer eyJ"},
		{"bearer low", "bearer low"},
	}
	for _, tc := range cases {
		if got := formatAuthorization(tc.in); got != tc.want {
			t.Fatalf("formatAuthorization(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestClient_HeadersApiKey(t *testing.T) {
	var gotMethod, gotPath string
	var gotAccount, gotAuth, gotAccept, gotCT, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAccount = r.Header.Get("Account-Id")
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotCT = r.Header.Get("Content-Type")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cl := New(config.Resolved{
		BaseURL:   srv.URL,
		AccountID: "acc-1",
		Token:     "tok-xyz",
	})
	_, _, b, err := cl.DoRequest(http.MethodGet, "scripts", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method: got %q", gotMethod)
	}
	if gotPath != "/scripts" {
		t.Fatalf("path: got %q want /scripts", gotPath)
	}
	if gotAccount != "acc-1" {
		t.Fatalf("Account-Id: got %q", gotAccount)
	}
	if gotAuth != "ApiKey tok-xyz" {
		t.Fatalf("Authorization: got %q", gotAuth)
	}
	if string(b) != `{"ok":true}` {
		t.Fatalf("body: %s", b)
	}
	if gotAccept != "application/json" || gotCT != "application/json" {
		t.Fatalf("Accept/Content-Type: %q %q", gotAccept, gotCT)
	}
	if !strings.HasPrefix(gotUA, "sse-cli/") {
		t.Fatalf("User-Agent: %q", gotUA)
	}
}

func TestClient_HeadersFromAPIKeyFlag(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.Header.Get("X-Api-Key") != "" {
			t.Error("unexpected X-Api-Key header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cl := New(config.Resolved{
		BaseURL:   srv.URL,
		AccountID: "acc",
		APIKey:    "secret-key",
	})
	_, _, _, err := cl.DoRequest(http.MethodGet, "scripts", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "ApiKey secret-key" {
		t.Fatalf("Authorization: got %q", gotAuth)
	}
}

func TestClient_ListScriptsPageSize(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"scripts":[],"next_cursor":null}`))
	}))
	defer srv.Close()

	cl := New(config.Resolved{BaseURL: srv.URL, AccountID: "a", Token: "t"})
	_, err := cl.ListScripts("cur1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "cursor=cur1") || !strings.Contains(gotQuery, "page_size=50") {
		t.Fatalf("query: %q", gotQuery)
	}
}

func TestClient_ExecuteBody(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/scripts/s1/execute" {
			t.Fatalf("path %q", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type: %q", ct)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"run_id":"r1","status":"pending"}`))
	}))
	defer srv.Close()

	cl := New(config.Resolved{BaseURL: srv.URL, AccountID: "a", Token: "t"})
	st, out, err := cl.ExecuteScript("s1", []byte(`{"mode":"async","trigger_type":"manual","payload":{"k":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	if st != http.StatusAccepted {
		t.Fatalf("status %d", st)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["mode"] != "async" || parsed["trigger_type"] != "manual" {
		t.Fatalf("body %s", body)
	}
	if !strings.Contains(string(out), "run_id") {
		t.Fatalf("response %s", out)
	}
}

func TestExtractLogURL(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"url", `{"url":"https://example/logs"}`, "https://example/logs"},
		{"presigned_url", `{"presigned_url":"https://s3/presigned"}`, "https://s3/presigned"},
		{"nested", `{"data":{"logUrl":"https://nested"}}`, "https://nested"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := ExtractLogURL([]byte(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			if u != tc.want {
				t.Fatalf("got %q want %q", u, tc.want)
			}
		})
	}
}

func TestClient_RunLogsFlow(t *testing.T) {
	logContent := "line1\nline2\n"
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/scripts/sid/runs/rid/logs":
			u := srv.URL + "/raw-logs"
			b, _ := json.Marshal(map[string]string{"url": u})
			_, _ = w.Write(b)
		case "/raw-logs":
			_, _ = w.Write([]byte(logContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cl := New(config.Resolved{BaseURL: srv.URL, AccountID: "a", Token: "t"})
	meta, err := cl.GetRunLogsMetadata("sid", "rid")
	if err != nil {
		t.Fatal(err)
	}
	u, err := ExtractLogURL(meta)
	if err != nil {
		t.Fatal(err)
	}
	b, err := FetchURL(cl.HTTP(), u)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != logContent {
		t.Fatalf("logs got %q", b)
	}
}

func TestAPIErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"VALIDATION_FAILED","message":"bad","details":[]}}`))
	}))
	defer srv.Close()

	cl := New(config.Resolved{BaseURL: srv.URL, AccountID: "a", Token: "t"})
	_, _, _, err := cl.DoRequest(http.MethodGet, "scripts", nil, nil, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("want APIError, got %T: %v", err, err)
	}
	if ae.Code != "VALIDATION_FAILED" || ae.Msg != "bad" {
		t.Fatalf("unexpected %+v", ae)
	}
}
