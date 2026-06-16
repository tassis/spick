package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

import "strings"

func TestParseSourceGitHub(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("github:owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "github" || parsed.Source.Locator != "github:owner/repo" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceGitLab(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("gitlab:group/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "gitlab" || parsed.Source.Locator != "gitlab:group/repo" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceGitLabSubgroup(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("gitlab:group/subgroup/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "gitlab" || parsed.Source.Locator != "gitlab:group/subgroup/repo" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceLocalPath(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("./skills")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "local" || parsed.Source.Path != "skills" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceRejectsMalformedHosted(t *testing.T) {
	w := New("/tmp")
	if _, err := w.ParseSource("github:owner"); err == nil {
		t.Fatal("expected error for malformed github source")
	}
	if _, err := w.ParseSource("gitlab:group"); err == nil {
		t.Fatal("expected error for malformed gitlab source")
	}
}

func TestParseSourceRejectsEmpty(t *testing.T) {
	w := New("/tmp")
	if _, err := w.ParseSource("   "); err == nil {
		t.Fatal("expected error for empty source")
	}
}

func TestLoadCatalogValidManifest(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "catalog:\n  skills:\n    - id: demo\n      path: .\n      name: Demo\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "demo" || got[0].Source == nil || got[0].Source.Path != "." {
		t.Fatalf("unexpected catalog: %+v", got)
	}
}

func TestLoadCatalogMissingManifest(t *testing.T) {
	loader := &Loader{Root: t.TempDir()}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected missing manifest error")
	}
}

func TestLoadCatalogDuplicateID(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "catalog:\n  skills:\n    - id: demo\n      path: .\n    - id: demo\n      path: .\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestLoadCatalogInvalidID(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "catalog:\n  skills:\n    - id: Bad\n      path: .\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestLoadCatalogPathEscapeRejected(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Dir(root)
	mustWrite(t, filepath.Join(root, "spick.yaml"), "catalog:\n  skills:\n    - id: demo\n      path: ../other\n")
	mustWrite(t, filepath.Join(parent, "SKILL.md"), "# other\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestLoadCatalogMissingSkillMD(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "catalog:\n  skills:\n    - id: demo\n      path: .\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected missing SKILL.md error")
	}
}

func TestManifestVersionDefaultsToOne(t *testing.T) {
	m, err := parseManifest([]byte(strings.TrimSpace("catalog:\n  skills:\n    - id: demo\n      path: .\n")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Version != 1 {
		t.Fatalf("expected default version 1, got %d", m.Version)
	}
}

func TestDiscoverCatalogRootSkill(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# root\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != filepath.Base(root) || got[0].Source == nil || got[0].Source.Path != "." {
		t.Fatalf("unexpected root discovery: %+v", got)
	}
}

func TestDiscoverCatalogMultiSkill(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "alpha", "SKILL.md"), "# alpha\n")
	mustWrite(t, filepath.Join(root, "beta", "SKILL.md"), "# beta\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 skills, got %+v", got)
	}
}

func TestLoadCatalogManifestOverridesDiscovery(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "catalog:\n  skills:\n    - id: demo\n      path: .\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# root\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "demo" {
		t.Fatalf("expected manifest-backed result, got %+v", got)
	}
}

func TestDiscoverCatalogSkipsNoiseDirs(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".git", "SKILL.md"), "# git\n")
	mustWrite(t, filepath.Join(root, "node_modules", "SKILL.md"), "# node\n")
	mustWrite(t, filepath.Join(root, "vendor", "SKILL.md"), "# vendor\n")
	mustWrite(t, filepath.Join(root, ".skills", "SKILL.md"), "# skills\n")
	mustWrite(t, filepath.Join(root, ".opencode", "SKILL.md"), "# opencode\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err == nil || len(got) != 0 {
		t.Fatalf("expected no discoveries from noise dirs, got %+v err=%v", got, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
