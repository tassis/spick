package spick

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

func TestListCommandHumanReadable(t *testing.T) {
	root := t.TempDir()
	writeListTestFiles(t, root)
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("project:\n  agents:\n    opencode: {}\n    codex: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.json = false
	listOpts.skill = false
	listOpts.plugins = false
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "skills") || !strings.Contains(output, "plugins") || !strings.Contains(output, "agents") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestListCommandJSON(t *testing.T) {
	root := t.TempDir()
	writeListTestFiles(t, root)
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	_, err := appService.Add(app.AddOptions{Scope: config.ScopeProject, Source: app.SourceFromLocator(root), All: true})
	if err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.json = true
	listOpts.skill = false
	listOpts.plugins = false
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"skills"`) {
		t.Fatalf("unexpected json: %q", buf.String())
	}
}

func TestListCommandSkillFilter(t *testing.T) {
	root := t.TempDir()
	writeListTestFiles(t, root)
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	if err := appService.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".skills", "demo")}}}); err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.skill = true
	defer func() { listOpts.skill = false }()
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "skills") || strings.Contains(got, "plugins") {
		t.Fatalf("expected skill-only section, got %q", got)
	}
}

func TestListSurfaceOmitsRemovedFlags(t *testing.T) {
	if listCmd.Flags().Lookup("scope") != nil {
		t.Fatal("did not expect --scope flag")
	}
	if listCmd.Flags().Lookup("json") == nil {
		t.Fatal("expected --json flag")
	}
	if listCmd.Flags().Lookup("skill") == nil || listCmd.Flags().Lookup("plugins") == nil {
		t.Fatal("expected --skill and --plugins flags")
	}
}

func TestListCommandEmptyPrintsNothing(t *testing.T) {
	root := t.TempDir()
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.json = false
	listOpts.skill = false
	listOpts.plugins = false
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); !strings.Contains(got, "(none)") {
		t.Fatalf("expected sectioned empty output, got %q", got)
	}
}

func writeListTestFiles(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
