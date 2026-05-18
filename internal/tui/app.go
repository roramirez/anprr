package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/roramirez/anprr/internal/github"
)

const (
	minWidth  = 80
	minHeight = 24
)

type screen int

const (
	screenList screen = iota
	screenDetail
)

// AppModel is the root bubbletea model. It routes between screens and holds
// shared state: github client, cache, current user, window size.
type AppModel struct {
	client      *github.Client
	cache       *github.Cache
	currentUser string
	repos       []string
	syntaxHL    bool

	width  int
	height int
	tooSmall bool

	active   screen
	list     ListModel
	detail   DetailModel
	showHelp bool

	statusText    string
	statusIsError bool
	statusIsOK    bool
}

func NewApp(client *github.Client, cache *github.Cache, repos []string, syntaxHL bool) AppModel {
	return AppModel{
		client:   client,
		cache:    cache,
		repos:    repos,
		syntaxHL: syntaxHL,
		active:   screenList,
		list:     newListModel(),
		detail:   newDetailModel(),
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		fetchCurrentUserCmd(m.client),
		fetchPRsCmd(m.client, m.cache, m.repos),
		searchReviewRequestedCmd(m.client, m.cache, m.repos),
		tickCmd(),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.tooSmall = msg.Width < minWidth || msg.Height < minHeight
		if !m.tooSmall {
			m.list = m.list.setSize(msg.Width, msg.Height)
			m.detail = m.detail.setSize(msg.Width, msg.Height)
		}
		return m, nil

	case CurrentUserMsg:
		if msg.Err != nil {
			return m, statusCmd("Auth error: "+msg.Err.Error(), true)
		}
		m.currentUser = msg.Login
		m.list = m.list.setCurrentUser(msg.Login)
		return m, nil

	case PRsLoadedMsg:
		var cmds []tea.Cmd
		m.list, cmds = updateListModel(m.list, msg)
		// check rate limit
		if rl := m.client.RateLimitRemaining(); rl >= 0 && rl < 100 {
			cmds = append(cmds, statusCmd(fmt.Sprintf("⚠ %d API requests remaining", rl), false))
		}
		return m, tea.Batch(cmds...)

	case MorePRsLoadedMsg:
		var cmds []tea.Cmd
		m.list, cmds = updateListModelMore(m.list, msg)
		return m, tea.Batch(cmds...)

	case NavigateToDetailMsg:
		m.active = screenDetail
		m.detail = newDetailModel()
		m.detail = m.detail.setSize(m.width, m.height)
		m.detail = m.detail.setPR(msg.PR, msg.FocusComment)
		return m, fetchDiffCmd(m.client, m.cache, msg.PR)

	case NavigateToListMsg:
		m.active = screenList
		return m, nil

	case DiffLoadedMsg:
		var cmd tea.Cmd
		m.detail, cmd = updateDetailDiff(m.detail, msg, m.syntaxHL)
		return m, cmd

	case ReviewDoneMsg:
		m.cache.Invalidate()
		if msg.Err != nil {
			return m, tea.Batch(statusCmd(msg.Err.Error(), true), fetchPRsCmd(m.client, m.cache, m.repos))
		}
		return m, tea.Batch(statusCmd("✓ Review submitted", false), fetchPRsCmd(m.client, m.cache, m.repos))

	case CommentDoneMsg:
		if msg.Err != nil {
			return m, statusCmd(msg.Err.Error(), true)
		}
		return m, statusCmd("✓ Comment posted", false)

	case StatusMsg:
		m.statusText = msg.Text
		m.statusIsError = msg.IsError
		m.statusIsOK = msg.IsOK
		return m, clearStatusCmd(3 * time.Second)

	case ClearStatusMsg:
		m.statusText = ""
		m.statusIsError = false
		m.statusIsOK = false
		return m, nil

	case ReviewRequestedLoadedMsg:
		if msg.Err == nil {
			m.list = m.list.setReviewRequestedSet(msg.Set)
		}
		return m, nil

	case TickMsg:
		m.cache.Invalidate()
		return m, tea.Batch(
			fetchPRsCmd(m.client, m.cache, m.repos),
			searchReviewRequestedCmd(m.client, m.cache, m.repos),
			tickCmd(),
		)

	case ToggleHelpMsg:
		m.showHelp = !m.showHelp
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp && (msg.String() == "esc" || msg.String() == "?") {
			m.showHelp = false
			return m, nil
		}
		if m.showHelp {
			return m, nil
		}
	}

	// delegate to active screen
	var cmd tea.Cmd
	switch m.active {
	case screenList:
		m.list, cmd = m.list.update(msg, m.client, m.cache, m.repos)
	case screenDetail:
		m.detail, cmd = m.detail.update(msg, m.client, m.cache)
	}
	return m, cmd
}

func (m AppModel) View() string {
	if m.tooSmall {
		return StyleTooSmall.Render(fmt.Sprintf(
			"Terminal too small (need %dx%d, current %dx%d).\nResize to continue.",
			minWidth, minHeight, m.width, m.height,
		))
	}

	var body string
	switch m.active {
	case screenList:
		body = m.list.view(m.width, m.height, m.statusBar(), m.client.RateLimitRemaining())
	case screenDetail:
		body = m.detail.view(m.width, m.height, m.statusBar())
	}

	if m.showHelp {
		return renderHelp(m.width, m.height)
	}
	return body
}

func (m AppModel) statusBar() string {
	if m.statusText == "" {
		return ""
	}
	if m.statusIsError {
		return StyleStatusBarError.Render(m.statusText)
	}
	if m.statusIsOK {
		return StyleStatusBarOK.Render(m.statusText)
	}
	return StyleStatusBarWarn.Render(m.statusText)
}

// Commands

func fetchPRsCmd(client *github.Client, cache *github.Cache, repos []string) tea.Cmd {
	return func() tea.Msg {
		prs, err := client.ListPRs(repos, cache)
		return PRsLoadedMsg{PRs: prs, Err: err}
	}
}

func fetchDiffCmd(client *github.Client, cache *github.Cache, pr github.PR) tea.Cmd {
	return func() tea.Msg {
		owner, repo := splitRepo(pr.Repo)
		diff, err := client.GetDiff(owner, repo, pr.Number, cache)
		return DiffLoadedMsg{RawDiff: diff, Err: err}
	}
}

func fetchCurrentUserCmd(client *github.Client) tea.Cmd {
	return func() tea.Msg {
		login, err := client.GetCurrentUser()
		return CurrentUserMsg{Login: login, Err: err}
	}
}

func statusCmd(text string, isError bool) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg{Text: text, IsError: isError, IsOK: !isError && text != ""}
	}
}

func clearStatusCmd(after time.Duration) tea.Cmd {
	return tea.Tick(after, func(time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}

func tickCmd() tea.Cmd {
	return tea.Tick(60*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func splitRepo(repo string) (owner, name string) {
	for i, c := range repo {
		if c == '/' {
			return repo[:i], repo[i+1:]
		}
	}
	return repo, ""
}
