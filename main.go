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

const usage = `anprr — GitHub PR review TUI

Usage:
  anprr [--token <token>] [--syntax]   Launch TUI
  anprr login --token <token>          Save token to config
  anprr repos list                     List tracked repos
  anprr repos add <owner/repo>         Add a repo
  anprr repos remove <owner/repo>      Remove a repo
  anprr help                           Show this help

Config file: ~/.config/anprr/config.toml
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
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
	flagSyntax := fs.Bool("syntax", false, "Enable syntax highlighting in diffs")
	flagDemo := fs.Bool("demo", false, "Run with mock data (no token required)")
	fs.Parse(os.Args[1:])

	var (
		client   *github.Client
		cache    *github.Cache
		repos    []string
		syntaxHL bool
	)

	if *flagDemo {
		client = github.NewClient("demo-token", demo.Transport{})
		cache = github.NewCache()
		repos = []string{"acme/backend"}
		syntaxHL = true
	} else {
		cfgPath := config.DefaultConfigPath()
		cfg, err := config.Load(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		token := config.ResolveToken(*flagToken, cfg)
		if token == "" {
			fmt.Fprintln(os.Stderr, "No token configured. Run: anprr login --token <token>")
			os.Exit(1)
		}
		if len(cfg.Repos) == 0 {
			fmt.Fprintln(os.Stderr, "No repositories configured. Run: anprr repos add <owner/repo>")
			os.Exit(1)
		}
		client = github.NewClient(token, nil)
		cache = github.NewCache()
		repos = cfg.Repos
		syntaxHL = cfg.Syntax || *flagSyntax
	}

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
	fs.Parse(args)

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
		if len(cfg.Repos) == 0 {
			fmt.Println("(none)")
			return
		}
		for _, r := range cfg.Repos {
			fmt.Println(r)
		}

	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: anprr repos add <owner/repo>")
			os.Exit(1)
		}
		repo := args[1]
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

	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: anprr repos remove <owner/repo>")
			os.Exit(1)
		}
		repo := args[1]
		var kept []string
		removed := false
		for _, r := range cfg.Repos {
			if strings.EqualFold(r, repo) {
				removed = true
				continue
			}
			kept = append(kept, r)
		}
		if !removed {
			fmt.Printf("%s not found.\n", repo)
			return
		}
		cfg.Repos = kept
		if err := config.Save(cfgPath, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Removed %s.\n", repo)

	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: anprr repos %s\n", args[0])
		os.Exit(1)
	}
}
