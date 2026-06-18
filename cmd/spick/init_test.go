package spick

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

type initPrompter struct{}

func (initPrompter) Select(string, []ui.Option, int) (int, error) { return 0, nil }
func (initPrompter) MultiSelect(string, []ui.Option, []int) ([]int, error) { return []int{0}, nil }
func (initPrompter) MatrixSelect(string, []ui.Option, []ui.Option, map[int][]int) (map[int][]int, error) {
	return map[int][]int{}, nil
}

func TestInitProjectScaffoldsWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	if err := runInit(newInitCmd(nil)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "project:") || !strings.Contains(string(data), "autoApply: true") {
		t.Fatalf("unexpected project scaffold: %s", string(data))
	}
	if err := runInit(newInitCmd(nil)); err == nil {
		t.Fatal("expected existing scaffold to fail safely")
	}
}

func TestInitProjectAllowsExistingReadme(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	if err := runInit(newInitCmd(nil)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# existing\n" {
		t.Fatalf("expected existing README preserved, got %q", string(data))
	}
}

func TestInitSkillExportsSelectedSkills(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alpha"), 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(filepath.Join(root, "beta"), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "alpha", "SKILL.md"), []byte("# alpha\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "beta", "SKILL.md"), []byte("# beta\n"), 0o644); err != nil { t.Fatal(err) }
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	if err := runSkillInit(newInitCmd(&cobra.Command{Use: "init"}), workspace.New(root)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.skill.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "description: \"\"") || !strings.Contains(text, "id: alpha") {
		t.Fatalf("unexpected skill scaffold: %s", text)
	}
}

func TestInitPluginDefaultsEntryAndPromptsRuntime(t *testing.T) {
	root := t.TempDir()
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	if err := runPluginInit(newInitCmd(nil), workspace.New(root)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "entry: index.ts") || !strings.Contains(text, "runtime: node") {
		t.Fatalf("unexpected plugin scaffold: %s", text)
	}
}

func newInitCmd(cmd *cobra.Command) *cobra.Command {
	if cmd != nil {
		return cmd
	}
	return &cobra.Command{Use: "init"}
}
