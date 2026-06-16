package prompttea

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Option struct {
	Label string
}

type Model struct {
	Title       string
	Options     []Option
	Cursor      int
	Selected    map[int]bool
	Multi       bool
	Done        bool
	Choice      int
	MultiChoice []int
}

func New(title string, options []Option, multi bool, defaults []int) Model {
	m := Model{Title: title, Options: options, Selected: map[int]bool{}, Multi: multi, Choice: -1}
	for _, d := range defaults {
		m.Selected[d] = true
		if !multi {
			m.Cursor = d
		}
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "enter":
			if m.Multi {
				m.MultiChoice = m.indices()
			} else {
				m.Choice = m.Cursor
			}
			m.Done = true
			return m, tea.Quit
		case "up":
			if m.Cursor > 0 { m.Cursor-- }
		case "down":
			if m.Cursor < len(m.Options)-1 { m.Cursor++ }
		case "a":
			if m.Multi {
				all := m.allSelected()
				m.Selected = map[int]bool{}
				if !all { for i := range m.Options { m.Selected[i] = true } }
			}
		case " ":
			if m.Multi { m.Selected[m.Cursor] = !m.Selected[m.Cursor] }
		}
	}
	return m, nil
}

func (m Model) View() string {
	lines := []string{m.Title, ""}

	for i, option := range m.Options {
		cursor := " "
		if i == m.Cursor {
			cursor = ">"
		}

		marker := " "
		if m.Multi {
			if m.Selected[i] {
				marker = "x"
			}
			lines = append(lines, fmt.Sprintf("%s [%s] %s", cursor, marker, option.Label))
			continue
		}

		lines = append(lines, fmt.Sprintf("%s %s", cursor, option.Label))
	}

	if m.Multi {
		lines = append(lines, "", "↑/↓ move • space toggle • a toggle all • enter confirm")
	} else {
		lines = append(lines, "", "↑/↓ move • enter confirm")
	}

	return strings.Join(lines, "\n")
}

func (m Model) indices() []int {
	out := make([]int, 0, len(m.Selected))
	for i := range m.Selected { if m.Selected[i] { out = append(out, i) } }
	return out
}

func (m Model) allSelected() bool {
	if len(m.Options) == 0 { return false }
	for i := range m.Options {
		if !m.Selected[i] { return false }
	}
	return true
}
