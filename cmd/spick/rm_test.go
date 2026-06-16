package spick

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
	"github.com/tassis/spick/internal/config"
)

type rmFakePrompter struct{ multi []int }

func (f rmFakePrompter) Select(title string, options []ui.Option, defaultIndex int) (int, error) { return defaultIndex, nil }
func (f rmFakePrompter) MultiSelect(title string, options []ui.Option, defaults []int) ([]int, error) { return f.multi, nil }

func TestRmNoArgsUsesSelectionPrompt(t *testing.T) {
	root := t.TempDir()
	writeRmTestFiles(t, root)
	appService = app.New(rmFakePrompter{multi: []int{1}}, workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil { t.Fatal(err) }
	buf := &bytes.Buffer{}
	rmCmd.SetOut(buf)
	if err := rmCmd.RunE(rmCmd, nil); err != nil { t.Fatal(err) }
	if !strings.Contains(buf.String(), "two") { t.Fatalf("expected selected skill removed, got %q", buf.String()) }
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "two")); !os.IsNotExist(err) { t.Fatalf("expected exposure removed, got %v", err) }
}

func TestRmPurgeAndPruneOutputIncludesContext(t *testing.T) {
	root := t.TempDir()
	writeRmTestFiles(t, root)
	appService = app.New(rmFakePrompter{multi: []int{0}}, workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil { t.Fatal(err) }
	buf := &bytes.Buffer{}
	rmCmd.SetOut(buf)
	rmOpts.purge = true
	rmOpts.pruneUnused = true
	defer func() { rmOpts.purge = false; rmOpts.pruneUnused = false }()
	if err := rmCmd.RunE(rmCmd, []string{"one"}); err != nil { t.Fatal(err) }
	if !strings.Contains(buf.String(), "pruned-unused") || !strings.Contains(buf.String(), "purged") {
		t.Fatalf("expected contextual output, got %q", buf.String())
	}
}

func TestRmNoInstalledSkillsPrintsMessage(t *testing.T) {
	root := t.TempDir()
	appService = app.New(rmFakePrompter{}, workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	rmCmd.SetOut(buf)
	if err := rmCmd.RunE(rmCmd, nil); err != nil { t.Fatal(err) }
	if !strings.Contains(buf.String(), "no installed skills") { t.Fatalf("expected empty-state message, got %q", buf.String()) }
}

func TestRmRejectsYesAndForceFlags(t *testing.T) {
	defer func() { rmOpts.yes = false; rmOpts.force = false }()
	rmOpts.yes = true
	if err := rmCmd.RunE(rmCmd, nil); err == nil || !strings.Contains(err.Error(), "--yes is not supported for rm") {
		t.Fatalf("expected yes rejection, got %v", err)
	}
	rmOpts.yes = false
	rmOpts.force = true
	if err := rmCmd.RunE(rmCmd, nil); err == nil || !strings.Contains(err.Error(), "--force is not supported for rm") {
		t.Fatalf("expected force rejection, got %v", err)
	}
}

func writeRmTestFiles(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("catalog:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil { t.Fatal(err) }
}
