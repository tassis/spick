package ui

import (
	"reflect"
	"testing"

	"github.com/tassis/spick/internal/ui/prompttea"
)

func TestSelectionHelpersReturnModelResults(t *testing.T) {
	model := prompttea.Model{Done: true, Choice: 1, MultiChoice: []int{2, 0}}
	if got := singleSelection(model, 9); got != 1 {
		t.Fatalf("expected single selection 1, got %d", got)
	}
	if got := multiSelection(model, []int{9}); !reflect.DeepEqual(got, []int{2, 0}) {
		t.Fatalf("expected multi selection [2 0], got %v", got)
	}
}

func TestSelectionHelpersFallbackWhenNotDone(t *testing.T) {
	model := prompttea.Model{Done: false, Choice: 1, MultiChoice: []int{2}}
	if got := singleSelection(model, 7); got != 7 {
		t.Fatalf("expected fallback 7, got %d", got)
	}
	if got := multiSelection(model, []int{7}); !reflect.DeepEqual(got, []int{7}) {
		t.Fatalf("expected fallback [7], got %v", got)
	}
}
