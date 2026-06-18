package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.plugin.yaml"), "version: 1\nplugin:\n  id: demo\n  runtime: node\n  entry: index.js\n  name: Demo\n  description: Example\n")
	m, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Version != 1 || m.Plugin.ID != "demo" || m.Plugin.Runtime != "node" || m.Plugin.Entry != "index.js" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestLoadManifestRequiresSchemaV1Fields(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.plugin.yaml"), "version: 1\nplugin:\n  id: demo\n  runtime: node\n")
	if _, err := LoadManifest(root); err == nil {
		t.Fatal("expected required field error")
	}
}

func TestLoadManifestRejectsUnsupportedVersion(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.plugin.yaml"), "version: 2\nplugin:\n  id: demo\n  runtime: node\n  entry: index.js\n")
	if _, err := LoadManifest(root); err == nil {
		t.Fatal("expected version error")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
