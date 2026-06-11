package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- generate-skill ---

func TestGenerateSkillFileAndStdout(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "skill.md")

	if err := runGenerateSkill([]string{"claude", "--out", out}); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, _ := os.ReadFile(out)
	if !strings.HasPrefix(string(data), "---\nname: github-docs-cli") {
		t.Errorf("claude skill missing frontmatter")
	}
	// Existing file without --force errors.
	if err := runGenerateSkill([]string{"claude", "--out", out}); err == nil {
		t.Error("expected error overwriting without --force")
	}
	// --force overwrites.
	if err := runGenerateSkill([]string{"codex", "--out", out, "--force"}); err != nil {
		t.Fatalf("force: %v", err)
	}
	// Unknown flavor.
	if err := runGenerateSkill([]string{"bogus", "--stdout"}); err == nil {
		t.Error("expected error for unknown flavor")
	}
}

func TestGenerateSkillStdout(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runGenerateSkill([]string{"--stdout"})
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	buf := make([]byte, 64)
	r.Read(buf)
	if !strings.Contains(string(buf), "GitHub Docs CLI") {
		t.Errorf("stdout output unexpected: %q", buf)
	}
}

func TestGenerateSkillFlavorAfterFlags(t *testing.T) {
	// Flavor given after a flag is resolved from the remaining positional args.
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runGenerateSkill([]string{"--stdout", "claude"})
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("flavor-after-flags: %v", err)
	}
}

func TestGenerateWorkflowFlavorAfterFlags(t *testing.T) {
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := runGenerateWorkflow([]string{"--stdout", "jekyll"})
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("flavor-after-flags: %v", err)
	}
}

// --- generate-workflow ---

func TestGenerateWorkflow(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "wf.yml")
	if err := runGenerateWorkflow([]string{"pages", "--out", out}); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), "deploy-pages") {
		t.Errorf("pages workflow missing deploy step")
	}
	if err := runGenerateWorkflow([]string{"pages", "--out", out}); err == nil {
		t.Error("expected error without --force")
	}
	if err := runGenerateWorkflow([]string{"jekyll", "--stdout"}); err != nil {
		t.Fatalf("stdout: %v", err)
	}
	if err := runGenerateWorkflow([]string{"bogus", "--stdout"}); err == nil {
		t.Error("expected error for unknown flavor")
	}
	if err := runGenerateWorkflow([]string{}); err == nil {
		t.Error("expected error when no flavor given")
	}
}

// --- wiki (against a local bare repo via the wikiRemoteURL seam) ---

func TestWikiCommands(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	// Bare "remote" wiki repo, initialized with a Home page.
	bare := filepath.Join(t.TempDir(), "wiki.git")
	run("", "init", "--bare", "-b", "master", bare)
	seed := t.TempDir()
	run("", "clone", bare, seed)
	os.WriteFile(filepath.Join(seed, "Home.md"), []byte("# Home\n"), 0o644)
	run(seed, "add", ".")
	run(seed, "commit", "-m", "init")
	run(seed, "push", "origin", "HEAD:master")

	// Point the CLI's wiki clone URL at the bare repo.
	orig := wikiRemoteURL
	wikiRemoteURL = func(common *commonFlags, owner, repo string) string { return bare }
	defer func() { wikiRemoteURL = orig }()

	srv := mockGitHub(t)
	defer srv.Close()

	out, err := runCmd(t, srv.URL, runWiki, "list", "--output", "text")
	if err != nil || !strings.Contains(out, "Home") {
		t.Fatalf("wiki list: %v %q", err, out)
	}
	if out, err := runCmd(t, srv.URL, runWiki, "get", "--page", "Home"); err != nil || !strings.Contains(out, "# Home") {
		t.Fatalf("wiki get: %v %q", err, out)
	}
	if out, err := runCmd(t, srv.URL, runWiki, "set", "--page", "Runbook", "--body", "# RB\n", "--output", "text"); err != nil || !strings.Contains(out, "saved") {
		t.Fatalf("wiki set: %v %q", err, out)
	}
	if out, err := runCmd(t, srv.URL, runWiki, "delete", "--page", "Runbook", "--output", "text"); err != nil || !strings.Contains(out, "deleted") {
		t.Fatalf("wiki delete: %v %q", err, out)
	}
	// Deleting a non-existent page errors.
	if _, err := runCmd(t, srv.URL, runWiki, "delete", "--page", "Nope"); err == nil {
		t.Error("expected error deleting missing page")
	}
	// Getting a non-existent page errors.
	if _, err := runCmd(t, srv.URL, runWiki, "get", "--page", "Nope"); err == nil {
		t.Error("expected error getting missing page")
	}
}

// --- main / usage / output helpers ---

func TestRunExitCodes(t *testing.T) {
	for _, arg := range []string{"version", "help", "-v", "--help"} {
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w
		code := run([]string{arg})
		w.Close()
		os.Stdout = old
		if code != 0 {
			t.Errorf("run(%q) = %d, want 0", arg, code)
		}
	}
	// No args and unknown command both return usage exit code 2.
	if code := run(nil); code != 2 {
		t.Errorf("run(nil) = %d, want 2", code)
	}
	if code := run([]string{"bogus"}); code != 2 {
		t.Errorf("run(bogus) = %d, want 2", code)
	}
	// A command that returns an error maps to exit code 1.
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{not json`), 0o644)
	t.Setenv("GITHUB_DOCS_CONFIG", bad)
	if code := run([]string{"issue", "list"}); code != 1 {
		t.Errorf("run(failing) = %d, want 1", code)
	}
}

func TestUsageWrites(t *testing.T) {
	r, w, _ := os.Pipe()
	usage(w)
	w.Close()
	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	if !strings.Contains(string(buf[:n]), "github-docs-cli") {
		t.Errorf("usage output unexpected")
	}
}

func TestEmitUnknownFormat(t *testing.T) {
	if err := emit("xml", nil, func() {}); err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestReadBodyFileAndStdin(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "b.txt")
	os.WriteFile(f, []byte("filebody"), 0o644)
	if got, _ := readBody("ignored", f); got != "filebody" {
		t.Errorf("readBody file = %q", got)
	}
	if got, _ := readBody("literal", ""); got != "literal" {
		t.Errorf("readBody literal = %q", got)
	}
	if _, err := readBody("", "/no/such/file"); err == nil {
		t.Error("expected error for missing body file")
	}
}
