package main

import (
	"strings"
	"testing"
)

func TestBuildSkillContainsReference(t *testing.T) {
	for _, flavor := range []string{"", "generic", "claude", "codex", "gemini", "opencode"} {
		out := buildSkill(flavor)
		for _, want := range []string{
			"GITHUB_TOKEN",
			"github-docs-cli issue",
			"github-docs-cli docs put",
			"note add",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("flavor %q missing %q", flavor, want)
			}
		}
	}
}

func TestClaudeFlavorHasFrontmatter(t *testing.T) {
	out := buildSkill("claude")
	if !strings.HasPrefix(out, "---\nname: github-docs-cli\n") {
		t.Errorf("claude flavor should start with YAML frontmatter, got:\n%.80s", out)
	}
	if !strings.Contains(out, "description:") {
		t.Error("claude frontmatter missing description")
	}
}

func TestNonClaudeFlavorsHaveNoFrontmatter(t *testing.T) {
	for _, flavor := range []string{"", "generic", "codex", "gemini", "opencode"} {
		if strings.HasPrefix(buildSkill(flavor), "---") {
			t.Errorf("flavor %q should not have YAML frontmatter", flavor)
		}
	}
}

func TestFlavorNamesExcludesEmpty(t *testing.T) {
	names := flavorNames()
	want := map[string]bool{"claude": true, "codex": true, "gemini": true, "opencode": true}
	if len(names) != len(want) {
		t.Fatalf("flavorNames = %v", names)
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected flavor %q", n)
		}
	}
}

func TestWorkflowFlavorsHaveContent(t *testing.T) {
	for _, fl := range workflowFlavors() {
		tmpl := workflowTemplates[fl]
		if tmpl.filename == "" || !strings.HasSuffix(tmpl.filename, ".yml") {
			t.Errorf("flavor %q has bad filename %q", fl, tmpl.filename)
		}
		if !strings.Contains(tmpl.content, "name:") {
			t.Errorf("flavor %q workflow missing a name:", fl)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" docs, decision ,, runbook ")
	want := []string{"docs", "decision", "runbook"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV len = %d (%v)", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if splitCSV("  ") != nil {
		t.Error("blank input should yield nil")
	}
}

func TestWikiFilenameRoundTrip(t *testing.T) {
	cases := map[string]string{
		"Home":       "Home.md",
		"My Page":    "My-Page.md",
		"Runbook.md": "Runbook.md",
		"A B C":      "A-B-C.md",
	}
	for page, file := range cases {
		if got := wikiFilename(page); got != file {
			t.Errorf("wikiFilename(%q) = %q, want %q", page, got, file)
		}
	}
	if got := wikiPageName("My-Page.md"); got != "My Page" {
		t.Errorf("wikiPageName = %q", got)
	}
}

func TestGitHost(t *testing.T) {
	cases := map[string]string{
		"https://api.github.com":         "github.com",
		"https://ghe.example.com/api/v3": "ghe.example.com",
		"https://ghe.example.com":        "ghe.example.com",
	}
	for in, want := range cases {
		if got := gitHost(in); got != want {
			t.Errorf("gitHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatNoteEntry(t *testing.T) {
	entry := formatNoteEntry("decision", "Use Go", "Single binary.", "2026-06-09T00:00:00Z")
	for _, want := range []string{"## [decision]", "2026-06-09T00:00:00Z", "Use Go", "Single binary."} {
		if !strings.Contains(entry, want) {
			t.Errorf("entry missing %q:\n%s", want, entry)
		}
	}
}
