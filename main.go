// Command github-docs-cli turns a GitHub repository into a single documentation
// tool for AI agents and automation: issues, discussions, the wiki, and the
// docs/ folder behind GitHub Pages, plus workflow and skill generation.
package main

import (
	"fmt"
	"os"
)

// version is overridable at build time:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "issue":
		err = runIssue(args)
	case "discussion":
		err = runDiscussion(args)
	case "wiki":
		err = runWiki(args)
	case "docs":
		err = runDocs(args)
	case "pages":
		err = runPages(args)
	case "note":
		err = runNote(args)
	case "generate-skill":
		err = runGenerateSkill(args)
	case "generate-workflow":
		err = runGenerateWorkflow(args)
	case "version", "-v", "--version":
		fmt.Printf("github-docs-cli %s\n", version)
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage(w *os.File) {
	fmt.Fprintf(w, `github-docs-cli %s — use a GitHub repo as a documentation tool

Usage:
  github-docs-cli <command> <action> [flags]

Commands:
  issue       Manage issues       (list | get | create | update | comment | label)
  discussion  Manage discussions  (list | get | create | comment | categories)
  wiki        Manage the wiki      (list | get | set | delete) — requires git
  docs        Manage docs/ files   (list | tree | get | put | delete)
  pages       Manage GitHub Pages  (get | enable | build)
  note        Record a dev/research/decision note to the repo (agent hook)
  generate-skill     Write a skill file for an AI agent
                     (flavors: claude, codex, gemini, opencode; none = generic)
  generate-workflow  Write a GitHub Actions workflow
                     (flavors: pages, jekyll, link-check)
  version     Print version
  help        Show this help

Run "github-docs-cli <command>" with no action for command-specific help.

Authentication:
  Token (PAT or GITHUB_TOKEN):  --token / GITHUB_TOKEN / GH_TOKEN
  Default repository:           --repo owner/name / GITHUB_REPO
  API base URL (GHE):           --api-url / GITHUB_API_URL (default api.github.com)
`, version)
}
