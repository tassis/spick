package spick

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootRegistersVerbFirstLifecycleGroups(t *testing.T) {
	if got := childCommandNames(rootCmd); !containsName(got, "init") {
		t.Fatalf("expected root to register init command, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "inspect") {
		t.Fatalf("expected root to register inspect command, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "list") {
		t.Fatalf("expected root to register list command, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "add") {
		t.Fatalf("expected root to register add command, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "rm") {
		t.Fatalf("expected root to register rm command, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "apply") {
		t.Fatalf("expected root to register apply command, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "sync") {
		t.Fatalf("expected root to register sync command, got %v", got)
	}
}

func TestAddGroupIsSourceOriented(t *testing.T) {
	if got := childCommandNames(addCmd); len(got) != 0 {
		t.Fatalf("expected add to have no child commands, got %v", got)
	}
}

func TestRmGroupIsSourceOriented(t *testing.T) {
	if got := childCommandNames(rmCmd); len(got) != 0 {
		t.Fatalf("expected rm to have no child commands, got %v", got)
	}
}

func TestApplyIsRootLevelAndUnnested(t *testing.T) {
	if !containsName(childCommandNames(rootCmd), "apply") {
		t.Fatalf("expected root to register apply command, got %v", childCommandNames(rootCmd))
	}
	if got := childCommandNames(applyCmd); len(got) != 0 {
		t.Fatalf("expected apply to have no child commands, got %v", got)
	}
	for _, name := range []string{"global", "skill", "plugin", "agent"} {
		if applyCmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected apply to register --%s flag", name)
		}
	}
	if applyCmd.Flags().Lookup("scope") != nil {
		t.Fatal("did not expect --scope flag on apply")
	}
}

func childCommandNames(cmd *cobra.Command) []string {
	children := cmd.Commands()
	got := make([]string, 0, len(children))
	for _, child := range children {
		got = append(got, child.Name())
	}
	return got
}

func containsName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
