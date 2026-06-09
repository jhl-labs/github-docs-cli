# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Report privately via GitHub Security Advisories:
<https://github.com/jhl-labs/github-docs-cli/security/advisories/new>

Include reproduction steps and affected versions where possible. When sharing
logs or configuration, **redact tokens and internal hostnames**.

## Supported versions

The latest released version receives security fixes. Older versions are not
maintained.

## Handling secrets

github-docs-cli authenticates with GitHub using a token (Personal Access Token
or `GITHUB_TOKEN`). To keep it safe:

- Provide the token via environment variables (`GITHUB_TOKEN` / `GH_TOKEN`) or a
  config file outside version control — never hardcode it.
- Keep config files (e.g. `~/.config/github-docs-cli/config.json`) readable only
  by your user.
- The token is sent only to the configured `GITHUB_API_URL` over HTTPS.
- Prefer fine-grained tokens scoped to the specific repository and the minimum
  permissions needed (issues, contents, pages, discussions).
- For `wiki` commands the token is embedded in the clone URL passed to `git`;
  it is not written to disk, but rotate any token that may have been exposed
  (logs, shell history, screen sharing) immediately.
