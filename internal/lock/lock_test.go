package lock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tassis/spick/internal/agents"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
)

func TestLockRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	lf := &model.Lockfile{Version: 1, Scope: "project", Skills: []model.LockSkill{{ID: "demo", Declared: model.LockDeclared{Source: "local"}, Resolved: model.LockResolved{Source: "local", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: ".spick/skills/demo"}, Projected: model.LockProjected{Mode: "copy"}}}}
	if err := s.Write("project", lf); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read("project")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.Scope != "project" || len(got.Skills) != 1 || got.Skills[0].ID != "demo" {
		t.Fatalf("unexpected roundtrip: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "spick.lock")); err != nil {
		t.Fatal(err)
	}
}

func TestLockNormalizesProjectPaths(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	lf := &model.Lockfile{Version: 1, Scope: "project", Skills: []model.LockSkill{{ID: "demo", Declared: model.LockDeclared{Source: "local"}, Resolved: model.LockResolved{Source: "local", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "skills", "demo")}, Projected: model.LockProjected{Mode: "symlink", Exposures: []model.Exposure{{Agent: "opencode", Path: filepath.Join(root, ".opencode", "skills", "demo")}}}}}}
	if err := s.Write("project", lf); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var got model.Lockfile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Skills[0].Materialized.Path != ".spick/skills/demo" {
		t.Fatalf("unexpected canonical path: %+v", got)
	}
	if got.Skills[0].Projected.Exposures[0].Path != ".opencode/skills/demo" {
		t.Fatalf("unexpected exposure path: %+v", got)
	}
}

func TestExposurePathForUsesRegistry(t *testing.T) {
	s := New(t.TempDir())
	if got := s.ExposurePathFor("project", "demo"); got != filepath.Join(s.Root, ".opencode", "skills", "demo") {
		t.Fatalf("unexpected project exposure: %s", got)
	}
	if got := s.ExposurePathFor(string(config.ScopeGlobal), "demo"); got != filepath.Join(userHome(), ".config", "opencode", "skills", "demo") {
		t.Fatalf("unexpected global exposure: %s", got)
	}
	if _, ok := agents.Lookup("opencode"); !ok {
		t.Fatal("expected known agent")
	}
}

func TestRemoveFilesMissingLockfileReturnsFriendlyError(t *testing.T) {
	s := New(t.TempDir())
	if err := s.RemoveFiles("project", []string{"demo"}, false); err == nil || err.Error() != "no installed skills" {
		t.Fatalf("expected friendly missing-state error, got %v", err)
	}
}

func TestLockReadsCanonicalSnapshotLayout(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	lf := &model.Lockfile{Version: 1, Scope: "project", Skills: []model.LockSkill{{ID: "demo", Declared: model.LockDeclared{Source: "github:owner/repo", Ref: "main"}, Resolved: model.LockResolved{Source: "github:owner/repo", Ref: "main", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: ".spick/skills/demo"}, Projected: model.LockProjected{Mode: "symlink", Exposures: []model.Exposure{{Agent: "opencode", Path: ".opencode/skills/demo"}}}}}}
	if err := s.Write("project", lf); err != nil {
		t.Fatal(err)
	}

	got, err := s.Read("project")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 1 {
		t.Fatalf("expected one skill, got %+v", got.Skills)
	}
	if got.Skills[0].ID != "demo" {
		t.Fatalf("unexpected id: %+v", got.Skills[0])
	}
	if got.Skills[0].Declared.Source != "github:owner/repo" || got.Skills[0].Declared.Ref != "main" {
		t.Fatalf("unexpected declared state: %+v", got.Skills[0].Declared)
	}
	if got.Skills[0].Materialized.Path != ".spick/skills/demo" || len(got.Skills[0].Projected.Exposures) != 1 {
		t.Fatalf("unexpected installed state: %+v", got.Skills[0].Materialized)
	}
}

func TestLockStoresMaterializedStateWithoutProjectIntent(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	lf := &model.Lockfile{Version: 1, Scope: "project", Skills: []model.LockSkill{{ID: "demo", Resolved: model.LockResolved{Source: "github:owner/repo", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: ".spick/skills/demo"}, Projected: model.LockProjected{Mode: "copy", Exposures: []model.Exposure{{Agent: "opencode", Path: ".opencode/skills/demo"}}}}}}
	if err := s.Write("project", lf); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read("project")
	if err != nil {
		t.Fatal(err)
	}
	if got.Skills[0].Declared.Source != "" {
		t.Fatalf("expected no declared source in materialized-only lockfile, got %+v", got.Skills[0].Declared)
	}
	if got.Skills[0].Resolved.Source != "github:owner/repo" || got.Skills[0].Materialized.Path != ".spick/skills/demo" {
		t.Fatalf("unexpected materialized state: %+v", got.Skills[0])
	}
}

func TestManagedPathsUseSpickLayout(t *testing.T) {
	s := New("/project")
	if got := s.ManagedSkillPathFor("project", "demo"); got != filepath.Join("/project", ".spick", "skills", "demo") {
		t.Fatalf("unexpected skill path: %s", got)
	}
	if got := s.ManagedPluginPathFor("project", "demo"); got != filepath.Join("/project", ".spick", "plugins", "demo") {
		t.Fatalf("unexpected plugin path: %s", got)
	}
}
