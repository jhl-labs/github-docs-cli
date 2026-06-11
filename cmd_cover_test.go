package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// errServer returns 500 for every request, to exercise error-return branches.
func errServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"boom"}`))
	}))
}

func TestCommandErrorsPropagate(t *testing.T) {
	srv := errServer(t)
	defer srv.Close()
	cases := []struct {
		fn   func([]string) error
		args []string
	}{
		{runIssue, []string{"list"}},
		{runIssue, []string{"get", "--number", "1"}},
		{runIssue, []string{"create", "--title", "T"}},
		{runIssue, []string{"update", "--number", "1", "--state", "closed"}},
		{runIssue, []string{"comment", "--number", "1", "--body", "x"}},
		{runIssue, []string{"label", "--number", "1", "--add", "a"}},
		{runDocs, []string{"list"}},
		{runDocs, []string{"get", "--path", "x.md"}},
		{runDocs, []string{"tree"}},
		{runDocs, []string{"put", "--path", "x.md", "--body", "y", "--message", "m"}},
		{runDocs, []string{"delete", "--path", "x.md", "--message", "m"}},
		{runPages, []string{"get"}},
		{runPages, []string{"enable", "--branch", "main"}},
		{runPages, []string{"build"}},
		{runDiscussion, []string{"list"}},
		{runDiscussion, []string{"get", "--number", "1"}},
		{runDiscussion, []string{"categories"}},
		{runDiscussion, []string{"create", "--category", "X", "--title", "T"}},
		{runDiscussion, []string{"comment", "--number", "1", "--body", "x"}},
		{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x"}},
		{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x", "--issue", "1"}},
		{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x", "--new-issue"}},
	}
	for _, c := range cases {
		if _, err := runCmd(t, srv.URL, c.fn, c.args...); err == nil {
			t.Errorf("expected error for %v", c.args)
		}
	}
}

func TestInvalidRepoErrors(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	// Override GITHUB_REPO to an invalid value after runCmd sets it: use args.
	t.Setenv("GITHUB_DOCS_CONFIG", "")
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_TOKEN", "tok")
	t.Setenv("GITHUB_REPO", "noslash")
	if err := runIssue([]string{"list"}); err == nil {
		t.Error("expected error for invalid repo")
	}
}

// emptyServer returns empty collections to exercise the "no results" branches.
func emptyServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/graphql"):
			handleGraphQL(w, r) // returns empty-ish nodes for list/categories
		case strings.Contains(r.URL.Path, "/issues/") && strings.Contains(r.URL.Path, "/labels"):
			w.Write([]byte(`[]`))
		case strings.Contains(r.URL.Path, "/issues/"):
			w.Write([]byte(`{"number":1,"title":"T","state":"open","labels":[]}`))
		case strings.HasSuffix(r.URL.Path, "/issues"):
			w.Write([]byte(`[]`))
		case strings.Contains(r.URL.Path, "/git/trees/"):
			w.Write([]byte(`{"sha":"s","tree":[]}`))
		case strings.HasSuffix(r.URL.Path, "/repos/o/r"):
			w.Write([]byte(`{"default_branch":"main"}`))
		case strings.HasSuffix(r.URL.Path, "/contents/docs"):
			w.Write([]byte(`[]`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
}

func TestEmptyResultBranches(t *testing.T) {
	srv := emptyServer(t)
	defer srv.Close()
	checks := []struct {
		fn   func([]string) error
		args []string
		want string
	}{
		{runIssue, []string{"list", "--output", "text"}, "no issues"},
		{runIssue, []string{"label", "--number", "1", "--output", "text"}, "no labels"},
		{runDocs, []string{"list", "--output", "text"}, "empty"},
		{runDocs, []string{"tree", "--output", "text"}, "no files"},
	}
	for _, c := range checks {
		out, err := runCmd(t, srv.URL, c.fn, c.args...)
		if err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("args %v output %q missing %q", c.args, out, c.want)
		}
	}
}

func TestNoteAppendToExistingFile(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	// docs/notes/research.md is NOT the DEVLOG path, so the mock returns
	// existing content -> exercises the append-to-existing branch.
	out, err := runCmd(t, srv.URL, runNote,
		"add", "--kind", "research", "--title", "X", "--body", "y",
		"--docs", "docs/notes/research.md", "--branch", "main", "--output", "text")
	if err != nil {
		t.Fatalf("note append: %v", err)
	}
	if !strings.Contains(out, "recorded research note") {
		t.Errorf("output %q", out)
	}
}

// callMain sets env pointing at srv and runs the dispatcher, returning its
// exit code. It drives main()'s command switch via run().
func callMain(t *testing.T, srvURL string, args ...string) int {
	t.Helper()
	t.Setenv("GITHUB_DOCS_CONFIG", "")
	t.Setenv("GITHUB_API_URL", srvURL)
	t.Setenv("GITHUB_TOKEN", "tok")
	t.Setenv("GITHUB_REPO", "o/r")
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	code := run(args)
	w.Close()
	os.Stdout = old
	return code
}

func TestMainDispatchGroups(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	// Each valid command exercises a switch case in main() without os.Exit.
	callMain(t, srv.URL, "issue", "list")
	callMain(t, srv.URL, "docs", "list", "--path", "docs")
	callMain(t, srv.URL, "pages", "get")
	callMain(t, srv.URL, "discussion", "list")
	callMain(t, srv.URL, "note", "add", "--kind", "dev", "--title", "T", "--body", "x", "--issue", "7")
	callMain(t, srv.URL, "generate-skill", "--stdout")
	callMain(t, srv.URL, "generate-workflow", "pages", "--stdout")
}
