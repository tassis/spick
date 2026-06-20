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
	if _, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true}); err != nil {
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
}

func TestRmSurfaceUsesFlagsAndMutualExclusion(t *testing.T) {
	if rmCmd.Flags().Lookup("skill") == nil || rmCmd.Flags().Lookup("plugin") == nil || rmCmd.Flags().Lookup("agent") == nil {
		t.Fatal("expected narrowing flags")
	}
	if rmCmd.Flags().Lookup("scope") != nil {
		t.Fatal("did not expect --scope flag")
	}
}

func TestRmNoInstalledSkillsPrintsMessage(t *testing.T) {
	root := t.TempDir()
	appService = app.New(rmFakePrompter{}, workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	rmCmd.SetOut(buf)
	if err := rmCmd.RunE(rmCmd, nil); err == nil || !strings.Contains(err.Error(), "no removable resource") {
		t.Fatalf("expected no-removable error, got %v", err)
	}
}

func writeRmTestFiles(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
