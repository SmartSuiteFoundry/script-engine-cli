package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sse-cli/internal/config"
)

// Client calls the Script Management API (paths and auth per docs/openapi.yaml).
type Client struct {
	base string
	http *http.Client
	cfg  config.Resolved
}

// New builds a client from resolved config.
// BaseURL is the API root (e.g. https://hotfix.ss-stage.com/v1/scripting); paths are /scripts, /runtimes, etc.
// Resolve() already normalizes .../scripting -> .../v1/scripting when the path is a single "scripting" segment.
func New(cfg config.Resolved) *Client {
	b := config.NormalizeBaseURL(cfg.BaseURL)
	return &Client{
		base: b,
		http: &http.Client{Timeout: 120 * time.Second},
		cfg:  cfg,
	}
}

// HTTP returns the underlying client (e.g. for presigned URL fetches).
func (c *Client) HTTP() *http.Client {
	return c.http
}

func (c *Client) url(path string, query url.Values) string {
	p := strings.Trim(path, "/")
	u := c.base + "/" + p
	if query != nil && len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// formatAuthorization builds the Authorization header per OpenAPI TokenAuth:
// "ApiKey <your-token>", while allowing full "Bearer …" or "ApiKey …" values to pass through unchanged.
func formatAuthorization(secret string) string {
	s := strings.TrimSpace(secret)
	if strings.HasPrefix(strings.ToLower(s), "bearer ") {
		return s
	}
	fields := strings.Fields(s)
	if len(fields) >= 2 && strings.ToLower(fields[0]) == "apikey" {
		return "ApiKey " + strings.TrimSpace(strings.Join(fields[1:], " "))
	}
	return "ApiKey " + s
}

func (c *Client) applyAuth(req *http.Request) {
	req.Header.Set("Account-Id", c.cfg.AccountID)
	var cred string
	switch {
	case c.cfg.Token != "":
		cred = c.cfg.Token
	case c.cfg.APIKey != "":
		cred = c.cfg.APIKey
	}
	if cred != "" {
		req.Header.Set("Authorization", formatAuthorization(cred))
	}
}

// applyJSONClientHeaders matches typical JSON API clients (and curl examples): some gateways
// return HTML error pages unless Accept/Content-Type indicate JSON; default Go User-Agent is
// also treated differently than curl by some WAFs.
func applyJSONClientHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "sse-cli/1.0")
	}
}

// APIError is the error envelope from the service.
type APIError struct {
	Status int
	Body   string
	Code   string
	Msg    string
}

func (e *APIError) Error() string {
	if e.Code != "" || e.Msg != "" {
		return fmt.Sprintf("api error: status=%d code=%s message=%s", e.Status, e.Code, e.Msg)
	}
	return fmt.Sprintf("api error: status=%d body=%s", e.Status, truncate(e.Body, 200))
}

type errEnvelope struct {
	Error struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	} `json:"error"`
}

func parseAPIError(status int, body []byte) error {
	var env errEnvelope
	if json.Unmarshal(body, &env) == nil && (env.Error.Code != "" || env.Error.Message != "") {
		return &APIError{Status: status, Code: env.Error.Code, Msg: env.Error.Message, Body: string(body)}
	}
	return &APIError{Status: status, Body: string(body)}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func looksLikeHTML(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	s := bytes.TrimSpace(body)
	if len(s) >= 9 && bytes.EqualFold(s[:9], []byte("<!DOCTYPE")) {
		return true
	}
	if len(s) >= 5 && bytes.EqualFold(s[:5], []byte("<html")) {
		return true
	}
	return false
}

// DoRequest performs HTTP and returns status, headers, and body bytes for 2xx; otherwise error.
func (c *Client) DoRequest(method, path string, query url.Values, body []byte, jsonBody bool) (int, http.Header, []byte, error) {
	u := c.url(path, query)
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return 0, nil, nil, err
	}
	c.applyAuth(req)
	applyJSONClientHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, resp.Header, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, resp.Header, b, parseAPIError(resp.StatusCode, b)
	}
	if looksLikeHTML(resp.Header.Get("Content-Type"), b) {
		return resp.StatusCode, resp.Header, b, fmt.Errorf("received HTML instead of JSON (URL %q): use --base-url ending with /v1/scripting, not .../scripting alone (docs/openapi.yaml servers)", u)
	}
	return resp.StatusCode, resp.Header, b, nil
}

// ListScripts GET /scripts
func (c *Client) ListScripts(cursor string, pageSize int) ([]byte, error) {
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if pageSize > 0 {
		q.Set("page_size", fmt.Sprintf("%d", pageSize))
	}
	_, _, b, err := c.DoRequest(http.MethodGet, "scripts", q, nil, false)
	return b, err
}

// GetScript GET /scripts/{id}
func (c *Client) GetScript(id string) ([]byte, error) {
	_, _, b, err := c.DoRequest(http.MethodGet, pathScript(id), nil, nil, false)
	return b, err
}

// CreateScript POST /scripts
func (c *Client) CreateScript(body []byte) ([]byte, error) {
	_, _, b, err := c.DoRequest(http.MethodPost, "scripts", nil, body, true)
	return b, err
}

// UpdateScript PUT /scripts/{id}
func (c *Client) UpdateScript(id string, body []byte) ([]byte, error) {
	_, _, b, err := c.DoRequest(http.MethodPut, pathScript(id), nil, body, true)
	return b, err
}

// DeleteScript DELETE /scripts/{id}
func (c *Client) DeleteScript(id string) error {
	_, _, _, err := c.DoRequest(http.MethodDelete, pathScript(id), nil, nil, false)
	return err
}

func pathScript(id string) string {
	return fmt.Sprintf("scripts/%s", url.PathEscape(id))
}

// ExecuteScript POST /scripts/{id}/execute
func (c *Client) ExecuteScript(id string, body []byte) (status int, bodyOut []byte, err error) {
	st, _, b, err := c.DoRequest(http.MethodPost, fmt.Sprintf("scripts/%s/execute", url.PathEscape(id)), nil, body, true)
	return st, b, err
}

// ListRuns GET /scripts/{id}/runs
func (c *Client) ListRuns(scriptID, cursor string, pageSize int) ([]byte, error) {
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if pageSize > 0 {
		q.Set("page_size", fmt.Sprintf("%d", pageSize))
	}
	_, _, b, err := c.DoRequest(http.MethodGet, fmt.Sprintf("scripts/%s/runs", url.PathEscape(scriptID)), q, nil, false)
	return b, err
}

// GetRun GET /scripts/{id}/runs/{runId}
func (c *Client) GetRun(scriptID, runID string) ([]byte, error) {
	_, _, b, err := c.DoRequest(http.MethodGet, fmt.Sprintf("scripts/%s/runs/%s", url.PathEscape(scriptID), url.PathEscape(runID)), nil, nil, false)
	return b, err
}

// GetRunLogsMetadata GET /scripts/{id}/runs/{runId}/logs — returns JSON body (presigned URL payload).
func (c *Client) GetRunLogsMetadata(scriptID, runID string) ([]byte, error) {
	_, _, b, err := c.DoRequest(http.MethodGet, fmt.Sprintf("scripts/%s/runs/%s/logs", url.PathEscape(scriptID), url.PathEscape(runID)), nil, nil, false)
	return b, err
}

// ListRuntimes GET /runtimes
func (c *Client) ListRuntimes() ([]byte, error) {
	_, _, b, err := c.DoRequest(http.MethodGet, "runtimes", nil, nil, false)
	return b, err
}

// ListRuntimeLibraries GET /runtimes/{runtime}/libraries
func (c *Client) ListRuntimeLibraries(runtime string) ([]byte, error) {
	_, _, b, err := c.DoRequest(http.MethodGet, fmt.Sprintf("runtimes/%s/libraries", url.PathEscape(runtime)), nil, nil, false)
	return b, err
}

// ExtractLogURL parses common JSON shapes for a presigned log URL.
func ExtractLogURL(meta []byte) (string, error) {
	var m map[string]any
	if err := json.Unmarshal(meta, &m); err != nil {
		return "", err
	}
	keys := []string{"url", "presignedUrl", "presigned_url", "logUrl", "log_url", "logsUrl", "logs_url", "href"}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.HasPrefix(s, "http") {
				return s, nil
			}
		}
	}
	if data, ok := m["data"].(map[string]any); ok {
		for _, k := range keys {
			if v, ok := data[k]; ok {
				if s, ok := v.(string); ok && strings.HasPrefix(s, "http") {
					return s, nil
				}
			}
		}
	}
	return "", errors.New("no presigned URL field found in logs response (expected url, presignedUrl, etc.)")
}

// FetchURL performs GET without API auth (for presigned S3 URLs).
func FetchURL(client *http.Client, rawURL string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return b, fmt.Errorf("fetch logs: status=%d", resp.StatusCode)
	}
	return b, nil
}
