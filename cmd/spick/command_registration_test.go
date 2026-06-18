package spick

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootRegistersSkillAndPluginGroups(t *testing.T) {
	if got := childCommandNames(rootCmd); !containsName(got, "init") {
		t.Fatalf("expected root to register init command, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "skill") {
		t.Fatalf("expected root to register skill group, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "plugin") {
		t.Fatalf("expected root to register plugin group, got %v", got)
	}
	if got := childCommandNames(rootCmd); containsName(got, "add") {
		t.Fatalf("did not expect flat add command at root, got %v", got)
	}
	if got := childCommandNames(rootCmd); !containsName(got, "sync") {
		t.Fatalf("expected root to register sync command, got %v", got)
	}
}

func TestSkillGroupRegistersLifecycleCommands(t *testing.T) {
	got := childCommandNames(skillCmd)
	for _, name := range []string{"add", "inspect", "list", "rm", "apply"} {
		if !containsName(got, name) {
			t.Fatalf("expected skill group to register %s, got %v", name, got)
		}
	}
}

func TestPluginGroupRegistersPluginCommands(t *testing.T) {
	got := childCommandNames(pluginCmd)
	for _, name := range []string{"add", "inspect", "list", "rm"} {
		if !containsName(got, name) {
			t.Fatalf("expected plugin group to register %s, got %v", name, got)
		}
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
