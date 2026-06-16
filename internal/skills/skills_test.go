package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tassis/spick/internal/model"
)

func TestResolvePathsProject(t *testing.T) {
	s := New(t.TempDir())
	got, _, err := s.resolvePaths("project", "", "copy", "demo")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if filepath.Base(filepath.Dir(got)) != ".skills" { t.Fatalf("unexpected path: %s", got) }
}

func TestMaterializeCopy(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "skills", "foo")
	if err := os.MkdirAll(srcDir, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(srcDir, "extra.txt"), []byte("more"), 0o644); err != nil { t.Fatal(err) }
	res, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), Mode: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: filepath.Join("skills", "foo")}})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if res.Path == "" { t.Fatal("expected target path") }
	data, err := os.ReadFile(filepath.Join(res.Path, "SKILL.md"))
	if err != nil { t.Fatal(err) }
	if string(data) != "hello" { t.Fatalf("unexpected content: %q", string(data)) }
	if _, err := os.Stat(filepath.Join(res.Path, "extra.txt")); err != nil { t.Fatal(err) }
	exposure := filepath.Join(root, ".opencode", "skills", "demo")
	if info, err := os.Lstat(exposure); err != nil || info.Mode()&os.ModeSymlink != 0 { t.Fatalf("expected copied exposure dir, got %v %v", info, err) }
}

func TestMaterializeSymlinkCreatesExposure(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(srcDir, "extra.txt"), []byte("more"), 0o644); err != nil { t.Fatal(err) }
	res, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), Mode: "symlink"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if info, err := os.Lstat(res.Path); err != nil || info.IsDir() == false { t.Fatalf("expected canonical copied dir: %v %v", info, err) }
	exposure := filepath.Join(root, ".opencode", "skills", "demo")
	if info, err := os.Lstat(exposure); err != nil || info.Mode()&os.ModeSymlink == 0 { t.Fatalf("expected exposure dir symlink: %v %v", info, err) }
}

func TestMaterializeDefaultsToSymlink(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil { t.Fatal(err) }
	res, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src")}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if res.Mode != "symlink" { t.Fatalf("expected default symlink mode, got %q", res.Mode) }
	if info, err := os.Lstat(res.Path); err != nil || info.IsDir() == false {
		t.Fatalf("expected canonical path to be copied dir, err=%v mode=%v", err, info.Mode())
	}
}

func TestMaterializeCopyModeCopiesExposure(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil { t.Fatal(err) }
	res, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), Mode: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if info, err := os.Lstat(res.Path); err != nil || info.IsDir() == false { t.Fatalf("expected canonical copied dir, got %v %v", info, err) }
	exposure := filepath.Join(root, ".opencode", "skills", "demo")
	if info, err := os.Lstat(exposure); err != nil || info.Mode()&os.ModeSymlink != 0 { t.Fatalf("expected exposure copied dir, got %v %v", info, err) }
}

func TestMaterializeFailsOnExistingDestinationWithoutForce(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil { t.Fatal(err) }
	if _, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), Mode: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}}); err != nil { t.Fatal(err) }
	if _, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), Mode: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}}); err == nil { t.Fatal("expected conflict error") }
}

func TestMaterializeForceOverwrites(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	srcDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(srcDir, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("hello"), 0o644); err != nil { t.Fatal(err) }
	if _, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), Mode: "copy"}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}}); err != nil { t.Fatal(err) }
	if _, err := s.materializeSkill(AddOptions{Scope: "project", SourceRoot: filepath.Join(root, "src"), Mode: "copy", Force: true}, model.CatalogSkill{ID: "demo", Source: &model.Source{Path: "nested"}}); err != nil { t.Fatalf("unexpected force error: %v", err) }
}
