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
	if len(mm.Selected) != 2 {
		t.Fatalf("expected all selected, got %+v", mm.Selected)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	mm = updated.(Model)
	if len(mm.Selected) != 0 {
		t.Fatalf("expected all deselected, got %+v", mm.Selected)
	}
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

func TestSingleSelectEscCancelsWithoutCompleting(t *testing.T) {
	m := New("t", []Option{{Label: "one"}, {Label: "two"}}, false, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(Model)
	if mm.Done {
		t.Fatalf("expected esc to cancel without completion, got %+v", mm)
	}
	if mm.Choice != -1 {
		t.Fatalf("expected no choice on esc, got %+v", mm)
	}
}

func TestMultiSelectEscCancelsWithoutCompleting(t *testing.T) {
	m := New("t", []Option{{Label: "one"}, {Label: "two"}}, true, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(Model)
	if mm.Done {
		t.Fatalf("expected esc to cancel without completion, got %+v", mm)
	}
	if len(mm.MultiChoice) != 0 {
		t.Fatalf("expected no multichoice on esc, got %+v", mm)
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

func TestMatrixModelMovesAcrossGridAndTogglesCells(t *testing.T) {
	m := NewMatrix("Apply", []MatrixOption{{Label: "skill-a"}, {Label: "skill-b"}}, []MatrixOption{{Label: "agent-1"}, {Label: "agent-2"}}, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(MatrixModel)
	if mm.Col != 1 {
		t.Fatalf("expected col 1, got %+v", mm)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm = updated.(MatrixModel)
	if mm.Row != 1 {
		t.Fatalf("expected row 1, got %+v", mm)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = updated.(MatrixModel)
	if !mm.Selected[1][1] {
		t.Fatalf("expected toggled cell selected, got %+v", mm.Selected)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(MatrixModel)
	if !mm.Done {
		t.Fatalf("expected matrix done, got %+v", mm)
	}
	if got := mm.Selections[1]; len(got) != 1 || got[0] != 1 {
		t.Fatalf("expected matrix selection [1], got %+v", mm.Selections)
	}
}

func TestMatrixViewIncludesGridAndLegend(t *testing.T) {
	m := NewMatrix("Apply", []MatrixOption{{Label: "skill-a"}}, []MatrixOption{{Label: "agent-1"}}, nil)
	view := m.View()
	if !strings.Contains(view, "skill-a") || !strings.Contains(view, "agent-1") {
		t.Fatalf("expected matrix labels in view, got %q", view)
	}
	if !strings.Contains(view, "space toggle") {
		t.Fatalf("expected legend in view, got %q", view)
	}
	if !strings.Contains(view, "esc cancel") {
		t.Fatalf("expected esc legend in view, got %q", view)
	}
}

func TestMatrixEscCancelsWithoutCompleting(t *testing.T) {
	m := NewMatrix("Apply", []MatrixOption{{Label: "skill-a"}}, []MatrixOption{{Label: "agent-1"}}, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(MatrixModel)
	if mm.Done {
		t.Fatalf("expected esc to cancel without completion, got %+v", mm)
	}
	if len(mm.Selections) != 0 {
		t.Fatalf("expected no selections on esc, got %+v", mm.Selections)
	}
}
