package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_missing(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cfg.Token != "" || len(cfg.Repos) != 0 {
		t.Fatalf("expected zero config, got %+v", cfg)
	}
}

func TestLoad_valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `token = "ghp_test"
repos = ["owner/repo1", "owner/repo2"]
no-syntax = true
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "ghp_test" {
		t.Errorf("token: got %q, want %q", cfg.Token, "ghp_test")
	}
	if len(cfg.Repos) != 2 || cfg.Repos[0] != "owner/repo1" {
		t.Errorf("repos: got %v", cfg.Repos)
	}
	if !cfg.NoSyntax {
		t.Error("expected no-syntax=true")
	}
}

func TestSave_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.toml")
	orig := &Config{Token: "tok", Repos: []string{"a/b"}, NoSyntax: false}
	if err := Save(path, orig); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Token != orig.Token || len(loaded.Repos) != 1 || loaded.Repos[0] != "a/b" {
		t.Errorf("roundtrip mismatch: got %+v", loaded)
	}
}

func TestResolveToken(t *testing.T) {
	cfg := &Config{Token: "file_token"}

	// flag wins
	if got := ResolveToken("flag_token", cfg); got != "flag_token" {
		t.Errorf("flag: got %q", got)
	}

	// env wins over file
	t.Setenv("GITHUB_TOKEN", "env_token")
	if got := ResolveToken("", cfg); got != "env_token" {
		t.Errorf("env: got %q", got)
	}

	// file fallback
	t.Setenv("GITHUB_TOKEN", "")
	if got := ResolveToken("", cfg); got != "file_token" {
		t.Errorf("file: got %q", got)
	}
}

func TestDefaultConfigPath_xdg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	path := DefaultConfigPath()
	if path != "/tmp/xdg/anprr/config.toml" {
		t.Errorf("got %q", path)
	}
}

func TestDefaultConfigPath_noXdg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	path := DefaultConfigPath()
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	if !strings.HasSuffix(path, "/.config/anprr/config.toml") {
		t.Errorf("got %q, expected suffix /.config/anprr/config.toml", path)
	}
}

func TestValidateRepo(t *testing.T) {
	cases := []struct {
		repo string
		ok   bool
	}{
		{"owner/repo", true},
		{"org/sub-repo", true},
		{"noslash", false},
		{"/noowner", false},
		{"noname/", false},
		{"", false},
	}
	for _, c := range cases {
		err := ValidateRepo(c.repo)
		if c.ok && err != nil {
			t.Errorf("ValidateRepo(%q): unexpected error %v", c.repo, err)
		}
		if !c.ok && err == nil {
			t.Errorf("ValidateRepo(%q): expected error", c.repo)
		}
	}
}
