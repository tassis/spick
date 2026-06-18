package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tassis/spick/internal/ui/prompttea"
)

type Option = prompttea.Option

type Prompter interface {
	Select(title string, options []Option, defaultIndex int) (int, error)
	MultiSelect(title string, options []Option, defaults []int) ([]int, error)
	MatrixSelect(title string, rows []Option, cols []Option, defaults map[int][]int) (map[int][]int, error)
}

type PromptTea struct{}

func NewPromptTea() PromptTea { return PromptTea{} }

func (PromptTea) Select(title string, options []Option, defaultIndex int) (int, error) {
	model, err := tea.NewProgram(prompttea.New(title, options, false, []int{defaultIndex})).Run()
	if err != nil {
		return 0, err
	}
	return singleSelection(model, defaultIndex), nil
}

func (PromptTea) MultiSelect(title string, options []Option, defaults []int) ([]int, error) {
	model, err := tea.NewProgram(prompttea.New(title, options, true, defaults)).Run()
	if err != nil {
		return nil, err
	}
	return multiSelection(model, defaults), nil
}

func (PromptTea) MatrixSelect(title string, rows []Option, cols []Option, defaults map[int][]int) (map[int][]int, error) {
	rowOpts := make([]prompttea.MatrixOption, 0, len(rows))
	for _, row := range rows {
		rowOpts = append(rowOpts, prompttea.MatrixOption{Label: row.Label})
	}
	colOpts := make([]prompttea.MatrixOption, 0, len(cols))
	for _, col := range cols {
		colOpts = append(colOpts, prompttea.MatrixOption{Label: col.Label})
	}
	model, err := tea.NewProgram(prompttea.NewMatrix(title, rowOpts, colOpts, defaults)).Run()
	if err != nil {
		return nil, err
	}
	return matrixSelection(model, defaults), nil
}

func singleSelection(model tea.Model, fallback int) int {
	if m, ok := model.(prompttea.Model); ok && m.Done {
		return m.Choice
	}
	return fallback
}

func multiSelection(model tea.Model, fallback []int) []int {
	if m, ok := model.(prompttea.Model); ok && m.Done {
		return m.MultiChoice
	}
	return fallback
}

func matrixSelection(model tea.Model, fallback map[int][]int) map[int][]int {
	if m, ok := model.(prompttea.MatrixModel); ok && m.Done {
		return m.Selections
	}
	return fallback
}
