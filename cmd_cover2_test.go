package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommandsJSONMode runs commands in default (JSON) mode to cover the
// json-encode branch of emit across the command set.
func TestCommandsJSONMode(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	cases := []struct {
		fn   func([]string) error
		args []string
	}{
		{runIssue, []string{"get", "--number", "7"}},
		{runIssue, []string{"update", "--number", "7", "--state", "closed"}},
		{runIssue, []string{"comment", "--number", "7", "--body", "x"}},
		{runIssue, []string{"label", "--number", "7", "--add", "a"}},
		{runDocs, []string{"list", "--path", "docs"}},
		{runDocs, []string{"tree", "--path", "docs"}},
		{runDocs, []string{"put", "--path", "docs/a.md", "--body", "x", "--message", "m", "--branch", "main"}},
		{runDocs, []string{"delete", "--path", "docs/a.md", "--message", "m"}},
		{runPages, []string{"get"}},
		{runPages, []string{"enable", "--branch", "main"}},
		{runPages, []string{"build"}},
		{runDiscussion, []string{"list"}},
		{runDiscussion, []string{"get", "--number", "7"}},
		{runDiscussion, []string{"categories"}},
		{runDiscussion, []string{"create", "--category", "Ideas", "--title", "T", "--body", "b"}},
		{runDiscussion, []string{"comment", "--number", "7", "--body", "ok"}},
		{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x", "--issue", "7"}},
		{runNote, []string{"add", "--kind", "dev", "--title", "T", "--body", "x", "--new-issue"}},
		{runNote, []string{"add", "--kind", "research", "--title", "T", "--body", "x", "--discussion", "7"}},
	}
	for _, c := range cases {
		out, err := runCmd(t, srv.URL, c.fn, c.args...)
		if err != nil {
			t.Errorf("args %v: %v", c.args, err)
		}
		if !strings.Contains(out, "{") && !strings.Contains(out, "[") {
			t.Errorf("args %v: expected JSON output, got %q", c.args, out)
		}
	}
}

func TestWikiTokenRequired(t *testing.T) {
	srv := mockGitHub(t)
	defer srv.Close()
	cfg := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(cfg, []byte(`{}`), 0o644)
	t.Setenv("GITHUB_DOCS_CONFIG", cfg)
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_REPO", "o/r")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	if err := runWiki([]string{"list"}); err == nil {
		t.Error("expected token-required error")
	}
}

func TestWikiCloneFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	srv := mockGitHub(t)
	defer srv.Close()
	orig := wikiRemoteURL
	wikiRemoteURL = func(common *commonFlags, owner, repo string) string {
		return filepath.Join(t.TempDir(), "does-not-exist.git")
	}
	defer func() { wikiRemoteURL = orig }()
	if _, err := runCmd(t, srv.URL, runWiki, "list"); err == nil {
		t.Error("expected clone failure error")
	}
}

func TestWikiNothingToCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = dir, gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	bare := filepath.Join(t.TempDir(), "wiki.git")
	git("", "init", "--bare", "-b", "master", bare)
	seed := t.TempDir()
	git("", "clone", bare, seed)
	os.WriteFile(filepath.Join(seed, "Home.md"), []byte("# Home\n"), 0o644)
	git(seed, "add", ".")
	git(seed, "commit", "-m", "init")
	git(seed, "push", "origin", "HEAD:master")

	orig := wikiRemoteURL
	wikiRemoteURL = func(common *commonFlags, owner, repo string) string { return bare }
	defer func() { wikiRemoteURL = orig }()

	srv := mockGitHub(t)
	defer srv.Close()
	// Writing the identical content -> "nothing to commit" branch (no error).
	out, err := runCmd(t, srv.URL, runWiki, "set", "--page", "Home", "--body", "# Home\n", "--output", "text")
	if err != nil {
		t.Fatalf("wiki set identical: %v", err)
	}
	if !strings.Contains(out, "saved") {
		t.Errorf("output %q", out)
	}
}

func TestReadBodyStdin(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	w.WriteString("from stdin")
	w.Close()
	defer func() { os.Stdin = old }()
	got, err := readBody("", "-")
	if err != nil || got != "from stdin" {
		t.Errorf("readBody stdin = %q, %v", got, err)
	}
}

func TestGenerateSkillDefaultOutAndWorkflowDir(t *testing.T) {
	// Run generate-skill writing to a temp working dir so the default filename
	// path (no --out) is exercised without polluting the repo.
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	if err := runGenerateSkill([]string{}); err != nil {
		t.Fatalf("default skill: %v", err)
	}
	if _, err := os.Stat("github-docs-skill.md"); err != nil {
		t.Errorf("default skill file not written: %v", err)
	}
	if err := runGenerateWorkflow([]string{"link-check"}); err != nil {
		t.Fatalf("default workflow: %v", err)
	}
	if _, err := os.Stat(filepath.Join(".github", "workflows", "link-check.yml")); err != nil {
		t.Errorf("default workflow file not written: %v", err)
	}
}
