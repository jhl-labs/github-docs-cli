package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github-docs-cli/internal/config"
)

// errClient points at a server that 500s on every request.
func errClient(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"boom"}`))
	}))
	t.Cleanup(srv.Close)
	cl, err := New(config.Config{APIURL: srv.URL, Token: "t"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cl.MaxRetries = 0
	return cl
}

// TestAllMethodsErrorPaths exercises every method's error-return branch.
func TestAllMethodsErrorPaths(t *testing.T) {
	cl := errClient(t)
	ctx := context.Background()
	checks := []func() error{
		func() error { _, e := cl.ListIssues(ctx, "o", "r", ListIssuesOptions{Limit: 1}); return e },
		func() error { _, e := cl.GetIssue(ctx, "o", "r", 1); return e },
		func() error { _, e := cl.ListIssueComments(ctx, "o", "r", 1); return e },
		func() error { _, e := cl.CreateIssue(ctx, "o", "r", CreateIssueInput{Title: "T"}); return e },
		func() error { _, e := cl.UpdateIssue(ctx, "o", "r", 1, UpdateIssueInput{State: "open"}); return e },
		func() error { _, e := cl.AddIssueComment(ctx, "o", "r", 1, "b"); return e },
		func() error { _, e := cl.AddIssueLabels(ctx, "o", "r", 1, []string{"a"}); return e },
		func() error { return cl.RemoveIssueLabel(ctx, "o", "r", 1, "a") },
		func() error { _, e := cl.GetFile(ctx, "o", "r", "p", ""); return e },
		func() error { _, e := cl.ListDir(ctx, "o", "r", "p", ""); return e },
		func() error { _, e := cl.PutFile(ctx, "o", "r", PutFileInput{Path: "p", Message: "m"}); return e },
		func() error { _, e := cl.DeleteFile(ctx, "o", "r", "p", "m", "s", ""); return e },
		func() error { _, e := cl.GetTree(ctx, "o", "r", "main", true); return e },
		func() error { _, e := cl.GetPages(ctx, "o", "r"); return e },
		func() error { _, e := cl.EnablePages(ctx, "o", "r", "main", "/docs"); return e },
		func() error { return cl.RequestPagesBuild(ctx, "o", "r") },
		func() error { _, e := cl.DefaultBranch(ctx, "o", "r"); return e },
		func() error { _, e := cl.ListDiscussions(ctx, "o", "r", 5); return e },
		func() error { _, e := cl.GetDiscussion(ctx, "o", "r", 1); return e },
		func() error { _, e := cl.ListDiscussionCategories(ctx, "o", "r"); return e },
		func() error { _, e := cl.CreateDiscussion(ctx, "o", "r", "C", "T", "b"); return e },
		func() error { _, e := cl.AddDiscussionComment(ctx, "o", "r", 1, "b"); return e },
	}
	for i, fn := range checks {
		if err := fn(); err == nil {
			t.Errorf("check %d: expected error", i)
		}
	}
}

func TestNewInsecureAndValidation(t *testing.T) {
	if _, err := New(config.Config{APIURL: "https://api.github.com", Token: "t", Insecure: true}, time.Second); err != nil {
		t.Errorf("New insecure: %v", err)
	}
	if _, err := New(config.Config{Token: "t"}, time.Second); err == nil {
		t.Error("New should fail without API URL")
	}
}

func TestBackoffHonorsRetryAfter(t *testing.T) {
	c := &Client{RetryWait: time.Millisecond}
	c.lastRetryAfter = 7 * time.Second
	if d := c.backoff(1); d != 7*time.Second {
		t.Errorf("backoff = %v, want 7s (Retry-After honored)", d)
	}
	// Consumed; next backoff is exponential.
	if d := c.backoff(1); d != time.Millisecond {
		t.Errorf("backoff after consume = %v, want 1ms", d)
	}
}

func TestNetworkErrorAndCancelledContext(t *testing.T) {
	// Connection refused: point at a closed port.
	cl, _ := New(config.Config{APIURL: "http://127.0.0.1:0", Token: "t"}, time.Second)
	cl.MaxRetries = 0
	if _, err := cl.GetIssue(context.Background(), "o", "r", 1); err == nil {
		t.Error("expected network error")
	}

	// Cancelled context returns promptly with an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	cl2, _ := New(config.Config{APIURL: srv.URL, Token: "t"}, time.Second)
	cl2.RetryWait = time.Hour // force the wait so cancellation wins
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cl2.GetIssue(ctx, "o", "r", 1); err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestGraphQLDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	cl, _ := New(config.Config{APIURL: srv.URL, Token: "t"}, time.Second)
	if _, err := cl.ListDiscussions(context.Background(), "o", "r", 5); err == nil {
		t.Error("expected graphql decode error")
	}
}

func TestJSONDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()
	cl, _ := New(config.Config{APIURL: srv.URL, Token: "t"}, time.Second)
	if _, err := cl.GetIssue(context.Background(), "o", "r", 1); err == nil {
		t.Error("expected json decode error")
	}
}
