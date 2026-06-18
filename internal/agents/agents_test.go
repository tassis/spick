package agents

import (
	"path/filepath"
	"testing"

	"github.com/tassis/spick/internal/config"
)

func TestLookupKnownAgent(t *testing.T) {
	a, ok := Lookup("opencode")
	if !ok {
		t.Fatal("expected known agent")
	}
	if a.Name != "opencode" {
		t.Fatalf("unexpected agent: %+v", a)
	}
}

func TestLookupCodexAgent(t *testing.T) {
	a, ok := Lookup("codex")
	if !ok {
		t.Fatal("expected codex agent")
	}
	if a.ProjectExposureRoot != ".agents" || a.GlobalExposureRoot != ".agents" {
		t.Fatalf("unexpected codex roots: %+v", a)
	}
}

func TestValidateUnknownAgent(t *testing.T) {
	if err := Validate("nosuch"); err == nil {
		t.Fatal("expected error")
	}
}

func TestExposureRootPerScope(t *testing.T) {
	project, err := ExposureRoot(config.ScopeProject, "opencode")
	if err != nil || project != ".opencode" {
		t.Fatalf("unexpected project root: %q %v", project, err)
	}
	global, err := ExposureRoot(config.ScopeGlobal, "opencode")
	if err != nil || global != filepath.Join(".config", "opencode") {
		t.Fatalf("unexpected global root: %q %v", global, err)
	}
	if project, err := ExposureRoot(config.ScopeProject, "codex"); err != nil || project != ".agents" {
		t.Fatalf("unexpected codex project root: %q %v", project, err)
	}
	if global, err := ExposureRoot(config.ScopeGlobal, "codex"); err != nil || global != ".agents" {
		t.Fatalf("unexpected codex global root: %q %v", global, err)
	}
}
