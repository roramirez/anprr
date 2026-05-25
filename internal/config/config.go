package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Scope struct {
	Token    string   `toml:"token"`
	Repos    []string `toml:"repos"`
	NoSyntax bool     `toml:"no-syntax"`
}

type Config struct {
	Token    string           `toml:"token"`
	Repos    []string         `toml:"repos"`
	NoSyntax bool             `toml:"no-syntax"` // set to true to disable syntax highlighting (on by default)
	Scopes   map[string]Scope `toml:"scopes"`
}

func DefaultConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "anprr", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "anprr", "config.toml")
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	return cfg, nil
}

func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	out := cleanTOML(buf.String())
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(out)
	return err
}

// cleanTOML removes encoder artifacts: redundant [scopes] parent header, indentation,
// and zero-value fields that are noise when missing (empty strings, false booleans, empty slices).
func cleanTOML(s string) string {
	s = strings.ReplaceAll(s, "\n[scopes]\n", "\n")
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimLeft(line, " \t")
		switch line {
		case `token = ""`, `no-syntax = false`, `repos = []`:
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// ResolveToken returns the first non-empty token from: flagToken > GITHUB_TOKEN env > cfgToken.
func ResolveToken(flagToken, cfgToken string) string {
	if flagToken != "" {
		return flagToken
	}
	if env := os.Getenv("GITHUB_TOKEN"); env != "" {
		return env
	}
	return cfgToken
}

// ResolveScope returns the effective Scope for the given name, merging base config defaults.
// If scope is empty, returns the base config fields. Returns an error if the named scope does not exist.
func ResolveScope(cfg *Config, scope string) (*Scope, error) {
	base := &Scope{Token: cfg.Token, Repos: cfg.Repos, NoSyntax: cfg.NoSyntax}
	if scope == "" {
		return base, nil
	}
	s, ok := cfg.Scopes[scope]
	if !ok {
		return nil, fmt.Errorf("scope %q not defined in config", scope)
	}
	if s.Token == "" {
		s.Token = base.Token
	}
	if len(s.Repos) == 0 {
		s.Repos = base.Repos
	}
	return &s, nil
}

// SetScope creates or updates a named scope in cfg.
func SetScope(cfg *Config, scope string, s Scope) {
	if cfg.Scopes == nil {
		cfg.Scopes = make(map[string]Scope)
	}
	cfg.Scopes[scope] = s
}

// ValidateRepo returns an error if repo is not in "owner/name" format.
func ValidateRepo(repo string) error {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid repo %q: must be owner/name", repo)
	}
	return nil
}
