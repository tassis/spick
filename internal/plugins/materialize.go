package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tassis/spick/internal/model"
)

type MaterializeOptions struct {
	WorkspaceRoot string
	Scope         string
	SourceRoot    string
	ID            string
	Force         bool
}

type MaterializeResult struct {
	ManagedPath string
	RuntimePath string
	Plugin      *Manifest
	Metadata    model.PluginMetadata
}

func ManagedPath(workspaceRoot, scope, id string) string {
	base := workspaceRoot
	if base == "" {
		base = "."
	}
	if scope == "global" {
		return filepath.Join(userHome(), ".spick", "plugins", id)
	}
	return filepath.Join(base, ".spick", "plugins", id)
}

func Materialize(opts MaterializeOptions) (*MaterializeResult, error) {
	manifest, err := LoadManifest(opts.SourceRoot)
	if err != nil {
		return nil, err
	}
	managed := ManagedPath(opts.WorkspaceRoot, opts.Scope, opts.ID)
	if !opts.Force {
		if _, err := os.Lstat(managed); err == nil {
			return nil, fmt.Errorf("destination exists: %s", managed)
		}
	}
	if err := os.RemoveAll(managed); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(managed), 0o755); err != nil {
		return nil, err
	}
	if err := copyDir(opts.SourceRoot, managed); err != nil {
		return nil, err
	}
	return &MaterializeResult{ManagedPath: managed, RuntimePath: managed, Plugin: manifest, Metadata: MetadataFromManifest(manifest)}, nil
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "."
	}
	return h
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if entry.Name() == ".spick" {
				continue
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
