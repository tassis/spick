package spick

import (
	"strings"
	"testing"
)

func TestAddRejectsYesFlag(t *testing.T) {
	defer func() { addOpts.yes = false }()
	addOpts.yes = true
	if err := addCmd.RunE(addCmd, []string{"source"}); err == nil || !strings.Contains(err.Error(), "--yes is not supported for add") {
		t.Fatalf("expected yes rejection, got %v", err)
	}
}
