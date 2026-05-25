package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/roramirez/anprr/internal/config"
	"github.com/roramirez/anprr/internal/demo"
	"github.com/roramirez/anprr/internal/github"
	"github.com/roramirez/anprr/internal/tui"
)

var version = "dev"

const usage = `anprr — GitHub PR review TUI

Usage:
  anprr [flags]                        Launch TUI
  anprr login --token <token>          Save token to config
  anprr repos list                     List tracked repos
  anprr repos add <owner/repo>         Add a repo
  anprr repos remove <owner/repo>      Remove a repo
  anprr scopes list                    List configured scopes
  anprr version                        Show version
  anprr help                           Show this help

Flags:
  --scope <name>     Use a named scope (personal, work, etc.)
  --token <token>    GitHub personal access token (overrides config and GITHUB_TOKEN)
  --no-syntax        Disable syntax highlighting in diffs (enabled by default)

Config file: ~/.config/anprr/config.toml
  token     = "ghp_xxxx"
  repos     = ["owner/repo"]
  no-syntax = false

  [scopes.work]
  token = "ghp_work_token"
  repos = ["myorg/backend", "myorg/frontend"]

  [scopes.personal]
  token = "ghp_personal_token"
  repos = ["me/project1"]
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Println(version)
			os.Exit(0)
		case "help", "--help", "-h":
			fmt.Print(usage)
			os.Exit(0)
		}
	}

	// Parse global flags first; flag stops at the first non-flag argument,
	// so subcommands in remaining args are untouched.
	fs := flag.NewFlagSet("anprr", flag.ExitOnError)
	flagToken := fs.String("token", "", "GitHub personal access token")
	flagNoSyntax := fs.Bool("no-syntax", false, "Disable syntax highlighting in diffs")
	flagDemo := fs.Bool("demo", false, "Run with mock data (no token required)")
	flagScope := fs.String("scope", "", "Named scope to use (e.g. work, personal)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if remaining := fs.Args(); len(remaining) > 0 {
		switch remaining[0] {
		case "login":
			cmdLogin(remaining[1:], *flagScope)
			return
		case "repos":
			cmdRepos(remaining[1:], *flagScope)
			return
		case "scopes":
			cmdScopes(remaining[1:])
			return
		}
	}

	client, cache, repos, syntaxHL := mustResolveApp(*flagDemo, *flagToken, *flagNoSyntax, *flagScope)

	app := tui.NewApp(client, cache, repos, syntaxHL)

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func cmdLogin(args []string, scope string) {
	fs := flag.NewFlagSet("anprr login", flag.ExitOnError)
	flagToken := fs.String("token", "", "GitHub personal access token")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	token := *flagToken
	if token == "" && len(fs.Args()) > 0 {
		token = fs.Args()[0]
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "Usage: anprr login --token <token>")
		os.Exit(1)
	}

	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if scope != "" {
		s := cfg.Scopes[scope]
		s.Token = token
		config.SetScope(cfg, scope, s)
	} else {
		cfg.Token = token
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Token saved.")
}

func cmdRepos(args []string, scope string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: anprr repos list|add|remove <owner/repo>")
		os.Exit(1)
	}

	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		cmdReposList(cfg, scope)
	case "add":
		cmdReposAdd(args[1:], cfgPath, cfg, scope)
	case "remove":
		cmdReposRemove(args[1:], cfgPath, cfg, scope)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: anprr repos %s\n", args[0])
		os.Exit(1)
	}
}

func cmdReposList(cfg *config.Config, scope string) {
	repos := cfg.Repos
	if scope != "" {
		s, ok := cfg.Scopes[scope]
		if !ok {
			fmt.Fprintf(os.Stderr, "scope %q not defined in config\n", scope)
			os.Exit(1)
		}
		repos = s.Repos
	}
	if len(repos) == 0 {
		fmt.Println("(none)")
		return
	}
	for _, r := range repos {
		fmt.Println(r)
	}
}

func cmdReposAdd(args []string, cfgPath string, cfg *config.Config, scope string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: anprr repos add [--scope <name>] <owner/repo>")
		os.Exit(1)
	}
	repo := args[0]
	if err := config.ValidateRepo(repo); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if scope != "" {
		s := cfg.Scopes[scope]
		s.Repos = append(s.Repos, repo)
		config.SetScope(cfg, scope, s)
	} else {
		cfg.Repos = append(cfg.Repos, repo)
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Added %s.\n", repo)
}

func mustResolveApp(isDemo bool, flagToken string, flagNoSyntax bool, scope string) (*github.Client, *github.Cache, []string, bool) {
	if isDemo {
		return github.NewClient("demo-token", demo.Transport{}), github.NewCache(), []string{"acme/backend"}, true
	}
	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	effective, err := config.ResolveScope(cfg, scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	token := config.ResolveToken(flagToken, effective.Token)
	if token == "" {
		fmt.Fprintln(os.Stderr, "No token configured. Run: anprr login --token <token>")
		os.Exit(1)
	}
	if len(effective.Repos) == 0 {
		fmt.Fprintln(os.Stderr, "No repositories configured. Run: anprr repos add <owner/repo>")
		os.Exit(1)
	}
	return github.NewClient(token, nil), github.NewCache(), effective.Repos, !effective.NoSyntax && !flagNoSyntax
}

func cmdReposRemove(args []string, cfgPath string, cfg *config.Config, scope string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: anprr repos remove [--scope <name>] <owner/repo>")
		os.Exit(1)
	}
	repo := args[0]

	if scope != "" {
		s, ok := cfg.Scopes[scope]
		if !ok {
			fmt.Fprintf(os.Stderr, "scope %q not defined in config\n", scope)
			os.Exit(1)
		}
		var kept []string
		for _, r := range s.Repos {
			if !strings.EqualFold(r, repo) {
				kept = append(kept, r)
			}
		}
		if len(kept) == len(s.Repos) {
			fmt.Printf("%s not found.\n", repo)
			return
		}
		s.Repos = kept
		config.SetScope(cfg, scope, s)
	} else {
		var kept []string
		for _, r := range cfg.Repos {
			if !strings.EqualFold(r, repo) {
				kept = append(kept, r)
			}
		}
		if len(kept) == len(cfg.Repos) {
			fmt.Printf("%s not found.\n", repo)
			return
		}
		cfg.Repos = kept
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed %s.\n", repo)
}

func cmdScopes(args []string) {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(os.Stderr, "Usage: anprr scopes list")
		os.Exit(1)
	}
	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(cfg.Scopes) == 0 {
		fmt.Println("(none)")
		return
	}
	for name := range cfg.Scopes {
		fmt.Println(name)
	}
}
