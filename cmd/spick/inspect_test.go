package spick

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

func TestInspectCommandHumanReadable(t *testing.T) {
	root := t.TempDir()
	writeInspectTestFiles(t, root)
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	inspectOpts.json = false
	buf := &bytes.Buffer{}
	inspectCmd.SetOut(buf)
	if err := inspectCmd.RunE(inspectCmd, []string{root}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "demo") {
		t.Fatalf("expected human readable output, got %q", buf.String())
	}
}

func TestInspectCommandJSON(t *testing.T) {
	root := t.TempDir()
	writeInspectTestFiles(t, root)
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	inspectOpts.json = true
	buf := &bytes.Buffer{}
	inspectCmd.SetOut(buf)
	if err := inspectCmd.RunE(inspectCmd, []string{root}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"skills"`) {
		t.Fatalf("expected JSON output, got %q", buf.String())
	}
}

func TestInspectCommandOmitsRefFlag(t *testing.T) {
	if inspectCmd.Flags().Lookup("ref") != nil {
		t.Fatal("did not expect --ref flag")
	}
}

func TestInspectCommandAcceptsInlineRef(t *testing.T) {
	base := t.TempDir()
	createHostedInspectRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	appService = app.New(ui.NewPromptTea(), workspace.New(t.TempDir()), skills.New(t.TempDir()))
	inspectOpts.json = false
	inspectOpts.scope = string(config.ScopeProject)
	buf := &bytes.Buffer{}
	inspectCmd.SetOut(buf)
	if err := inspectCmd.RunE(inspectCmd, []string{"github:owner/repo@main"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "demo") {
		t.Fatalf("expected ref-based output, got %q", buf.String())
	}
}

func TestInspectCommandTreatsLocalAtAsPath(t *testing.T) {
	local := t.TempDir() + "@main"
	writeInspectTestFiles(t, local)
	appService = app.New(ui.NewPromptTea(), workspace.New(local), skills.New(local))
	inspectOpts.json = false
	inspectOpts.scope = string(config.ScopeProject)
	buf := &bytes.Buffer{}
	inspectCmd.SetOut(buf)
	if err := inspectCmd.RunE(inspectCmd, []string{local}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "demo") {
		t.Fatalf("expected local path output, got %q", buf.String())
	}
}

func TestInspectCommandTreatsLocalAtAsPathRejectedByPathLookup(t *testing.T) {
	root := t.TempDir()
	writeInspectTestFiles(t, root)
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	inspectOpts.json = false
	inspectOpts.scope = string(config.ScopeProject)
	buf := &bytes.Buffer{}
	inspectCmd.SetOut(buf)
	if err := inspectCmd.RunE(inspectCmd, []string{root + "@main"}); err == nil {
		t.Fatal("expected local @ path to be treated as a literal path and fail")
	}
}

func writeInspectTestFiles(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, "spick.yaml")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.skill.yaml"), []byte("version: 1\nskills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func createHostedInspectRepo(t *testing.T, base, rel string) {
	t.Helper()
	repo := filepath.Join(base, rel)
	work := filepath.Join(base, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInspectTestFiles(t, work)
	mustRunInspect(t, work, "git", "init")
	mustRunInspect(t, work, "git", "config", "user.email", "test@example.com")
	mustRunInspect(t, work, "git", "config", "user.name", "Test")
	mustRunInspect(t, work, "git", "add", ".")
	mustRunInspect(t, work, "git", "commit", "-m", "init")
	mustRunInspect(t, work, "git", "branch", "-M", "main")
	mustRunInspect(t, work, "git", "clone", "--bare", work, repo)
}

func mustRunInspect(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, string(out))
	}
}
