package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tassis/spick/internal/agents"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
)

func TestResolvePathsProject(t *testing.T) {
	s := New(t.TempDir())
	got, _, err := s.resolvePaths("project", "", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(filepath.Dir(got)) != "skills" || filepath.Base(filepath.Dir(filepath.Dir(got))) != ".spick" {
		t.Fatalf("unexpected path: %s", got)
	}
}

func TestResolvePathsUsesRegistryRoots(t *testing.T) {
	s := New(t.TempDir())
	_, exposure, err := s.resolvePaths("project", "opencode", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exposure != filepath.Join(s.WorkspaceRoot, ".opencode", "skills") {
		t.Fatalf("unexpected exposure root: %s", exposure)
	}
}

func TestRegistryLookupAndValidation(t *testing.T) {
	if _, ok := agents.Lookup("opencode"); !ok {
		t.Fatal("expected known agent")
	}
	if _, ok := agents.Lookup("codex"); !ok {
		t.Fatal("expected codex agent")
	}
	if err := agents.Validate("missing"); err == nil {
		t.Fatal("expected unknown agent rejection")
	}
	if root, err := agents.ExposureRoot(config.ScopeProject, "opencode"); err != nil || root != ".opencode" {
		t.Fatalf("unexpected root: %q %v", root, err)
	}
	if root, err := agents.ExposureRoot(config.ScopeGlobal, "opencode"); err != nil || root != filepath.Join(".config", "opencode") {
		t.Fatalf("unexpected global root: %q %v", root, err)
	}
	if root, err := agents.ExposureRoot(config.ScopeProject, "codex"); err != nil || root != ".agents" {
		t.Fatalf("unexpected codex project root: %q %v", root, err)
	}
	if root, err := agents.ExposureRoot(config.ScopeGlobal, "codex"); err != nil || root != ".agents" {
		t.Fatalf("unexpected codex global root: %q %v", root, err)
	}
}

func TestMaterializeCopy(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "skills", "foo")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "extra.txt"), []byte("more"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), ExposureMethod: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: filepath.Join("skills", "foo")}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Path == "" {
		t.Fatal("expected target path")
	}
	data, err := os.ReadFile(filepath.Join(res.Path, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(res.Path, "extra.txt")); err != nil {
		t.Fatal(err)
	}
	exposure := filepath.Join(root, ".opencode", "skills", "demo")
	if info, err := os.Lstat(exposure); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected copied exposure dir, got %v %v", info, err)
	}
}

func TestMaterializeSymlinkCreatesExposure(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "extra.txt"), []byte("more"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), ExposureMethod: "symlink"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info, err := os.Lstat(res.Path); err != nil || info.IsDir() == false {
		t.Fatalf("expected canonical copied dir: %v %v", info, err)
	}
	exposure := filepath.Join(root, ".opencode", "skills", "demo")
	if info, err := os.Lstat(exposure); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected exposure dir symlink: %v %v", info, err)
	}
}

func TestMaterializeDefaultsToSymlink(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src")}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Mode != "symlink" {
		t.Fatalf("expected default symlink mode, got %q", res.Mode)
	}
	if info, err := os.Lstat(res.Path); err != nil || info.IsDir() == false {
		t.Fatalf("expected canonical path to be copied dir, err=%v mode=%v", err, info.Mode())
	}
}

func TestMaterializeCopyModeCopiesExposure(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), ExposureMethod: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info, err := os.Lstat(res.Path); err != nil || info.IsDir() == false {
		t.Fatalf("expected canonical copied dir, got %v %v", info, err)
	}
	exposure := filepath.Join(root, ".opencode", "skills", "demo")
	if info, err := os.Lstat(exposure); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected exposure copied dir, got %v %v", info, err)
	}
}

func TestMaterializeFailsOnExistingDestinationWithoutForce(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), ExposureMethod: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), ExposureMethod: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}}); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestMaterializeForceOverwrites(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), ExposureMethod: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), ExposureMethod: "copy", Force: true}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}}); err != nil {
		t.Fatalf("unexpected force error: %v", err)
	}
}
