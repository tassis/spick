package spick

import "testing"

func TestApplyUsesNewFlagSurface(t *testing.T) {
	if applyCmd.Flags().Lookup("exposure-method") == nil {
		t.Fatal("expected exposure-method flag")
	}
	if applyCmd.Flags().Lookup("agent") == nil {
		t.Fatal("expected agent flag")
	}
}
