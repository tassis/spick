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

func TestListCommandHumanReadable(t *testing.T) {
	root := t.TempDir()
	writeListTestFiles(t, root)
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil { t.Fatal(err) }
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.json = false
	if err := listCmd.RunE(listCmd, nil); err != nil { t.Fatal(err) }
	if !strings.Contains(buf.String(), "demo") { t.Fatalf("unexpected output: %q", buf.String()) }
}

func TestListCommandJSON(t *testing.T) {
	root := t.TempDir()
	writeListTestFiles(t, root)
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil { t.Fatal(err) }
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.json = true
	if err := listCmd.RunE(listCmd, nil); err != nil { t.Fatal(err) }
	if !strings.Contains(buf.String(), `"skills"`) { t.Fatalf("unexpected json: %q", buf.String()) }
}

func TestListCommandRejectsAllFlag(t *testing.T) {
	listOpts.all = true
	defer func() { listOpts.all = false }()
	if err := listCmd.RunE(listCmd, nil); err == nil || !strings.Contains(err.Error(), "--all is not supported for list") {
		t.Fatalf("expected all rejection, got %v", err)
	}
}

func TestListCommandEmptyPrintsNothing(t *testing.T) {
	root := t.TempDir()
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.json = false
	listOpts.all = false
	if err := listCmd.RunE(listCmd, nil); err != nil { t.Fatal(err) }
	if got := buf.String(); got != "" {
		t.Fatalf("expected no output for empty list, got %q", got)
	}
}

func writeListTestFiles(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("catalog:\n  skills:\n    - id: demo\n      path: .\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil { t.Fatal(err) }
}
