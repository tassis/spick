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
)

type declinePrompter struct{}

func (declinePrompter) Select(string, []ui.Option, int) (int, error)          { return 1, nil }
func (declinePrompter) MultiSelect(string, []ui.Option, []int) ([]int, error) { return nil, nil }
func (declinePrompter) MatrixSelect(string, []ui.Option, []ui.Option, map[int][]int) (map[int][]int, error) {
	return nil, nil
}

func TestAddSurfaceOmitsRemovedFlags(t *testing.T) {
	if addCmd.Flags().Lookup("version") != nil {
		t.Fatal("did not expect --version flag")
	}
	if addCmd.Flags().Lookup("yes") != nil {
		t.Fatal("did not expect --yes flag")
	}
	if addCmd.Flags().Lookup("ref") != nil {
		t.Fatal("did not expect --ref flag")
	}
	if addCmd.Flags().Lookup("all") == nil {
		t.Fatal("expected --all flag")
	}
	if addCmd.Flags().Lookup("exposure-method") == nil {
		t.Fatal("expected --exposure-method flag")
	}
	if addCmd.Flags().Lookup("force") == nil {
		t.Fatal("expected --force flag")
	}
}

func TestAddUsesExposureMethodFlag(t *testing.T) {
	if addCmd.Flags().Lookup("exposure-method") == nil {
		t.Fatal("expected --exposure-method flag")
	}
	if addCmd.Flags().Lookup("mode") != nil {
		t.Fatal("did not expect legacy --mode flag")
	}
}

func TestAddSurfaceUsesGlobalFlag(t *testing.T) {
	if addCmd.Flags().Lookup("scope") != nil {
		t.Fatal("did not expect --scope flag")
	}
	if addCmd.Flags().Lookup("global") == nil {
		t.Fatal("expected --global flag")
	}
	if addCmd.Flags().Lookup("g") != nil {
		t.Fatal("expected shorthand only via global flag metadata")
	}
}

func TestAddProjectPreflightInitializesAndContinues(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(initPrompter{}, workspace.New(root), skills.New(root))
	prev := addOpts.global
	addOpts.global = false
	defer func() { addOpts.global = prev }()
	buf := &bytes.Buffer{}
	addCmd.SetOut(buf)
	if err := addCmd.RunE(addCmd, []string{root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "spick.yaml")); err != nil {
		t.Fatalf("expected init to create project config: %v", err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Fatalf("expected add to continue after init, got %q", buf.String())
	}
}

func TestAddProjectPreflightDoesNotShowInitModePicker(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompter := &trackingInitPrompter{}
	appService = app.New(prompter, workspace.New(root), skills.New(root))
	prev := addOpts.global
	addOpts.global = false
	defer func() { addOpts.global = prev }()
	buf := &bytes.Buffer{}
	addCmd.SetOut(buf)
	if err := addCmd.RunE(addCmd, []string{root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "spick.yaml")); err != nil {
		t.Fatalf("expected init to create project config: %v", err)
	}
	if prompter.modePromptSeen {
		t.Fatal("did not expect add preflight to show init mode picker")
	}
}

func TestAddProjectPreflightDeclineStopsBeforeAdd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.res.yaml"), []byte("version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(declinePrompter{}, workspace.New(root), skills.New(root))
	prev := addOpts.global
	addOpts.global = false
	defer func() { addOpts.global = prev }()
	buf := &bytes.Buffer{}
	addCmd.SetOut(buf)
	if err := addCmd.RunE(addCmd, []string{root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "spick.yaml")); err == nil {
		t.Fatalf("expected no init on decline")
	}
	_ = strings.TrimSpace(buf.String())
}
