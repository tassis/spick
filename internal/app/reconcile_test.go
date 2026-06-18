package app

import (
	"path/filepath"
	"testing"

	"github.com/tassis/spick/internal/lock"
	"github.com/tassis/spick/internal/model"
	"github.com/tassis/spick/internal/workspace"
)

func TestSkillReconcileInputsUseConfigAndLockfile(t *testing.T) {
	root := t.TempDir()
	project := &workspace.ProjectConfig{
		Skills: []model.ProjectSkill{{ID: "declared", Source: "./skill"}},
		Agents: map[string]model.ProjectAgentEnablement{"opencode": {Skills: []string{"declared"}}},
	}
	lockStore := &lock.Store{Root: root}
	if err := lockStore.UpsertInstalled("project", []model.InstalledSkill{{ID: "materialized", Install: &model.SkillInstall{CanonicalPath: filepath.Join(root, ".spick", "skills", "materialized"), Mode: "symlink"}}}); err != nil {
		t.Fatal(err)
	}
	inputs, err := skillReconcileInputs(project, lockStore, "project")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs.Declared) != 1 || inputs.Declared[0].ID != "declared" {
		t.Fatalf("expected declared config input, got %+v", inputs.Declared)
	}
	if len(inputs.Materialized) != 1 || inputs.Materialized[0].ID != "materialized" {
		t.Fatalf("expected lockfile materialized input, got %+v", inputs.Materialized)
	}
	inputs.Enabled["opencode"] = model.ProjectAgentEnablement{Skills: []string{"mutated"}}
	if project.Agents["opencode"].Skills[0] != "declared" {
		t.Fatalf("expected config enablement to be cloned, got %+v", project.Agents)
	}
	actions := planSkillReconcile(inputs)
	if len(actions) != 2 {
		t.Fatalf("expected two reconcile actions, got %d", len(actions))
	}
	if actions[0].ID != "declared" || actions[0].Declared == nil || actions[0].Materialized != nil {
		t.Fatalf("expected declared-only action, got %+v", actions[0])
	}
	if actions[1].ID != "materialized" || actions[1].Declared != nil || actions[1].Materialized == nil {
		t.Fatalf("expected materialized-only action, got %+v", actions[1])
	}
}

func TestPluginReconcileInputsUseConfigAndLockfile(t *testing.T) {
	root := t.TempDir()
	project := &workspace.ProjectConfig{Plugins: []model.ProjectPlugin{{ID: "plugin-a", Source: "./plugin"}}, Agents: map[string]model.ProjectAgentEnablement{"opencode": {Plugins: []string{"plugin-a"}}}}
	lockStore := &lock.Store{Root: root}
	if err := lockStore.UpsertPlugins("project", []model.LockPlugin{{ID: "plugin-b", Materialized: model.LockMaterialized{Path: filepath.Join(root, "plugin-b")}, Projected: model.LockPluginProjected{Path: filepath.Join(root, "plugin-b")}}}); err != nil {
		t.Fatal(err)
	}
	inputs, err := pluginReconcileInputs(project, lockStore, "project")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs.Declared) != 1 || inputs.Declared[0].ID != "plugin-a" {
		t.Fatalf("expected declared plugin input, got %+v", inputs.Declared)
	}
	if len(inputs.Materialized) != 1 || inputs.Materialized[0].ID != "plugin-b" {
		t.Fatalf("expected lockfile plugin input, got %+v", inputs.Materialized)
	}
	inputs.Enabled["opencode"] = model.ProjectAgentEnablement{Plugins: []string{"mutated"}}
	if project.Agents["opencode"].Plugins[0] != "plugin-a" {
		t.Fatalf("expected config enablement to be cloned, got %+v", project.Agents)
	}
	actions := planPluginReconcile(inputs)
	if len(actions) != 2 {
		t.Fatalf("expected two reconcile actions, got %d", len(actions))
	}
	if actions[0].ID != "plugin-a" || actions[0].Declared == nil || actions[0].Materialized != nil {
		t.Fatalf("expected declared-only plugin action, got %+v", actions[0])
	}
	if actions[1].ID != "plugin-b" || actions[1].Declared != nil || actions[1].Materialized == nil {
		t.Fatalf("expected materialized-only plugin action, got %+v", actions[1])
	}
}
