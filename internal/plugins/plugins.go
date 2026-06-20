package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tassis/spick/internal/model"
	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Version int            `yaml:"version"`
	Plugin  ManifestPlugin `yaml:"plugin"`
}

type ManifestPlugin struct {
	ID          string `yaml:"id"`
	Runtime     string `yaml:"runtime"`
	Entry       string `yaml:"entry"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func LoadManifest(root string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, "spick.res.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("load plugin manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse plugin manifest: %w", err)
	}
	if m.Version != 1 {
		return nil, fmt.Errorf("unsupported plugin manifest version %d", m.Version)
	}
	if m.Plugin.ID == "" || m.Plugin.Runtime == "" || m.Plugin.Entry == "" {
		return nil, fmt.Errorf("plugin.id, plugin.runtime, and plugin.entry are required")
	}
	return &m, nil
}

func MetadataFromManifest(m *Manifest) model.PluginMetadata {
	if m == nil {
		return model.PluginMetadata{}
	}
	return model.PluginMetadata{Name: m.Plugin.Name, Description: m.Plugin.Description}
}
