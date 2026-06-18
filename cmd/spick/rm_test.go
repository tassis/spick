package spick

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

type rmFakePrompter struct{ multi []int }

func (f rmFakePrompter) Select(title string, options []ui.Option, defaultIndex int) (int, error) {
	return defaultIndex, nil
}
func (f rmFakePrompter) MultiSelect(title string, options []ui.Option, defaults []int) ([]int, error) {
	return f.multi, nil
}
func (f rmFakePrompter) MatrixSelect(title string, rows []ui.Option, cols []ui.Option, defaults map[int][]int) (map[int][]int, error) {
	return nil, nil
}

func TestRmNoArgsUsesSelectionPrompt(t *testing.T) {
	root := t.TempDir()
	writeRmTestFiles(t, root)
	appService = app.New(rmFakePrompter{multi: []int{1}}, workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	rmCmd.SetOut(buf)
	if err := rmCmd.RunE(rmCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "two") {
		t.Fatalf("expected selected skill removed, got %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "two")); !os.IsNotExist(err) {
		t.Fatalf("expected exposure removed, got %v", err)
	}
}

func TestRmRemovesAllTrackedExposures(t *testing.T) {
	root := t.TempDir()
	writeRmTestFiles(t, root)
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("project:\n  agents:\n    opencode: {}\n    codex: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.skill.yaml"), []byte("version: 1\nskills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(rmFakePrompter{multi: []int{0}}, workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := rmCmd.RunE(rmCmd, []string{"one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "one")); !os.IsNotExist(err) {
		t.Fatalf("expected opencode exposure removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "one")); !os.IsNotExist(err) {
		t.Fatalf("expected codex exposure removed, got %v", err)
	}
}

func TestRmPruneOutputIncludesContext(t *testing.T) {
	root := t.TempDir()
	writeRmTestFiles(t, root)
	appService = app.New(rmFakePrompter{multi: []int{0}}, workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	rmCmd.SetOut(buf)
	rmOpts.pruneUnused = true
	defer func() { rmOpts.pruneUnused = false }()
	if err := rmCmd.RunE(rmCmd, []string{"one"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "pruned-unused") || !strings.Contains(buf.String(), "removed fully") {
		t.Fatalf("expected contextual output, got %q", buf.String())
	}
}

func TestRmNoInstalledSkillsPrintsMessage(t *testing.T) {
	root := t.TempDir()
	appService = app.New(rmFakePrompter{}, workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	rmCmd.SetOut(buf)
	if err := rmCmd.RunE(rmCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no installed skills") {
		t.Fatalf("expected empty-state message, got %q", buf.String())
	}
}

func TestRmSurfaceOmitsRemovedFlags(t *testing.T) {
	if rmCmd.Flags().Lookup("yes") != nil {
		t.Fatal("did not expect --yes flag")
	}
	if rmCmd.Flags().Lookup("force") != nil {
		t.Fatal("did not expect --force flag")
	}
	if rmCmd.Flags().Lookup("prune-unused") == nil {
		t.Fatal("expected --prune-unused flag")
	}
}

func writeRmTestFiles(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.skill.yaml"), []byte("version: 1\nskills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
