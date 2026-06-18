package spick

import "testing"

func TestAddSurfaceOmitsRemovedFlags(t *testing.T) {
	if addCmd.Flags().Lookup("version") != nil {
		t.Fatal("did not expect --version flag")
	}
	if addCmd.Flags().Lookup("yes") != nil {
		t.Fatal("did not expect --yes flag")
	}
	if addCmd.Flags().Lookup("ref") != nil {
		t.Fatal("did not expect --ref flag")
	}
	if addCmd.Flags().Lookup("all") == nil {
		t.Fatal("expected --all flag")
	}
	if addCmd.Flags().Lookup("exposure-method") == nil {
		t.Fatal("expected --exposure-method flag")
	}
	if addCmd.Flags().Lookup("force") == nil {
		t.Fatal("expected --force flag")
	}
}

func TestAddUsesExposureMethodFlag(t *testing.T) {
	if addCmd.Flags().Lookup("exposure-method") == nil {
		t.Fatal("expected --exposure-method flag")
	}
	if addCmd.Flags().Lookup("mode") != nil {
		t.Fatal("did not expect legacy --mode flag")
	}
}
