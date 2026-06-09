package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github-docs-cli/internal/config"
)

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	cl, err := New(config.Config{APIURL: srv.URL, Token: "t"}, 5*time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cl.RetryWait = time.Millisecond // keep tests fast
	return cl
}

func TestSendsBearerAuthAndVersion(t *testing.T) {
	var gotAuth, gotVersion, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("X-GitHub-Api-Version")
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(Issue{Number: 1, Title: "Hi"})
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	if _, err := cl.GetIssue(context.Background(), "o", "r", 1); err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if gotAuth != "Bearer t" {
		t.Errorf("auth header = %q, want Bearer t", gotAuth)
	}
	if gotVersion != "2022-11-28" {
		t.Errorf("api version header = %q", gotVersion)
	}
	if gotPath != "/repos/o/r/issues/1" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRetriesOn503ThenSucceeds(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(Issue{Number: 9})
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	got, err := cl.GetIssue(context.Background(), "o", "r", 9)
	if err != nil {
		t.Fatalf("GetIssue after retries: %v", err)
	}
	if calls != 3 {
		t.Errorf("server calls = %d, want 3", calls)
	}
	if got.Number != 9 {
		t.Errorf("number = %d", got.Number)
	}
}

func TestNoRetryOn404(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	_, err := cl.GetIssue(context.Background(), "o", "r", 5)
	if err == nil {
		t.Fatal("expected error")
	}
	if !NotFound(err) {
		t.Errorf("NotFound(err) = false, want true (err=%v)", err)
	}
	if calls != 1 {
		t.Errorf("server calls = %d, want 1 (no retry on 404)", calls)
	}
}

func TestParseAPIErrorWithFieldDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Validation Failed","errors":[{"field":"title","code":"missing_field"}]}`))
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	_, err := cl.CreateIssue(context.Background(), "o", "r", CreateIssueInput{})
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != 422 {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if want := "Validation Failed (title: missing_field)"; apiErr.Message != want {
		t.Errorf("message = %q, want %q", apiErr.Message, want)
	}
}

func TestGraphQLErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":null,"errors":[{"message":"Could not resolve to a Repository"}]}`))
	}))
	defer srv.Close()

	cl := newTestClient(t, srv)
	_, err := cl.ListDiscussions(context.Background(), "o", "r", 10)
	if err == nil {
		t.Fatal("expected graphql error")
	}
	if want := "Could not resolve to a Repository"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err.Error(), want)
	}
}

func TestGraphQLURLDerivation(t *testing.T) {
	tests := map[string]string{
		"https://api.github.com":         "https://api.github.com/graphql",
		"https://ghe.example.com/api/v3": "https://ghe.example.com/api/graphql",
		"https://other.example.com":      "https://other.example.com/graphql",
	}
	for in, want := range tests {
		if got := graphqlURL(in); got != want {
			t.Errorf("graphqlURL(%q) = %q, want %q", in, got, want)
		}
	}
}
