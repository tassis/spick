package spick

import (
	"fmt"
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

func (initPrompter) Select(string, []ui.Option, int) (int, error)          { return 0, nil }
func (initPrompter) MultiSelect(string, []ui.Option, []int) ([]int, error) { return []int{0}, nil }
func (initPrompter) MatrixSelect(string, []ui.Option, []ui.Option, map[int][]int) (map[int][]int, error) {
	return map[int][]int{}, nil
}

type failInitPrompt struct{}

func (failInitPrompt) Select(string, []ui.Option, int) (int, error) {
	return 0, fmt.Errorf("prompt should not be called")
}
func (failInitPrompt) MultiSelect(string, []ui.Option, []int) ([]int, error) {
	return nil, fmt.Errorf("prompt should not be called")
}
func (failInitPrompt) MatrixSelect(string, []ui.Option, []ui.Option, map[int][]int) (map[int][]int, error) {
	return nil, fmt.Errorf("prompt should not be called")
}

type trackingInitPrompter struct{ modePromptSeen bool }

func (p *trackingInitPrompter) Select(title string, _ []ui.Option, _ int) (int, error) {
	if title == "Choose init mode" {
		p.modePromptSeen = true
	}
	return 0, nil
}
func (p *trackingInitPrompter) MultiSelect(string, []ui.Option, []int) ([]int, error) {
	return []int{0}, nil
}
func (p *trackingInitPrompter) MatrixSelect(string, []ui.Option, []ui.Option, map[int][]int) (map[int][]int, error) {
	return map[int][]int{}, nil
}

func TestInitProjectScaffoldsWithoutCreatingReadme(t *testing.T) {
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
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no README.md to be created, got err=%v", err)
	}
}

func TestInitProjectExistingFileWithoutForceFails(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	prev := initForce
	initForce = false
	defer func() { initForce = prev }()
	cmd := newInitCmd(nil)
	cmd.Flags().Bool("project", false, "")
	_ = cmd.Flags().Set("project", "true")
	if err := runInit(cmd); err == nil || !strings.Contains(err.Error(), "spick.yaml already exists") {
		t.Fatalf("expected existing-file error, got %v", err)
	}
}

func TestInitProjectForceOverwritesExistingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	prev := initForce
	initForce = true
	defer func() { initForce = prev }()
	if err := runInit(newInitCmd(nil)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "old: true") {
		t.Fatalf("expected overwrite, got %s", string(data))
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

func TestInitInteractiveSkipsPromptWhenProjectExists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompter := &trackingInitPrompter{}
	appService = app.New(prompter, workspace.New(root), nil)
	if err := runInit(newInitCmd(nil)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "spick.res.yaml")); err != nil {
		t.Fatalf("expected resource init, got %v", err)
	}
	if prompter.modePromptSeen {
		t.Fatal("did not expect mode prompt when project target already exists")
	}
}

func TestInitInteractiveSkipsPromptWhenResourceExists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompter := &trackingInitPrompter{}
	appService = app.New(prompter, workspace.New(root), nil)
	if err := runInit(newInitCmd(nil)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "spick.yaml")); err != nil {
		t.Fatalf("expected project init, got %v", err)
	}
	if prompter.modePromptSeen {
		t.Fatal("did not expect mode prompt when resource target already exists")
	}
}

func TestInitInteractiveFailsWhenBothTargetsExist(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(failInitPrompt{}, workspace.New(root), nil)
	if err := runInit(newInitCmd(nil)); err == nil || !strings.Contains(err.Error(), "use --force") {
		t.Fatalf("expected pre-prompt failure, got %v", err)
	}
}

func TestInitResourceExportsSelectedSkills(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "skills", "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "alpha", "SKILL.md"), []byte("# alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "beta", "SKILL.md"), []byte("# beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	if err := runResourceInit(newInitCmd(&cobra.Command{Use: "init"}), workspace.New(root)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.res.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "kind: resources") || !strings.Contains(text, "id: alpha") {
		t.Fatalf("unexpected skill scaffold: %s", text)
	}
}

func TestInitResourceExistingFileWithoutForceFails(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	prev := initForce
	initForce = false
	defer func() { initForce = prev }()
	err := runResourceInit(newInitCmd(&cobra.Command{Use: "init"}), workspace.New(root))
	if err == nil || !strings.Contains(err.Error(), "spick.res.yaml already exists; use --force to regenerate") {
		t.Fatalf("expected regenerate error, got %v", err)
	}
}

func TestInitResourceCreatesEmptyManifestWhenNoLocalSkills(t *testing.T) {
	root := t.TempDir()
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	err := runResourceInit(newInitCmd(&cobra.Command{Use: "init"}), workspace.New(root))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.res.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "kind: resources") || strings.Contains(text, "id:") {
		t.Fatalf("expected empty resource manifest, got %s", text)
	}
}

func TestInitResourceForceOverwritesExistingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("old: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(initPrompter{}, workspace.New(root), nil)
	prev := initForce
	initForce = true
	defer func() { initForce = prev }()
	if err := runResourceInit(newInitCmd(&cobra.Command{Use: "init"}), workspace.New(root)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.res.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "old: true") {
		t.Fatalf("expected overwrite, got %s", string(data))
	}
}

func newInitCmd(cmd *cobra.Command) *cobra.Command {
	if cmd != nil {
		return cmd
	}
	return &cobra.Command{Use: "init"}
}
