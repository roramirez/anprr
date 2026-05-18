package tui

import "github.com/charmbracelet/bubbles/key"

type listKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Enter   key.Binding
	Approve key.Binding
	Changes key.Binding
	Comment key.Binding
	Refresh key.Binding
	LoadMore key.Binding
	Help    key.Binding
	Quit    key.Binding
}

type detailKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	PageUp  key.Binding
	PageDown key.Binding
	Approve key.Binding
	Changes key.Binding
	Comment key.Binding
	Refresh key.Binding
	Web     key.Binding
	Back    key.Binding
	Help    key.Binding
	Quit    key.Binding
}

var ListKeys = listKeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/↑", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/↓", "down")),
	Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "my PRs")),
	Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "needs review")),
	Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view")),
	Approve: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "approve")),
	Changes: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "request changes")),
	Comment: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comment")),
	Refresh: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "refresh")),
	LoadMore: key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "load more")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

var DetailKeys = detailKeyMap{
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/↑", "scroll up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/↓", "scroll down")),
	PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	Approve:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "approve")),
	Changes:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "request changes")),
	Comment:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comment")),
	Refresh:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "refresh")),
	Web:      key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "open in browser")),
	Back:     key.NewBinding(key.WithKeys("b", "esc"), key.WithHelp("b/esc", "back")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
