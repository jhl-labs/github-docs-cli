package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// runWithEnv runs fn(args) with the given env, discarding stdout, returning err.
func runWithEnv(t *testing.T, env map[string]string, fn func([]string) error, args ...string) error {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := fn(args)
	w.Close()
	os.Stdout = old
	io.Discard.Write(nil)
	return err
}

// allActions is one valid invocation per command action (required flags set),
// used to drive the shared auth/repo/config error branches.
var allActions = []struct {
	fn   func([]string) error
	args []string
}{
	{runIssue, []string{"list"}},
	{runIssue, []string{"get", "--number", "7"}},
	{runIssue, []string{"create", "--title", "T", "--body", "B"}},
	{runIssue, []string{"update", "--number", "7", "--state", "closed"}},
	{runIssue, []string{"comment", "--number", "7", "--body", "x"}},
	{runIssue, []string{"label", "--number", "7", "--add", "a"}},
	{runDocs, []string{"list"}},
	{runDocs, []string{"tree"}},
	{runDocs, []string{"get", "--path", "x.md"}},
	{runDocs, []string{"put", "--path", "x.md", "--body", "y", "--message", "m"}},
	{runDocs, []string{"delete", "--path", "x.md", "--message", "m"}},
	{runPages, []string{"get"}},
	{runPages, []string{"enable", "--branch", "main"}},
	{runPages, []string{"build"}},
	{runDiscussion, []string{"list"}},
	{runDiscussion, []string{"get", "--number", "7"}},
	{runDiscussion, []string{"categories"}},
	{runDiscussion, []string{"create", "--category", "X", "--title", "T", "--body", "b"}},
	{runDiscussion, []string{"comment", "--number", "7", "--body", "x"}},
	{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x"}},
	{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x", "--issue", "7"}},
	{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x", "--new-issue"}},
	{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x", "--discussion", "7"}},
	{runWiki, []string{"list"}},
	{runWiki, []string{"get", "--page", "Home"}},
	{runWiki, []string{"set", "--page", "P", "--body", "x"}},
	{runWiki, []string{"delete", "--page", "P"}},
}

func TestSharedErrorBranches(t *testing.T) {
	good := filepath.Join(t.TempDir(), "good.json")
	os.WriteFile(good, []byte(`{}`), 0o644)
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{not json`), 0o644)

	modes := []struct {
		name string
		env  map[string]string
	}{
		{"malformed-config", map[string]string{
			"GITHUB_DOCS_CONFIG": bad, "GITHUB_API_URL": "https://api.github.com",
			"GITHUB_TOKEN": "t", "GITHUB_REPO": "o/r", "GH_TOKEN": "",
		}},
		{"bad-repo", map[string]string{
			"GITHUB_DOCS_CONFIG": good, "GITHUB_API_URL": "https://api.github.com",
			"GITHUB_TOKEN": "t", "GITHUB_REPO": "noslash", "GH_TOKEN": "",
		}},
		{"empty-token", map[string]string{
			"GITHUB_DOCS_CONFIG": good, "GITHUB_API_URL": "https://api.github.com",
			"GITHUB_TOKEN": "", "GITHUB_REPO": "o/r", "GH_TOKEN": "",
		}},
	}
	for _, m := range modes {
		for _, a := range allActions {
			if err := runWithEnv(t, m.env, a.fn, a.args...); err == nil {
				t.Errorf("[%s] %v: expected error", m.name, a.args)
			}
		}
	}
}

func TestMainWikiCase(t *testing.T) {
	// Cover the wiki branch of main()'s dispatch (token-missing -> error path,
	// but main() swallows the error via os.Exit only on non-nil; here we set a
	// token and a clone seam so it returns cleanly is hard — instead assert it
	// runs without panicking by pointing at a missing remote, which errors and
	// triggers os.Exit. So we cover the case indirectly through runWiki only.)
	t.Setenv("GITHUB_DOCS_CONFIG", "")
	t.Setenv("GITHUB_API_URL", "https://api.github.com")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_REPO", "o/r")
	if err := runWiki([]string{"list"}); err == nil {
		t.Error("expected token error")
	}
}
