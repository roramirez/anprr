package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Token  string   `toml:"token"`
	Repos  []string `toml:"repos"`
	Syntax bool     `toml:"syntax"`
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
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// ResolveToken returns the first non-empty token from: flagToken > GITHUB_TOKEN env > cfg.Token.
func ResolveToken(flagToken string, cfg *Config) string {
	if flagToken != "" {
		return flagToken
	}
	if env := os.Getenv("GITHUB_TOKEN"); env != "" {
		return env
	}
	return cfg.Token
}

// ValidateRepo returns an error if repo is not in "owner/name" format.
func ValidateRepo(repo string) error {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid repo %q: must be owner/name", repo)
	}
	return nil
}
