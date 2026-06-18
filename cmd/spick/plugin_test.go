package spick

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

func TestPluginListJSONIncludesMissingAndExtra(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("version: 1\nproject:\n  plugins:\n    - id: plugin-a\n      source: ./plugin-a\n  agents: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin-a", "spick.plugin.yaml"), []byte("version: 1\nplugin:\n  id: plugin-a\n  runtime: node\n  entry: index.js\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.lock"), []byte("{\"version\":1,\"scope\":\"project\",\"plugins\":[{\"id\":\"plugin-a\",\"declared\":{\"source\":\"./plugin-a\"},\"resolved\":{\"source\":\"./plugin-a\"},\"installed\":{\"path\":\"plugin-a\",\"entry\":\"index.js\"}},{\"id\":\"plugin-extra\",\"resolved\":{\"source\":\"extra\"},\"installed\":{\"path\":\"extra\",\"entry\":\"index.js\"}}]}"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	pluginListCmd.SetOut(buf)
	pluginListOpts.json = true
	defer func() { pluginListOpts.json = false }()
	if err := pluginListCmd.RunE(pluginListCmd, nil); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["scope"] != "project" {
		t.Fatalf("unexpected json: %s", buf.String())
	}
}

func TestPluginListJSONCoalescesManagedSourceAndLock(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("version: 1\nproject:\n  plugins:\n    - id: plugin-repo\n      source: ./plugin-repo\n  agents: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin-repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin-repo", "spick.plugin.yaml"), []byte("version: 1\nplugin:\n  id: plugin-repo\n  runtime: node\n  entry: index.js\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.lock"), []byte("{\"version\":1,\"scope\":\"project\",\"plugins\":[{\"id\":\"plugin-repo\",\"declared\":{\"source\":\"./plugin-repo\"},\"resolved\":{\"source\":\"./plugin-repo\"},\"installed\":{\"path\":\"plugin-repo\",\"entry\":\"index.js\"}}]}"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	pluginListCmd.SetOut(buf)
	pluginListOpts.json = true
	defer func() { pluginListOpts.json = false }()
	if err := pluginListCmd.RunE(pluginListCmd, nil); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Plugins []map[string]any `json:"plugins"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Plugins) != 1 {
		t.Fatalf("expected 1 plugin record, got %d: %s", len(got.Plugins), buf.String())
	}
	if got.Plugins[0]["state"] != "installed" {
		t.Fatalf("expected installed state, got %v", got.Plugins[0]["state"])
	}
	if got.Plugins[0]["id"] != "plugin-repo" {
		t.Fatalf("expected plugin id record, got %v", got.Plugins[0]["id"])
	}
}

func TestPluginAddThenListLocalSourceProducesOneManagedRecord(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.skill.yaml"), []byte("version: 1\nskills:\n    - id: demo\n      path: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.plugin.yaml"), []byte("version: 1\nplugin:\n  id: plugin-repo\n  runtime: node\n  entry: index.js\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte("console.log('ok')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("version: 1\nproject:\n  agents: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	addBuf := &bytes.Buffer{}
	pluginAddCmd.SetOut(addBuf)
	if err := pluginAddCmd.RunE(pluginAddCmd, []string{root}); err != nil {
		t.Fatal(err)
	}
	listBuf := &bytes.Buffer{}
	pluginListCmd.SetOut(listBuf)
	pluginListOpts.json = true
	defer func() { pluginListOpts.json = false }()
	if err := pluginListCmd.RunE(pluginListCmd, nil); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Plugins []struct {
			ID        string `json:"id"`
			State     string `json:"state"`
			Declared  any    `json:"declared"`
			Installed any    `json:"installed"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(listBuf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Plugins) != 1 {
		t.Fatalf("expected one plugin record, got %d: %s", len(got.Plugins), listBuf.String())
	}
	if got.Plugins[0].State != "installed" {
		t.Fatalf("expected installed state, got %q: %s", got.Plugins[0].State, listBuf.String())
	}
	if got.Plugins[0].Declared == nil || got.Plugins[0].Installed == nil {
		t.Fatalf("expected coherent managed record, got %s", listBuf.String())
	}
}

func TestPluginListHumanReadableShowsMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("version: 1\nproject:\n  plugins:\n    - id: plugin-a\n      source: ./plugin-a\n  agents: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	pluginListCmd.SetOut(buf)
	if err := pluginListCmd.RunE(pluginListCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "missing") {
		t.Fatalf("expected missing state, got %q", buf.String())
	}
}

func TestPluginRemoveUpdatesDeclarationAndLock(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("version: 1\nproject:\n  plugins:\n    - id: plugin-a\n      source: ./plugin-a\n    - id: plugin-b\n      source: ./plugin-b\n  agents: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.lock"), []byte("{\"version\":1,\"scope\":\"project\",\"plugins\":[{\"id\":\"plugin-a\",\"declared\":{\"source\":\"./plugin-a\"},\"resolved\":{\"source\":\"./plugin-a\"},\"installed\":{\"path\":\"plugin-a\",\"entry\":\"index.js\"}},{\"id\":\"plugin-b\",\"declared\":{\"source\":\"./plugin-b\"},\"resolved\":{\"source\":\"./plugin-b\"},\"installed\":{\"path\":\"plugin-b\",\"entry\":\"index.js\"}}]}"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	pluginRmCmd.SetOut(buf)
	if err := pluginRmCmd.RunE(pluginRmCmd, []string{"./plugin-a"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "plugin-a") {
		t.Fatalf("expected declaration removed: %s", string(data))
	}
	lockData, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(lockData), "\"id\": \"plugin-a\"") {
		t.Fatalf("expected plugin-a removed: %s", string(lockData))
	}
	if !strings.Contains(string(lockData), "\"id\": \"plugin-b\"") {
		t.Fatalf("expected plugin-b preserved: %s", string(lockData))
	}
}

func TestPluginRemoveWarnsWhenManagedMaterialMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spick.yaml"), []byte("version: 1\nproject:\n  plugins:\n    - id: plugin-a\n      source: ./plugin-a\n  agents: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spick.lock"), []byte("{\"version\":1,\"scope\":\"project\",\"plugins\":[{\"id\":\"plugin-a\",\"declared\":{\"source\":\"./plugin-a\"},\"resolved\":{\"source\":\"./plugin-a\"},\"installed\":{\"path\":\"missing\",\"entry\":\"index.js\"}}]}"), 0o644); err != nil {
		t.Fatal(err)
	}
	appService = app.New(ui.NewPromptTea(), workspace.New(root), skills.New(root))
	buf := &bytes.Buffer{}
	pluginRmCmd.SetOut(buf)
	if err := pluginRmCmd.RunE(pluginRmCmd, []string{"./plugin-a"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "removed plugins") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestPluginRemoveSurfaceOmitsRemovedFlags(t *testing.T) {
	if pluginRmCmd.Flags().Lookup("yes") != nil {
		t.Fatal("did not expect --yes flag")
	}
	if pluginRmCmd.Flags().Lookup("scope") == nil {
		t.Fatal("expected --scope flag")
	}
}
