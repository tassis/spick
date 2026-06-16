package lock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tassis/spick/internal/model"
)

func TestLockRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	lf := &model.Lockfile{Version: 1, Scope: "project", Skills: map[string]model.Skill{"demo": {ID: "demo", Install: &model.SkillInstall{Mode: "copy", CanonicalPath: ".skills/demo"}}}}
	if err := s.Write("project", lf); err != nil { t.Fatal(err) }
	got, err := s.Read("project")
	if err != nil { t.Fatal(err) }
	if got.Version != 1 || got.Scope != "project" || got.Skills["demo"].ID != "demo" { t.Fatalf("unexpected roundtrip: %+v", got) }
	if _, err := os.Stat(filepath.Join(root, "spick.lock")); err != nil { t.Fatal(err) }
}

func TestLockNormalizesProjectPaths(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	lf := &model.Lockfile{Version: 1, Scope: "project", Skills: map[string]model.Skill{"demo": {ID: "demo", Source: &model.SkillSource{Path: filepath.Join(root, ".skills", "demo")}, Install: &model.SkillInstall{CanonicalPath: filepath.Join(root, ".skills", "demo")}, Exposures: []model.Exposure{{Agent: "opencode", Path: filepath.Join(root, ".opencode", "skills", "demo")}}}}}
	if err := s.Write("project", lf); err != nil { t.Fatal(err) }
	data, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil { t.Fatal(err) }
	var got model.Lockfile
	if err := json.Unmarshal(data, &got); err != nil { t.Fatal(err) }
	if got.Skills["demo"].Install == nil || got.Skills["demo"].Install.CanonicalPath != ".skills/demo" { t.Fatalf("unexpected canonical path: %+v", got) }
	if got.Skills["demo"].Exposures[0].Path != ".opencode/skills/demo" { t.Fatalf("unexpected exposure path: %+v", got) }
}

func TestRemoveFilesMissingLockfileReturnsFriendlyError(t *testing.T) {
	s := New(t.TempDir())
	if err := s.RemoveFiles("project", []string{"demo"}, false, false); err == nil || err.Error() != "no installed skills" {
		t.Fatalf("expected friendly missing-state error, got %v", err)
	}
}
