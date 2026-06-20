package spick

import "testing"

func TestApplyUsesNewFlagSurface(t *testing.T) {
	if got := childCommandNames(applyCmd); len(got) != 0 {
		t.Fatalf("expected apply to have no child commands, got %v", childCommandNames(applyCmd))
	}
	if applyCmd.Flags().Lookup("scope") != nil {
		t.Fatal("did not expect --scope flag")
	}
	for _, name := range []string{"global", "skill", "plugin", "agent"} {
		if applyCmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected --%s flag", name)
		}
	}
}
