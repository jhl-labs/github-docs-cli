# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-09

### Added
- Initial release: use a GitHub repository as a single documentation tool.
- `issue` — list / get / create / update / comment / label.
- `discussion` — list / get / create / comment / categories (GraphQL API).
- `docs` — list / tree / get / put / delete files via the Contents API
  (manage the `docs/` folder for GitHub Pages).
- `pages` — get / enable / build GitHub Pages.
- `wiki` — list / get / set / delete pages (clones the `<repo>.wiki.git` repo;
  requires the `git` binary).
- `note` — record dev/research/decision notes (agent hook): append to a
  Markdown log, or post to an issue/discussion, or open a new issue.
- `generate-skill` — emit an agent skill doc for claude / codex / gemini /
  opencode / generic.
- `generate-workflow` — emit a GitHub Actions workflow (pages / jekyll /
  link-check).
- Authentication via a token (Bearer); configuration from flags, environment
  variables, or a config file. `GITHUB_REPO` default repository.
- Supports GitHub.com and GitHub Enterprise Server (`GITHUB_API_URL`).
- Resilient HTTP client: retries on 429/5xx and secondary rate limits honoring
  `Retry-After`.
- JSON-first output with `--output text` for human-readable summaries.
- Zero external Go dependencies; cross-platform single binary.

[Unreleased]: https://github.com/jhl-labs/github-docs-cli/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jhl-labs/github-docs-cli/releases/tag/v0.1.0
