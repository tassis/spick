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

type MatrixOption struct {
	Label string
}

type MatrixModel struct {
	Title      string
	Rows       []MatrixOption
	Cols       []MatrixOption
	Row        int
	Col        int
	Selected   map[int]map[int]bool
	Done       bool
	Selections map[int][]int
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

func NewMatrix(title string, rows, cols []MatrixOption, defaults map[int][]int) MatrixModel {
	m := MatrixModel{Title: title, Rows: rows, Cols: cols, Selected: map[int]map[int]bool{}, Selections: map[int][]int{}}
	if len(rows) > 0 {
		m.Row = 0
	}
	if len(cols) > 0 {
		m.Col = 0
	}
	for r, cols := range defaults {
		if m.Selected[r] == nil {
			m.Selected[r] = map[int]bool{}
		}
		for _, c := range cols {
			m.Selected[r][c] = true
		}
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, tea.Quit
		case "ctrl+c", "q", "enter":
			if m.Multi {
				m.MultiChoice = m.indices()
			} else {
				m.Choice = m.Cursor
			}
			m.Done = true
			return m, tea.Quit
		case "up":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down":
			if m.Cursor < len(m.Options)-1 {
				m.Cursor++
			}
		case "a":
			if m.Multi {
				all := m.allSelected()
				m.Selected = map[int]bool{}
				if !all {
					for i := range m.Options {
						m.Selected[i] = true
					}
				}
			}
		case " ":
			if m.Multi {
				m.Selected[m.Cursor] = !m.Selected[m.Cursor]
			}
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
		lines = append(lines, "", "↑/↓ move • space toggle • a toggle all • enter confirm • esc cancel")
	} else {
		lines = append(lines, "", "↑/↓ move • enter confirm • esc cancel")
	}

	return strings.Join(lines, "\n")
}

func (m MatrixModel) Init() tea.Cmd { return nil }

func (m MatrixModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, tea.Quit
		case "ctrl+c", "q", "enter":
			m.Selections = m.matrixSelections()
			m.Done = true
			return m, tea.Quit
		case "left":
			if m.Col > 0 {
				m.Col--
			}
		case "right":
			if m.Col < len(m.Cols)-1 {
				m.Col++
			}
		case "up":
			if m.Row > 0 {
				m.Row--
			}
		case "down":
			if m.Row < len(m.Rows)-1 {
				m.Row++
			}
		case " ":
			if m.Selected[m.Row] == nil {
				m.Selected[m.Row] = map[int]bool{}
			}
			m.Selected[m.Row][m.Col] = !m.Selected[m.Row][m.Col]
		}
	}
	return m, nil
}

func (m MatrixModel) View() string {
	lines := []string{m.Title, ""}
	headers := []string{""}
	for i, col := range m.Cols {
		head := col.Label
		if i == m.Col {
			head = "> " + head
		}
		headers = append(headers, head)
	}
	lines = append(lines, strings.Join(headers, "  "))
	for r, row := range m.Rows {
		parts := []string{row.Label}
		for c := range m.Cols {
			mark := " "
			if m.Selected[r] != nil && m.Selected[r][c] {
				mark = "x"
			}
			cell := fmt.Sprintf("[%s]", mark)
			if r == m.Row && c == m.Col {
				cell = ">" + cell[1:]
			}
			parts = append(parts, cell)
		}
		lines = append(lines, strings.Join(parts, "  "))
	}
	lines = append(lines, "", "←/→ agent • ↑/↓ skill • space toggle • enter confirm • esc cancel")
	return strings.Join(lines, "\n")
}

func (m MatrixModel) matrixSelections() map[int][]int {
	out := map[int][]int{}
	for r, cols := range m.Selected {
		for c, ok := range cols {
			if ok {
				out[r] = append(out[r], c)
			}
		}
	}
	return out
}

func (m Model) indices() []int {
	out := make([]int, 0, len(m.Selected))
	for i := range m.Selected {
		if m.Selected[i] {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) allSelected() bool {
	if len(m.Options) == 0 {
		return false
	}
	for i := range m.Options {
		if !m.Selected[i] {
			return false
		}
	}
	return true
}
