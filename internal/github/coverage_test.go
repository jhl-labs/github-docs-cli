package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- issues: remaining methods ---

func TestListIssueCommentsAndAddComment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]Comment{{ID: 1, Body: "hi", User: &User{Login: "a"}}})
			return
		}
		var body map[string]any
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		json.NewEncoder(w).Encode(Comment{ID: 99, Body: body["body"].(string), HTMLURL: "http://x"})
	}))
	defer srv.Close()
	cl := newTestClient(t, srv)

	cs, err := cl.ListIssueComments(context.Background(), "o", "r", 1)
	if err != nil || len(cs) != 1 || cs[0].Body != "hi" {
		t.Fatalf("ListIssueComments: %v %+v", err, cs)
	}
	c, err := cl.AddIssueComment(context.Background(), "o", "r", 1, "new")
	if err != nil || c.ID != 99 || c.Body != "new" {
		t.Fatalf("AddIssueComment: %v %+v", err, c)
	}
}

func TestAddIssueLabels(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &got)
		json.NewEncoder(w).Encode([]Label{{Name: "a"}, {Name: "b"}})
	}))
	defer srv.Close()
	cl := newTestClient(t, srv)

	labels, err := cl.AddIssueLabels(context.Background(), "o", "r", 3, []string{"a", "b"})
	if err != nil || len(labels) != 2 {
		t.Fatalf("AddIssueLabels: %v %+v", err, labels)
	}
	if ls, _ := got["labels"].([]any); len(ls) != 2 || ls[0] != "a" {
		t.Errorf("posted labels = %v", got["labels"])
	}
	if LabelNames(labels) != "a, b" {
		t.Errorf("LabelNames = %q", LabelNames(labels))
	}
	if joinLabels(nil) != "" {
		t.Errorf("joinLabels(nil) should be empty")
	}
}

// --- contents: remaining methods ---

func TestListDirFileAndDir(t *testing.T) {
	// Directory response (array).
	srvDir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ContentEntry{{Type: "file", Path: "docs/a.md"}, {Type: "dir", Path: "docs/sub"}})
	}))
	defer srvDir.Close()
	entries, err := newTestClient(t, srvDir).ListDir(context.Background(), "o", "r", "docs", "")
	if err != nil || len(entries) != 2 {
		t.Fatalf("ListDir dir: %v %+v", err, entries)
	}

	// Single-file response (object) -> wrapped into a one-element slice.
	srvFile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ContentEntry{Type: "file", Path: "docs/a.md"})
	}))
	defer srvFile.Close()
	entries, err = newTestClient(t, srvFile).ListDir(context.Background(), "o", "r", "docs/a.md", "main")
	if err != nil || len(entries) != 1 || entries[0].Path != "docs/a.md" {
		t.Fatalf("ListDir file: %v %+v", err, entries)
	}
}

func TestDecodedPlain(t *testing.T) {
	e := ContentEntry{Content: "raw text", Encoding: ""}
	got, err := e.Decoded()
	if err != nil || got != "raw text" {
		t.Errorf("Decoded plain = %q, %v", got, err)
	}
}

func TestDeleteFile(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		json.NewEncoder(w).Encode(FileCommit{})
	}))
	defer srv.Close()
	_, err := newTestClient(t, srv).DeleteFile(context.Background(), "o", "r", "docs/a.md", "msg", "sha1", "main")
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if body["sha"] != "sha1" || body["message"] != "msg" || body["branch"] != "main" {
		t.Errorf("delete payload = %v", body)
	}
}

func TestRequestPagesBuildAndDefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	}))
	defer srv.Close()
	cl := newTestClient(t, srv)
	if err := cl.RequestPagesBuild(context.Background(), "o", "r"); err != nil {
		t.Fatalf("RequestPagesBuild: %v", err)
	}
	br, err := cl.DefaultBranch(context.Background(), "o", "r")
	if err != nil || br != "main" {
		t.Fatalf("DefaultBranch: %v %q", err, br)
	}
}

func TestGetPagesAndEnableCreate(t *testing.T) {
	// EnablePages when pages not yet enabled: GET 404 -> POST create.
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if posted {
				json.NewEncoder(w).Encode(PagesInfo{Status: "built", HTMLURL: "http://pages"})
			} else {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"Not Found"}`))
			}
		case http.MethodPost:
			posted = true
			json.NewEncoder(w).Encode(PagesInfo{HTMLURL: "http://pages", Status: "queued"})
		}
	}))
	defer srv.Close()
	cl := newTestClient(t, srv)

	// GetPages 404 path.
	if _, err := cl.GetPages(context.Background(), "o", "r"); !NotFound(err) {
		t.Errorf("expected NotFound before enable, got %v", err)
	}
	info, err := cl.EnablePages(context.Background(), "o", "r", "main", "/docs")
	if err != nil || info.HTMLURL != "http://pages" {
		t.Fatalf("EnablePages create: %v %+v", err, info)
	}
}

// --- discussions (GraphQL) ---

// graphqlMux routes GraphQL requests by matching substrings of the query.
func graphqlMux(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var req struct {
			Query string `json:"query"`
		}
		json.Unmarshal(data, &req)
		for needle, resp := range routes {
			if strings.Contains(req.Query, needle) {
				w.Write([]byte(resp))
				return
			}
		}
		t.Errorf("no graphql route matched query: %s", req.Query)
		w.Write([]byte(`{"data":{}}`))
	}))
}

func TestGetDiscussionAndCategories(t *testing.T) {
	srv := graphqlMux(t, map[string]string{
		"discussion(number":    `{"data":{"repository":{"discussion":{"id":"D1","number":7,"title":"T","body":"B","url":"http://d","category":{"id":"C1","name":"General"},"author":{"login":"a"},"comments":{"nodes":[{"id":"DC1","body":"hello","author":{"login":"b"}}]}}}}}`,
		"discussionCategories": `{"data":{"repository":{"discussionCategories":{"nodes":[{"id":"C1","name":"General","description":"chat"}]}}}}`,
	})
	defer srv.Close()
	cl := newTestClient(t, srv)

	d, err := cl.GetDiscussion(context.Background(), "o", "r", 7)
	if err != nil || d.Number != 7 || len(d.Comments) != 1 || d.Comments[0].Body != "hello" {
		t.Fatalf("GetDiscussion: %v %+v", err, d)
	}
	cats, err := cl.ListDiscussionCategories(context.Background(), "o", "r")
	if err != nil || len(cats) != 1 || cats[0].Name != "General" {
		t.Fatalf("ListDiscussionCategories: %v %+v", err, cats)
	}
}

func TestCreateDiscussion(t *testing.T) {
	srv := graphqlMux(t, map[string]string{
		// repoAndCategoryID query asks for id + discussionCategories together.
		"discussionCategories(first:50)": `{"data":{"repository":{"id":"R1","discussionCategories":{"nodes":[{"id":"C1","name":"Ideas"}]}}}}`,
		"createDiscussion":               `{"data":{"createDiscussion":{"discussion":{"id":"D9","number":9,"title":"RFC","url":"http://d9","category":{"id":"C1","name":"Ideas"}}}}}`,
	})
	defer srv.Close()
	cl := newTestClient(t, srv)

	d, err := cl.CreateDiscussion(context.Background(), "o", "r", "Ideas", "RFC", "body")
	if err != nil || d.Number != 9 {
		t.Fatalf("CreateDiscussion: %v %+v", err, d)
	}
}

func TestCreateDiscussionCategoryNotFound(t *testing.T) {
	srv := graphqlMux(t, map[string]string{
		"discussionCategories(first:50)": `{"data":{"repository":{"id":"R1","discussionCategories":{"nodes":[{"id":"C1","name":"Ideas"}]}}}}`,
	})
	defer srv.Close()
	cl := newTestClient(t, srv)
	_, err := cl.CreateDiscussion(context.Background(), "o", "r", "Nonexistent", "T", "b")
	if err == nil || !strings.Contains(err.Error(), "category not found") {
		t.Fatalf("expected category-not-found error, got %v", err)
	}
}

func TestAddDiscussionComment(t *testing.T) {
	srv := graphqlMux(t, map[string]string{
		"discussion(number":    `{"data":{"repository":{"discussion":{"id":"D1","number":7,"title":"T","category":{"id":"C1","name":"General"},"comments":{"nodes":[]}}}}}`,
		"addDiscussionComment": `{"data":{"addDiscussionComment":{"comment":{"id":"DC9","body":"ok","url":"http://c9","author":{"login":"a"}}}}}`,
	})
	defer srv.Close()
	cl := newTestClient(t, srv)
	c, err := cl.AddDiscussionComment(context.Background(), "o", "r", 7, "ok")
	if err != nil || c.Body != "ok" {
		t.Fatalf("AddDiscussionComment: %v %+v", err, c)
	}
}

// --- client edge cases ---

func TestRetryAfterHonored(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(Issue{Number: 1})
	}))
	defer srv.Close()
	cl := newTestClient(t, srv)
	cl.RetryWait = time.Millisecond
	if _, err := cl.GetIssue(context.Background(), "o", "r", 1); err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestSecondaryRateLimit403Retries(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		json.NewEncoder(w).Encode(Issue{Number: 1})
	}))
	defer srv.Close()
	cl := newTestClient(t, srv)
	cl.RetryWait = time.Millisecond
	if _, err := cl.GetIssue(context.Background(), "o", "r", 1); err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (403 + Retry-After should retry)", calls)
	}
}

func TestForbiddenWithoutRetryAfterFails(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Forbidden"}`))
	}))
	defer srv.Close()
	cl := newTestClient(t, srv)
	_, err := cl.GetIssue(context.Background(), "o", "r", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on plain 403)", calls)
	}
}

func TestParseAPIErrorShortBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`plain text error`))
	}))
	defer srv.Close()
	cl := newTestClient(t, srv)
	_, err := cl.GetIssue(context.Background(), "o", "r", 1)
	apiErr, ok := err.(*APIError)
	if !ok || !strings.Contains(apiErr.Message, "plain text error") {
		t.Fatalf("err = %v", err)
	}
	if apiErr.Error() == "" {
		t.Error("Error() empty")
	}
}

func TestWebURLHelpersNoOp(t *testing.T) {
	// graphqlURL trailing-slash trimming for GHE without /api/v3.
	if got := graphqlURL("https://ghe.example.com/"); got != "https://ghe.example.com/graphql" {
		t.Errorf("graphqlURL = %q", got)
	}
}
