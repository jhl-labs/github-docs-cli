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
	os.Exit(run(os.Args[1:]))
}

// run dispatches a single command and returns the process exit code. It is
// separated from main so it can be unit-tested without os.Exit.
func run(args []string) int {
	if len(args) < 1 {
		usage(os.Stderr)
		return 2
	}

	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "issue":
		err = runIssue(rest)
	case "discussion":
		err = runDiscussion(rest)
	case "wiki":
		err = runWiki(rest)
	case "docs":
		err = runDocs(rest)
	case "pages":
		err = runPages(rest)
	case "note":
		err = runNote(rest)
	case "generate-skill":
		err = runGenerateSkill(rest)
	case "generate-workflow":
		err = runGenerateWorkflow(rest)
	case "version", "-v", "--version":
		fmt.Printf("github-docs-cli %s\n", version)
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage(os.Stderr)
		return 2
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
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
