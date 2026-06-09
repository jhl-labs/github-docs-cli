package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

// skillFlavors are the supported agent platforms. The empty string and
// "generic" both map to the universal, frontmatter-less skill.
var skillFlavors = map[string]bool{
	"":         true, // generic
	"generic":  true,
	"claude":   true,
	"codex":    true,
	"gemini":   true,
	"opencode": true,
}

func runGenerateSkill(args []string) error {
	// The flavor is an optional leading positional, e.g.
	//   generate-skill            -> generic
	//   generate-skill claude     -> claude
	//   generate-skill claude --stdout
	var flavor string
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		flavor, rest = args[0], args[1:]
	}

	fs := flag.NewFlagSet("generate-skill", flag.ExitOnError)
	var (
		out   = fs.String("out", "github-docs-skill.md", "output file path")
		toOut = fs.Bool("stdout", false, "write to stdout instead of a file")
		force = fs.Bool("force", false, "overwrite the output file if it exists")
	)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: github-docs-cli generate-skill [flavor] [flags]")
		fmt.Fprintf(fs.Output(), "\nFlavors: %s\n", strings.Join(flavorNames(), ", "))
		fmt.Fprintln(fs.Output(), "  (no flavor = generic / universal skill)")
		fs.PrintDefaults()
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if flavor == "" && fs.NArg() > 0 {
		flavor = fs.Arg(0)
	}

	if !skillFlavors[flavor] {
		return fmt.Errorf("unknown flavor %q (supported: %s)", flavor, strings.Join(flavorNames(), ", "))
	}

	content := buildSkill(flavor)

	if *toOut {
		_, err := os.Stdout.WriteString(content)
		return err
	}

	if !*force {
		if _, err := os.Stat(*out); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite or --stdout to print)", *out)
		}
	}
	if err := os.WriteFile(*out, []byte(content), 0o644); err != nil {
		return err
	}

	label := flavor
	if label == "" {
		label = "generic"
	}
	fmt.Printf("wrote %s (%s flavor, %d bytes)\n", *out, label, len(content))
	return nil
}

// flavorNames returns the explicit, sorted platform flavor names. The default
// flavor (no argument / "generic") is omitted since it is the implicit default.
func flavorNames() []string {
	names := make([]string, 0, len(skillFlavors))
	for k := range skillFlavors {
		if k == "" || k == "generic" {
			continue
		}
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// buildSkill renders the github-docs skill markdown for the given flavor.
// The platform-specific preamble is prepended to a shared command reference.
func buildSkill(flavor string) string {
	var preamble string
	switch flavor {
	case "", "generic":
		preamble = genericPreamble
	case "claude":
		preamble = claudePreamble
	case "codex":
		preamble = codexPreamble
	case "gemini":
		preamble = geminiPreamble
	case "opencode":
		preamble = opencodePreamble
	}
	return preamble + skillReference
}

const genericPreamble = `# GitHub Docs CLI Skill

> Universal skill describing how an AI agent should use the ` + "`github-docs-cli`" + ` tool.
> Place or import this file wherever your agent reads project instructions.

`

const claudePreamble = `---
name: github-docs-cli
description: Use a GitHub repository as a documentation tool via the github-docs-cli command-line tool — manage issues, discussions, the wiki, and the docs/ folder behind GitHub Pages, and record dev/research/decision notes. Use when the user wants to search, read, create, update, or organize repository docs, issues, discussions, or wiki pages.
---

# GitHub Docs CLI Skill

> **Install (Claude):** rename this file to ` + "`SKILL.md`" + ` and place it under a folder
> matching the skill name, e.g. ` + "`.claude/skills/github-docs-cli/SKILL.md`" + `
> (the folder name must match the ` + "`name`" + ` field above).

`

const codexPreamble = `# GitHub Docs CLI — Agent Instructions

> **Install (Codex):** append this content to your project ` + "`AGENTS.md`" + ` (or
> ` + "`~/.codex/AGENTS.md`" + ` for global use). Codex reads AGENTS.md as standard Markdown.

`

const geminiPreamble = `# GitHub Docs CLI — Context

> **Install (Gemini CLI):** add this content to ` + "`GEMINI.md`" + ` at your project root
> (or ` + "`~/.gemini/GEMINI.md`" + ` for global), or import it with ` + "`@github-docs-skill.md`" + `.

`

const opencodePreamble = `# GitHub Docs CLI — Agent Rules

> **Install (opencode):** opencode reads ` + "`AGENTS.md`" + `; append this content there,
> or reference this ` + "`github-docs-skill.md`" + ` from it.

`

// skillReference is the shared, platform-agnostic body: what the tool is, how
// to authenticate, the command reference, and usage guidance.
const skillReference = `## What this is

` + "`github-docs-cli`" + ` is a small, dependency-free CLI that turns a **GitHub repository into a
single documentation tool**. It wraps the GitHub REST and GraphQL APIs so an agent can manage
**issues**, **discussions**, the **wiki**, and the **docs/** folder published via **GitHub Pages**,
and can record **dev/research/decision notes** through a single ` + "`note`" + ` command. Prefer it
over calling the API directly: it handles auth, retries, pagination defaults, and Pages/Contents
plumbing.

## Setup (authentication)

Set these environment variables before invoking the CLI:

| Variable | Required | Purpose |
|---|---|---|
| ` + "`GITHUB_TOKEN`" + ` (or ` + "`GH_TOKEN`" + `) | yes | Personal Access Token / Actions token (Bearer auth) |
| ` + "`GITHUB_REPO`" + ` | yes | Default repository as ` + "`owner/name`" + ` (lets you omit ` + "`--repo`" + `) |
| ` + "`GITHUB_API_URL`" + ` | no | REST base; defaults to ` + "`https://api.github.com`" + ` (set for GitHub Enterprise) |

Token scopes: ` + "`repo`" + ` (issues/contents/pages), ` + "`write:discussion`" + ` for discussions.

## Commands

Every command prints **JSON by default** (ideal for parsing); add ` + "`--output text`" + ` for a
human-readable summary. Commands are grouped: ` + "`github-docs-cli <group> <action> [flags]`" + `.

### issue — issues as documents/notes
` + "```bash" + `
github-docs-cli issue list --state open --label docs --limit 20
github-docs-cli issue get --number 42 --comments
github-docs-cli issue create --title "Design doc" --body-file design.md --label docs
github-docs-cli issue update --number 42 --state closed
github-docs-cli issue comment --number 42 --body "Shipped in v1.2"
github-docs-cli issue label --number 42 --add "docs,decision" --remove "draft"
` + "```" + `

### discussion — long-form discussions (GraphQL)
` + "```bash" + `
github-docs-cli discussion categories
github-docs-cli discussion list --limit 20
github-docs-cli discussion get --number 7
github-docs-cli discussion create --category "Ideas" --title "RFC" --body-file rfc.md
github-docs-cli discussion comment --number 7 --body "Agreed."
` + "```" + `

### docs — files in the repo (the docs/ folder for Pages)
` + "```bash" + `
github-docs-cli docs tree --path docs
github-docs-cli docs list --path docs
github-docs-cli docs get --path docs/guide.md          # prints raw file
github-docs-cli docs put --path docs/guide.md --body-file guide.md --message "docs: update guide"
github-docs-cli docs delete --path docs/old.md --message "docs: remove old page"
` + "```" + `
` + "`docs put`" + ` creates or updates the file (it looks up the current blob SHA automatically).

### pages — GitHub Pages publishing
` + "```bash" + `
github-docs-cli pages get
github-docs-cli pages enable --path /docs       # build from the docs/ folder
github-docs-cli pages build                      # trigger a rebuild
` + "```" + `

### wiki — the repository wiki (requires the git binary)
` + "```bash" + `
github-docs-cli wiki list
github-docs-cli wiki get --page Home
github-docs-cli wiki set --page "Runbook" --body-file runbook.md
github-docs-cli wiki delete --page "Old Page"
` + "```" + `

### note — record dev/research/decision notes (agent hook)
` + "```bash" + `
# Default: append a timestamped entry to docs/notes/DEVLOG.md (committed via the API)
github-docs-cli note add --kind decision --title "Use Go" --body "Single static binary."
github-docs-cli note add --kind research --title "Bench" --body-file findings.md --docs docs/notes/research.md
github-docs-cli note add --kind dev --title "Refactor" --body "..." --issue 42        # as an issue comment
github-docs-cli note add --kind decision --title "ADR-1" --body-file adr.md --new-issue
` + "```" + `

### generate-workflow / generate-skill
` + "```bash" + `
github-docs-cli generate-workflow pages          # .github/workflows/pages.yml
github-docs-cli generate-workflow jekyll
github-docs-cli generate-skill claude --stdout
` + "```" + `

## Usage guidance for agents

- Use ` + "`--output json`" + ` when you need to parse results; ` + "`--output text`" + ` for quick checks.
- Issue/discussion/note/wiki bodies are **GitHub-Flavored Markdown** (not HTML).
- ` + "`docs put`" + ` resolves the current file SHA for you — just pass the new content.
- For agent hooks (recording decisions/research), use ` + "`note add`" + `; it defaults to appending a
  timestamped Markdown entry to ` + "`docs/notes/DEVLOG.md`" + `, or target an issue/discussion.
- Returned objects include URLs (` + "`html_url`" + ` / ` + "`url`" + `); surface them to the user when done.
- Never hardcode credentials; rely on ` + "`GITHUB_TOKEN`" + ` / ` + "`GITHUB_REPO`" + `.
- Confirm destructive or outward-facing writes (creating issues/discussions, deleting files or
  wiki pages on shared repos) with the user first.
`
