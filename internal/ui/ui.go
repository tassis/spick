package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tassis/spick/internal/ui/prompttea"
)

type Option = prompttea.Option

type Prompter interface {
	Select(title string, options []Option, defaultIndex int) (int, error)
	MultiSelect(title string, options []Option, defaults []int) ([]int, error)
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
