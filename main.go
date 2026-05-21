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
  anprr version                        Show version
  anprr help                           Show this help

Flags:
  --token <token>    GitHub personal access token (overrides config and GITHUB_TOKEN)
  --no-syntax        Disable syntax highlighting in diffs (enabled by default)

Config file: ~/.config/anprr/config.toml
  token     = "ghp_xxxx"
  repos     = ["owner/repo"]
  no-syntax = false   # set to true to disable syntax highlighting
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
		case "login":
			cmdLogin(os.Args[2:])
			return
		case "repos":
			cmdRepos(os.Args[2:])
			return
		}
	}

	// launch TUI
	fs := flag.NewFlagSet("anprr", flag.ExitOnError)
	flagToken := fs.String("token", "", "GitHub personal access token")
	flagNoSyntax := fs.Bool("no-syntax", false, "Disable syntax highlighting in diffs")
	flagDemo := fs.Bool("demo", false, "Run with mock data (no token required)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client, cache, repos, syntaxHL := mustResolveApp(*flagDemo, *flagToken, *flagNoSyntax)

	app := tui.NewApp(client, cache, repos, syntaxHL)

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func cmdLogin(args []string) {
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
	cfg.Token = token
	if err := config.Save(cfgPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Token saved.")
}

func cmdRepos(args []string) {
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
		cmdReposList(cfg)
	case "add":
		cmdReposAdd(args[1:], cfgPath, cfg)
	case "remove":
		cmdReposRemove(args[1:], cfgPath, cfg)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: anprr repos %s\n", args[0])
		os.Exit(1)
	}
}

func cmdReposList(cfg *config.Config) {
	if len(cfg.Repos) == 0 {
		fmt.Println("(none)")
		return
	}
	for _, r := range cfg.Repos {
		fmt.Println(r)
	}
}

func cmdReposAdd(args []string, cfgPath string, cfg *config.Config) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: anprr repos add <owner/repo>")
		os.Exit(1)
	}
	repo := args[0]
	if err := config.ValidateRepo(repo); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg.Repos = append(cfg.Repos, repo)
	if err := config.Save(cfgPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Added %s.\n", repo)
}

func mustResolveApp(isDemo bool, flagToken string, flagNoSyntax bool) (*github.Client, *github.Cache, []string, bool) {
	if isDemo {
		return github.NewClient("demo-token", demo.Transport{}), github.NewCache(), []string{"acme/backend"}, true
	}
	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	token := config.ResolveToken(flagToken, cfg)
	if token == "" {
		fmt.Fprintln(os.Stderr, "No token configured. Run: anprr login --token <token>")
		os.Exit(1)
	}
	if len(cfg.Repos) == 0 {
		fmt.Fprintln(os.Stderr, "No repositories configured. Run: anprr repos add <owner/repo>")
		os.Exit(1)
	}
	return github.NewClient(token, nil), github.NewCache(), cfg.Repos, !cfg.NoSyntax && !flagNoSyntax
}

func cmdReposRemove(args []string, cfgPath string, cfg *config.Config) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: anprr repos remove <owner/repo>")
		os.Exit(1)
	}
	repo := args[0]
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
	if err := config.Save(cfgPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed %s.\n", repo)
}
