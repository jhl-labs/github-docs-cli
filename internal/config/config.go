// Package config loads github-docs-cli settings from a config file and
// environment variables. Command-line flags layer on top of the result.
//
// Precedence (low -> high): config file < environment variables < flags.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config holds the connection, auth, and default-repository settings used to
// talk to a GitHub (or GitHub Enterprise) instance.
type Config struct {
	// APIURL is the REST API base, e.g. https://api.github.com (the default)
	// or https://ghe.example.com/api/v3 for GitHub Enterprise Server.
	APIURL string `json:"api_url"`
	// Token is a Personal Access Token or GITHUB_TOKEN used as a Bearer token.
	Token string `json:"token"`
	// Repo is the default "owner/name" repository used by commands that take
	// --repo when the flag is not given.
	Repo string `json:"repo"`
	// Insecure skips TLS certificate verification (internal CA / GHE only).
	Insecure bool `json:"insecure"`
}

// DefaultAPIURL is the public GitHub REST API base.
const DefaultAPIURL = "https://api.github.com"

// Env var names.
const (
	EnvAPIURL   = "GITHUB_API_URL"
	EnvToken    = "GITHUB_TOKEN"
	EnvGHToken  = "GH_TOKEN"
	EnvRepo     = "GITHUB_REPO"
	EnvInsecure = "GITHUB_INSECURE"
	EnvConfig   = "GITHUB_DOCS_CONFIG"
)

// Load reads the config file (if any) and overlays environment variables.
// A missing config file is not an error; a malformed one is.
func Load() (Config, error) {
	var c Config

	path, err := configFilePath()
	if err != nil {
		return c, err
	}
	if path != "" {
		if err := loadFile(path, &c); err != nil {
			return c, err
		}
	}

	// Environment overrides file values when set.
	if v := os.Getenv(EnvAPIURL); v != "" {
		c.APIURL = v
	}
	if v := os.Getenv(EnvToken); v != "" {
		c.Token = v
	} else if v := os.Getenv(EnvGHToken); v != "" {
		c.Token = v
	}
	if v := os.Getenv(EnvRepo); v != "" {
		c.Repo = v
	}
	if v := os.Getenv(EnvInsecure); v != "" {
		c.Insecure = truthy(v)
	}

	if c.APIURL == "" {
		c.APIURL = DefaultAPIURL
	}
	c.APIURL = strings.TrimRight(strings.TrimSpace(c.APIURL), "/")
	c.Repo = strings.TrimSpace(c.Repo)
	return c, nil
}

// Validate ensures the config is usable for making requests.
func (c Config) Validate() error {
	if c.APIURL == "" {
		return errors.New("API URL is not set (use --api-url or " + EnvAPIURL + ")")
	}
	if !strings.HasPrefix(c.APIURL, "http://") && !strings.HasPrefix(c.APIURL, "https://") {
		return fmt.Errorf("API URL must start with http:// or https:// (got %q)", c.APIURL)
	}
	if c.Token == "" {
		return errors.New("no credentials: set a token (--token / " + EnvToken + " / " + EnvGHToken + ")")
	}
	return nil
}

// SplitRepo parses an "owner/name" string into its parts.
func SplitRepo(repo string) (owner, name string, err error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", "", errors.New("no repository set (use --repo owner/name or " + EnvRepo + ")")
	}
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository %q (want owner/name)", repo)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

// configFilePath returns the config file to read, or "" if none exists.
func configFilePath() (string, error) {
	if p := os.Getenv(EnvConfig); p != "" {
		return p, nil
	}
	dir, err := defaultConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "github-docs-cli", "config.json")
	if _, err := os.Stat(p); err != nil {
		return "", nil // no file -> no config, not an error
	}
	return p, nil
}

func defaultConfigDir() (string, error) {
	// os.UserConfigDir is cross-platform (XDG on Linux, AppData on Windows,
	// Application Support on macOS).
	dir, err := os.UserConfigDir()
	if err == nil {
		return dir, nil
	}
	// Fallback to home for unusual environments.
	home, herr := os.UserHomeDir()
	if herr != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return home, nil
	}
	return filepath.Join(home, ".config"), nil
}

func loadFile(path string, c *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("parsing config %s: %w", path, err)
	}
	return nil
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
