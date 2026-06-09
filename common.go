package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github-docs-cli/internal/config"
	"github-docs-cli/internal/github"
)

// commonFlags are the connection/auth/output flags shared by every command.
type commonFlags struct {
	apiURL   string
	token    string
	repo     string
	insecure bool
	output   string
	timeout  time.Duration
	retries  int
}

// registerCommon adds the shared flags to fs, defaulting to values loaded from
// the config file and environment. Flags therefore override env/file.
func registerCommon(fs *flag.FlagSet) (*commonFlags, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	c := &commonFlags{}
	fs.StringVar(&c.apiURL, "api-url", cfg.APIURL, "GitHub REST API base URL")
	fs.StringVar(&c.token, "token", cfg.Token, "auth token (PAT or GITHUB_TOKEN)")
	fs.StringVar(&c.repo, "repo", cfg.Repo, "target repository as owner/name")
	fs.BoolVar(&c.insecure, "insecure", cfg.Insecure, "skip TLS certificate verification")
	fs.StringVar(&c.output, "output", "json", "output format: json|text")
	fs.DurationVar(&c.timeout, "timeout", 30*time.Second, "HTTP request timeout")
	fs.IntVar(&c.retries, "retries", 3, "retry attempts on transient (429/5xx) errors")
	return c, nil
}

// client builds a GitHub client from the parsed common flags.
func (c *commonFlags) client() (*github.Client, error) {
	cfg := config.Config{
		APIURL:   c.apiURL,
		Token:    c.token,
		Repo:     c.repo,
		Insecure: c.insecure,
	}
	cl, err := github.New(cfg, c.timeout)
	if err != nil {
		return nil, err
	}
	cl.MaxRetries = c.retries
	return cl, nil
}

// ownerRepo resolves the target repository into its owner and name parts.
func (c *commonFlags) ownerRepo() (owner, repo string, err error) {
	return config.SplitRepo(c.repo)
}

// subUsage builds a per-group usage function that lists the available actions.
func subUsage(group, summary string, actions []string) func() {
	return func() {
		fmt.Fprintf(os.Stderr, "Usage: github-docs-cli %s <action> [flags]\n\n%s\n\nActions:\n", group, summary)
		for _, a := range actions {
			fmt.Fprintf(os.Stderr, "  %s\n", a)
		}
		fmt.Fprintf(os.Stderr, "\nRun \"github-docs-cli %s <action> -h\" for action-specific flags.\n", group)
	}
}

// dispatch routes "<group> <action> ..." to the matching handler.
func dispatch(group string, args []string, usage func(), actions map[string]func([]string) error) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("%s: missing action", group)
	}
	action := args[0]
	switch action {
	case "-h", "--help", "help":
		usage()
		return nil
	}
	h, ok := actions[action]
	if !ok {
		usage()
		return fmt.Errorf("%s: unknown action %q", group, action)
	}
	return h(args[1:])
}

// readBody resolves body content from a literal value, a file path, or stdin
// ("-"). file takes precedence over value when both are given.
func readBody(value, file string) (string, error) {
	switch {
	case file == "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading body file: %w", err)
		}
		return string(data), nil
	default:
		return value, nil
	}
}

// requireFlag returns a usage-style error for a missing required flag.
func requireFlag(name, value string) error {
	if value == "" {
		return fmt.Errorf("--%s is required", name)
	}
	return nil
}

// splitCSV splits a comma-separated list, trimming blanks.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
