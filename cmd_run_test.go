package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockGitHub returns an httptest server that answers the REST + GraphQL routes
// the commands use, and a pointer to a captured-requests log.
func mockGitHub(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !handleMockGitHubRoute(w, r) {
			t.Logf("unhandled %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
}

type mockRoute struct {
	match  func(*http.Request) bool
	handle func(http.ResponseWriter, *http.Request)
}

func handleMockGitHubRoute(w http.ResponseWriter, r *http.Request) bool {
	for _, route := range mockRoutes() {
		if route.match(r) {
			route.handle(w, r)
			return true
		}
	}
	return false
}

func mockRoutes() []mockRoute {
	return []mockRoute{
		{hasSuffix("/graphql"), handleGraphQL},
		{methodAnd(contains("/issues/"), hasSuffix("/comments"), isMethod(http.MethodGet)), writeJSON([]map[string]any{{"id": 1, "body": "c", "user": map[string]any{"login": "u"}}})},
		{methodAnd(contains("/issues/"), hasSuffix("/comments"), isMethod(http.MethodPost)), writeJSON(map[string]any{"id": 5, "body": "x", "html_url": "http://x/c"})},
		{methodAnd(contains("/issues/"), contains("/labels")), writeJSON([]map[string]any{{"name": "docs"}})},
		{methodAnd(hasSuffix("/issues"), isMethod(http.MethodGet)), writeJSON([]map[string]any{
			{"number": 1, "title": "Doc", "state": "open", "labels": []map[string]any{{"name": "docs"}}},
			{"number": 2, "title": "PR", "state": "open", "pull_request": map[string]any{"url": "x"}},
		})},
		{methodAnd(hasSuffix("/issues"), isMethod(http.MethodPost)), writeJSON(map[string]any{"number": 7, "title": "T", "state": "open", "html_url": "http://x/7"})},
		{methodAnd(contains("/issues/"), isMethod(http.MethodGet)), writeJSON(map[string]any{"number": 7, "title": "T", "state": "open", "html_url": "http://x/7", "labels": []map[string]any{{"name": "docs"}}, "user": map[string]any{"login": "u"}, "body": "B"})},
		{methodAnd(contains("/issues/"), isMethod(http.MethodPatch)), writeJSON(map[string]any{"number": 7, "state": "closed", "html_url": "http://x/7"})},
		{contains("/contents/"), handleContents},
		{contains("/git/trees/"), writeJSON(map[string]any{"sha": "root", "truncated": false, "tree": []map[string]any{
			{"path": "docs/a.md", "type": "blob"}, {"path": "docs/sub", "type": "tree"}, {"path": "README.md", "type": "blob"},
		}})},
		{methodAnd(hasSuffix("/pages"), isMethod(http.MethodGet)), writeJSON(map[string]any{"status": "built", "html_url": "http://pages", "source": map[string]any{"branch": "main", "path": "/docs"}})},
		{methodAnd(hasSuffix("/pages"), isMethod(http.MethodPost)), writeJSON(map[string]any{"status": "queued", "html_url": "http://pages"})},
		{methodAnd(hasSuffix("/pages"), isMethod(http.MethodPut)), statusOnly(http.StatusNoContent)},
		{methodAnd(hasSuffix("/pages/builds"), isMethod(http.MethodPost)), statusOnly(http.StatusCreated)},
		{func(r *http.Request) bool {
			return strings.HasSuffix(r.URL.Path, "/repos/o/r") || strings.HasSuffix(r.URL.Path, "/repos/o/r/")
		}, writeJSON(map[string]any{"default_branch": "main"})},
	}
}

func contains(part string) func(*http.Request) bool {
	return func(r *http.Request) bool { return strings.Contains(r.URL.Path, part) }
}

func hasSuffix(suffix string) func(*http.Request) bool {
	return func(r *http.Request) bool { return strings.HasSuffix(r.URL.Path, suffix) }
}

func isMethod(method string) func(*http.Request) bool {
	return func(r *http.Request) bool { return r.Method == method }
}

func methodAnd(matchers ...func(*http.Request) bool) func(*http.Request) bool {
	return func(r *http.Request) bool {
		for _, match := range matchers {
			if !match(r) {
				return false
			}
		}
		return true
	}
}

func writeJSON(v any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(v)
	}
}

func statusOnly(status int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}
}

func handleContents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if strings.Contains(r.URL.Path, "DEVLOG") || strings.Contains(r.URL.Path, "missing") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/contents/docs") {
			json.NewEncoder(w).Encode([]map[string]any{{"type": "file", "name": "a.md", "path": "docs/a.md"}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"type": "file", "name": "a.md", "path": "docs/a.md", "sha": "sha1",
			"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("# A\n")),
		})
	case http.MethodPut, http.MethodDelete:
		json.NewEncoder(w).Encode(map[string]any{
			"content": map[string]any{"path": "docs/a.md", "sha": "sha2"},
			"commit":  map[string]any{"sha": "c1", "html_url": "http://x/commit"},
		})
	}
}

func handleGraphQL(w http.ResponseWriter, r *http.Request) {
	data, _ := io.ReadAll(r.Body)
	var req struct {
		Query string `json:"query"`
	}
	json.Unmarshal(data, &req)
	q := req.Query
	switch {
	case strings.Contains(q, "createDiscussion"):
		w.Write([]byte(`{"data":{"createDiscussion":{"discussion":{"id":"D9","number":9,"title":"RFC","url":"http://d9","category":{"id":"C1","name":"Ideas"}}}}}`))
	case strings.Contains(q, "addDiscussionComment"):
		w.Write([]byte(`{"data":{"addDiscussionComment":{"comment":{"id":"DC9","body":"ok","url":"http://c9","author":{"login":"a"}}}}}`))
	case strings.Contains(q, "discussion(number"):
		w.Write([]byte(`{"data":{"repository":{"discussion":{"id":"D1","number":7,"title":"T","body":"B","url":"http://d","category":{"id":"C1","name":"General"},"author":{"login":"a"},"comments":{"nodes":[{"id":"DC1","body":"hi","author":{"login":"b"}}]}}}}}`))
	case strings.Contains(q, "discussionCategories(first:50)") && strings.Contains(q, "id\n"):
		w.Write([]byte(`{"data":{"repository":{"id":"R1","discussionCategories":{"nodes":[{"id":"C1","name":"Ideas"}]}}}}`))
	case strings.Contains(q, "discussionCategories"):
		w.Write([]byte(`{"data":{"repository":{"discussionCategories":{"nodes":[{"id":"C1","name":"General","description":"chat"}]}}}}`))
	case strings.Contains(q, "discussions(first"):
		w.Write([]byte(`{"data":{"repository":{"discussions":{"nodes":[{"id":"D1","number":7,"title":"T","url":"http://d","category":{"id":"C1","name":"General"}}]}}}}`))
	default:
		w.Write([]byte(`{"data":{}}`))
	}
}

// runCmd sets isolated env pointing at srv, runs fn(args), and returns stdout.
func runCmd(t *testing.T, srvURL string, fn func([]string) error, args ...string) (string, error) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "cfg.json")
	os.WriteFile(cfg, []byte(`{}`), 0o644)
	t.Setenv("GITHUB_DOCS_CONFIG", cfg)
	t.Setenv("GITHUB_API_URL", srvURL)
	t.Setenv("GITHUB_TOKEN", "tok")
	t.Setenv("GITHUB_REPO", "o/r")
	t.Setenv("GH_TOKEN", "")

	old := os.Stdout
	rd, wr, _ := os.Pipe()
	os.Stdout = wr
	err := fn(args)
	wr.Close()
	os.Stdout = old
	out, _ := io.ReadAll(rd)
	return string(out), err
}

func TestIssueCommands(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"list-json", []string{"list"}, `"number": 1`},
		{"list-text", []string{"list", "--output", "text"}, "#1"},
		{"get", []string{"get", "--number", "7", "--comments", "--output", "text"}, "#7"},
		{"create", []string{"create", "--title", "T", "--body", "B", "--label", "docs", "--output", "text"}, "created issue #7"},
		{"update", []string{"update", "--number", "7", "--state", "closed", "--body", "new", "--output", "text"}, "updated issue #7"},
		{"comment", []string{"comment", "--number", "7", "--body", "hi", "--output", "text"}, "added comment"},
		{"label", []string{"label", "--number", "7", "--add", "docs", "--remove", "old", "--output", "text"}, "docs"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := runCmd(t, srv.URL, runIssue, c.args...)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("output %q missing %q", out, c.want)
			}
		})
	}
}

func TestDocsCommands(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"list", []string{"list", "--path", "docs", "--output", "text"}, "docs/a.md"},
		{"tree", []string{"tree", "--path", "docs", "--output", "text"}, "docs/"},
		{"get-raw", []string{"get", "--path", "docs/a.md", "--output", "text"}, "# A"},
		{"get-json", []string{"get", "--path", "docs/a.md"}, `"content"`},
		{"put", []string{"put", "--path", "docs/a.md", "--body", "x", "--message", "m", "--branch", "main", "--output", "text"}, "docs/a.md"},
		{"delete", []string{"delete", "--path", "docs/a.md", "--message", "m", "--output", "text"}, "deleted"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := runCmd(t, srv.URL, runDocs, c.args...)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("output %q missing %q", out, c.want)
			}
		})
	}
}

func TestPagesCommands(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	for _, c := range []struct {
		name, want string
		args       []string
	}{
		{"get", "built", []string{"get", "--output", "text"}},
		{"enable", "Pages enabled", []string{"enable", "--branch", "main", "--path", "/docs", "--output", "text"}},
		{"build", "build requested", []string{"build", "--output", "text"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, err := runCmd(t, srv.URL, runPages, c.args...)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("output %q missing %q", out, c.want)
			}
		})
	}
}

func TestDiscussionCommands(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	for _, c := range []struct {
		name, want string
		args       []string
	}{
		{"categories", "General", []string{"categories", "--output", "text"}},
		{"list", "#7", []string{"list", "--output", "text"}},
		{"get", "#7", []string{"get", "--number", "7", "--output", "text"}},
		{"create", "created discussion #9", []string{"create", "--category", "Ideas", "--title", "RFC", "--body", "b", "--output", "text"}},
		{"comment", "added comment", []string{"comment", "--number", "7", "--body", "ok", "--output", "text"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, err := runCmd(t, srv.URL, runDiscussion, c.args...)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("output %q missing %q", out, c.want)
			}
		})
	}
}

func TestNoteCommands(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	for _, c := range []struct {
		name, want string
		args       []string
	}{
		{"default-docs", "recorded decision note", []string{"add", "--kind", "decision", "--title", "Use Go", "--body", "x", "--branch", "main", "--output", "text"}},
		{"issue", "on issue #7", []string{"add", "--kind", "dev", "--title", "R", "--body", "x", "--issue", "7", "--output", "text"}},
		{"new-issue", "as issue #7", []string{"add", "--kind", "decision", "--title", "A", "--body", "x", "--new-issue", "--output", "text"}},
		{"discussion", "on discussion #7", []string{"add", "--kind", "research", "--title", "D", "--body", "x", "--discussion", "7", "--output", "text"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, err := runCmd(t, srv.URL, runNote, c.args...)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("output %q missing %q", out, c.want)
			}
		})
	}
}

func TestNoteAddValidation(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	if _, err := runCmd(t, srv.URL, runNote, "add", "--kind", "bogus", "--title", "x"); err == nil {
		t.Error("expected error for bad kind")
	}
	if _, err := runCmd(t, srv.URL, runNote, "add", "--kind", "dev"); err == nil {
		t.Error("expected error when neither title nor body given")
	}
}

func TestDispatchErrors(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	if _, err := runCmd(t, srv.URL, runIssue); err == nil {
		t.Error("missing action should error")
	}
	if _, err := runCmd(t, srv.URL, runIssue, "bogus"); err == nil {
		t.Error("unknown action should error")
	}
	// help action prints usage without error.
	if _, err := runCmd(t, srv.URL, runIssue, "help"); err != nil {
		t.Errorf("help should not error: %v", err)
	}
}

func TestMissingRequiredFlags(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	for _, args := range [][]string{
		{"get"},                      // issue get without --number
		{"create"},                   // issue create without --title
		{"comment", "--number", "7"}, // comment without body
	} {
		if _, err := runCmd(t, srv.URL, runIssue, args...); err == nil {
			t.Errorf("expected error for args %v", args)
		}
	}
}
