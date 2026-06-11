package config

import (
	"os"
	"path/filepath"
	"testing"
)

// clearEnv unsets every env var the loader reads, so each test starts clean.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{EnvAPIURL, EnvToken, EnvGHToken, EnvRepo, EnvInsecure, EnvConfig} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestLoadDefaultsAPIURL(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "none.json")) // missing -> treated as no file? no: explicit path errors
	// An explicit but missing config path is an error; point at a real empty file instead.
	empty := filepath.Join(t.TempDir(), "cfg.json")
	os.WriteFile(empty, []byte(`{}`), 0o644)
	t.Setenv(EnvConfig, empty)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIURL != DefaultAPIURL {
		t.Errorf("APIURL = %q, want default %q", cfg.APIURL, DefaultAPIURL)
	}
}

func TestLoadEnvOverridesFileAndTrims(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"api_url":"https://file.example.com/","token":"filetok","repo":"f/r","insecure":true}`), 0o644)
	t.Setenv(EnvConfig, cfgPath)
	t.Setenv(EnvAPIURL, "https://env.example.com/")
	t.Setenv(EnvRepo, "  o/r  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIURL != "https://env.example.com" { // env wins, trailing slash trimmed
		t.Errorf("APIURL = %q", cfg.APIURL)
	}
	if cfg.Token != "filetok" { // not overridden by env
		t.Errorf("Token = %q", cfg.Token)
	}
	if cfg.Repo != "o/r" { // trimmed
		t.Errorf("Repo = %q", cfg.Repo)
	}
	if !cfg.Insecure {
		t.Errorf("Insecure = false, want true (from file)")
	}
}

func TestLoadGHTokenFallback(t *testing.T) {
	clearEnv(t)
	empty := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(empty, []byte(`{}`), 0o644)
	t.Setenv(EnvConfig, empty)
	t.Setenv(EnvGHToken, "gh-fallback")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != "gh-fallback" {
		t.Errorf("Token = %q, want gh-fallback", cfg.Token)
	}

	// GITHUB_TOKEN takes precedence over GH_TOKEN.
	t.Setenv(EnvToken, "primary")
	cfg, _ = Load()
	if cfg.Token != "primary" {
		t.Errorf("Token = %q, want primary", cfg.Token)
	}
}

func TestLoadInsecureFromEnv(t *testing.T) {
	clearEnv(t)
	empty := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(empty, []byte(`{}`), 0o644)
	t.Setenv(EnvConfig, empty)
	t.Setenv(EnvInsecure, "yes")
	cfg, _ := Load()
	if !cfg.Insecure {
		t.Errorf("Insecure = false, want true")
	}
}

func TestLoadMalformedFileErrors(t *testing.T) {
	clearEnv(t)
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{not json`), 0o644)
	t.Setenv(EnvConfig, bad)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for malformed config")
	}
}

func TestLoadMissingExplicitConfigErrors(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "does-not-exist.json"))
	if _, err := Load(); err == nil {
		t.Fatal("expected error for missing explicit config path")
	}
}

func TestLoadNoConfigFileViaUserConfigDir(t *testing.T) {
	clearEnv(t)
	// Point os.UserConfigDir at an empty temp dir so no config file exists and
	// the default-path branch (stat miss -> "") is exercised.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("AppData", tmp)
	t.Setenv(EnvToken, "tok")
	t.Setenv(EnvRepo, "o/r")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != "tok" || cfg.Repo != "o/r" {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.APIURL != DefaultAPIURL {
		t.Errorf("APIURL = %q", cfg.APIURL)
	}
}

func TestLoadReadsDefaultPathConfig(t *testing.T) {
	clearEnv(t)
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("AppData", tmp)
	dir := filepath.Join(tmp, "github-docs-cli")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"token":"filetok","repo":"a/b"}`), 0o644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != "filetok" || cfg.Repo != "a/b" {
		t.Errorf("cfg = %+v, want token=filetok repo=a/b", cfg)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"ok", Config{APIURL: "https://api.github.com", Token: "t"}, false},
		{"no url", Config{Token: "t"}, true},
		{"bad scheme", Config{APIURL: "ftp://x", Token: "t"}, true},
		{"no token", Config{APIURL: "https://api.github.com"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSplitRepo(t *testing.T) {
	tests := []struct {
		in          string
		owner, repo string
		wantErr     bool
	}{
		{"jhl-labs/github-docs-cli", "jhl-labs", "github-docs-cli", false},
		{"o/r.git", "o", "r", false},
		{"", "", "", true},
		{"noslash", "", "", true},
		{"/r", "", "", true},
		{"o/", "", "", true},
	}
	for _, tt := range tests {
		owner, repo, err := SplitRepo(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("SplitRepo(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && (owner != tt.owner || repo != tt.repo) {
			t.Errorf("SplitRepo(%q) = %q,%q want %q,%q", tt.in, owner, repo, tt.owner, tt.repo)
		}
	}
}

func TestTruthy(t *testing.T) {
	for _, v := range []string{"1", "true", "YES", "on", " On "} {
		if !truthy(v) {
			t.Errorf("truthy(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"0", "false", "no", "", "x"} {
		if truthy(v) {
			t.Errorf("truthy(%q) = true, want false", v)
		}
	}
}
