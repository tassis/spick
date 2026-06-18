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
	if err := os.WriteFile(filepath.Join(root, "spick.skill.yaml"), []byte("version: 1\nskills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
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
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "demo [codex, opencode]") {
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
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"exposures"`) {
		t.Fatalf("unexpected json: %q", buf.String())
	}
}

func TestListCommandHidesEmptyAgentBadge(t *testing.T) {
	root := t.TempDir()
	writeListTestFiles(t, root)
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	if err := appService.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".skills", "demo")}}}); err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.json = false
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "demo" {
		t.Fatalf("expected plain skill name, got %q", got)
	}
}

func TestListSurfaceOmitsRemovedFlags(t *testing.T) {
	if listCmd.Flags().Lookup("all") != nil {
		t.Fatal("did not expect --all flag")
	}
	if listCmd.Flags().Lookup("json") == nil {
		t.Fatal("expected --json flag")
	}
	if listCmd.Flags().Lookup("scope") == nil {
		t.Fatal("expected --scope flag")
	}
}

func TestListCommandEmptyPrintsNothing(t *testing.T) {
	root := t.TempDir()
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	listOpts.json = false
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "" {
		t.Fatalf("expected no output for empty list, got %q", got)
	}
}

func writeListTestFiles(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.skill.yaml"), []byte("version: 1\nskills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
