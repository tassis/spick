package spick

import "testing"

func TestSyncSurfaceUsesGlobalFlag(t *testing.T) {
	if syncCmd.Flags().Lookup("scope") != nil {
		t.Fatal("did not expect --scope flag")
	}
	if syncCmd.Flags().Lookup("global") == nil {
		t.Fatal("expected --global flag")
	}
	if syncCmd.Flags().Lookup("g") != nil {
		t.Fatal("expected shorthand only via global flag metadata")
	}
}
