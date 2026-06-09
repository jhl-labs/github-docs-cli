# GitHub Docs CLI — Agent Instructions

> **Install (Codex):** append this content to your project `AGENTS.md` (or
> `~/.codex/AGENTS.md` for global use). Codex reads AGENTS.md as standard Markdown.

## What this is

`github-docs-cli` is a small, dependency-free CLI that turns a **GitHub repository into a
single documentation tool**. It wraps the GitHub REST and GraphQL APIs so an agent can manage
**issues**, **discussions**, the **wiki**, and the **docs/** folder published via **GitHub Pages**,
and can record **dev/research/decision notes** through a single `note` command. Prefer it
over calling the API directly: it handles auth, retries, pagination defaults, and Pages/Contents
plumbing.

## Setup (authentication)

Set these environment variables before invoking the CLI:

| Variable | Required | Purpose |
|---|---|---|
| `GITHUB_TOKEN` (or `GH_TOKEN`) | yes | Personal Access Token / Actions token (Bearer auth) |
| `GITHUB_REPO` | yes | Default repository as `owner/name` (lets you omit `--repo`) |
| `GITHUB_API_URL` | no | REST base; defaults to `https://api.github.com` (set for GitHub Enterprise) |

Token scopes: `repo` (issues/contents/pages), `write:discussion` for discussions.

## Commands

Every command prints **JSON by default** (ideal for parsing); add `--output text` for a
human-readable summary. Commands are grouped: `github-docs-cli <group> <action> [flags]`.

### issue — issues as documents/notes
```bash
github-docs-cli issue list --state open --label docs --limit 20
github-docs-cli issue get --number 42 --comments
github-docs-cli issue create --title "Design doc" --body-file design.md --label docs
github-docs-cli issue update --number 42 --state closed
github-docs-cli issue comment --number 42 --body "Shipped in v1.2"
github-docs-cli issue label --number 42 --add "docs,decision" --remove "draft"
```

### discussion — long-form discussions (GraphQL)
```bash
github-docs-cli discussion categories
github-docs-cli discussion list --limit 20
github-docs-cli discussion get --number 7
github-docs-cli discussion create --category "Ideas" --title "RFC" --body-file rfc.md
github-docs-cli discussion comment --number 7 --body "Agreed."
```

### docs — files in the repo (the docs/ folder for Pages)
```bash
github-docs-cli docs tree --path docs
github-docs-cli docs list --path docs
github-docs-cli docs get --path docs/guide.md          # prints raw file
github-docs-cli docs put --path docs/guide.md --body-file guide.md --message "docs: update guide"
github-docs-cli docs delete --path docs/old.md --message "docs: remove old page"
```
`docs put` creates or updates the file (it looks up the current blob SHA automatically).

### pages — GitHub Pages publishing
```bash
github-docs-cli pages get
github-docs-cli pages enable --path /docs       # build from the docs/ folder
github-docs-cli pages build                      # trigger a rebuild
```

### wiki — the repository wiki (requires the git binary)
```bash
github-docs-cli wiki list
github-docs-cli wiki get --page Home
github-docs-cli wiki set --page "Runbook" --body-file runbook.md
github-docs-cli wiki delete --page "Old Page"
```

### note — record dev/research/decision notes (agent hook)
```bash
# Default: append a timestamped entry to docs/notes/DEVLOG.md (committed via the API)
github-docs-cli note add --kind decision --title "Use Go" --body "Single static binary."
github-docs-cli note add --kind research --title "Bench" --body-file findings.md --docs docs/notes/research.md
github-docs-cli note add --kind dev --title "Refactor" --body "..." --issue 42        # as an issue comment
github-docs-cli note add --kind decision --title "ADR-1" --body-file adr.md --new-issue
```

### generate-workflow / generate-skill
```bash
github-docs-cli generate-workflow pages          # .github/workflows/pages.yml
github-docs-cli generate-workflow jekyll
github-docs-cli generate-skill claude --stdout
```

## Usage guidance for agents

- Use `--output json` when you need to parse results; `--output text` for quick checks.
- Issue/discussion/note/wiki bodies are **GitHub-Flavored Markdown** (not HTML).
- `docs put` resolves the current file SHA for you — just pass the new content.
- For agent hooks (recording decisions/research), use `note add`; it defaults to appending a
  timestamped Markdown entry to `docs/notes/DEVLOG.md`, or target an issue/discussion.
- Returned objects include URLs (`html_url` / `url`); surface them to the user when done.
- Never hardcode credentials; rely on `GITHUB_TOKEN` / `GITHUB_REPO`.
- Confirm destructive or outward-facing writes (creating issues/discussions, deleting files or
  wiki pages on shared repos) with the user first.
