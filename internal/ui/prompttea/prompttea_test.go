package prompttea

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSingleSelectEnterKeepsCursor(t *testing.T) {
	m := New("t", []Option{{Label: "one"}, {Label: "two"}}, false, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)
	if !mm.Done || mm.Choice != 0 {
		t.Fatalf("expected choice 0 done=true, got %+v", mm)
	}
}

func TestMultiSelectATogglesAll(t *testing.T) {
	m := New("t", []Option{{Label: "one"}, {Label: "two"}}, true, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	mm := updated.(Model)
	if len(mm.Selected) != 2 { t.Fatalf("expected all selected, got %+v", mm.Selected) }
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	mm = updated.(Model)
	if len(mm.Selected) != 0 { t.Fatalf("expected all deselected, got %+v", mm.Selected) }
}

func TestMultiSelectSpaceAndEnterReturnsSelectedIndices(t *testing.T) {
	m := New("t", []Option{{Label: "one"}, {Label: "two"}, {Label: "three"}}, true, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if !mm.Done || len(mm.MultiChoice) != 1 || mm.MultiChoice[0] != 1 {
		t.Fatalf("expected selection [1] done=true, got %+v", mm)
	}
}

func TestViewRendersOptions(t *testing.T) {
	m := New("Pick one", []Option{{Label: "one"}, {Label: "two"}}, false, nil)
	view := m.View()
	if !strings.Contains(view, "one") || !strings.Contains(view, "two") {
		t.Fatalf("expected options in view, got %q", view)
	}
	if !strings.Contains(view, "> one") {
		t.Fatalf("expected cursor indicator in view, got %q", view)
	}
}
