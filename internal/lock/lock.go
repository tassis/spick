package lock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tassis/spick/internal/model"
)

type Store struct {
	Root string
}

func New(root string) *Store { return &Store{Root: root} }

func (s *Store) pathFor(scope string) string {
	if scope == "global" {
		return filepath.Join(userHome(), ".spick", "spick.lock")
	}
	root := s.Root
	if root == "" { root = "." }
	return filepath.Join(root, "spick.lock")
}

func (s *Store) Read(scope string) (*model.Lockfile, error) {
	data, err := os.ReadFile(s.pathFor(scope))
	if err != nil { return nil, err }
	var lf model.Lockfile
	if err := json.Unmarshal(data, &lf); err != nil { return nil, fmt.Errorf("parse lockfile: %w", err) }
	if lf.Version == 0 { lf.Version = 1 }
	if lf.Skills == nil { lf.Skills = map[string]model.Skill{} }
	if lf.Scope == "" { lf.Scope = scope }
	return &lf, nil
}

func (s *Store) Write(scope string, lf *model.Lockfile) error {
	if lf == nil { return fmt.Errorf("lockfile is required") }
	if lf.Version == 0 { lf.Version = 1 }
	if lf.Scope == "" { lf.Scope = scope }
	normalizeLockfilePaths(s.Root, scope, lf)
	path := s.pathFor(scope)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return err }
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil { return err }
	tmp, err := os.CreateTemp(filepath.Dir(path), ".spick-*.tmp")
	if err != nil { return err }
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil { tmp.Close(); _ = os.Remove(tmpPath); return err }
	if err := tmp.Close(); err != nil { _ = os.Remove(tmpPath); return err }
	if err := os.Rename(tmpPath, path); err != nil { _ = os.Remove(tmpPath); return err }
	return nil
}

func (s *Store) UpsertInstalled(scope string, skills []model.InstalledSkill) error {
	lf, err := s.Read(scope)
	if err != nil && !os.IsNotExist(err) { return err }
	if lf == nil { lf = &model.Lockfile{Version: 1, Scope: scope, Skills: map[string]model.Skill{}} }
	if lf.Skills == nil { lf.Skills = map[string]model.Skill{} }
	for _, sk := range skills {
		lf.Skills[sk.ID] = model.Skill{ID: sk.ID, Source: normalizeSkillSource(s.Root, scope, sk.Source), Install: normalizeSkillInstall(s.Root, scope, sk.Install), Exposures: normalizeExposures(s.Root, scope, sk.Exposures)}
	}
	return s.Write(scope, lf)
}

func (s *Store) List(scope string) ([]model.InstalledSkill, error) {
	lf, err := s.Read(scope)
	if err != nil { return nil, err }
	ids := make([]string, 0, len(lf.Skills))
	for id := range lf.Skills { ids = append(ids, id) }
	sort.Strings(ids)
	out := make([]model.InstalledSkill, 0, len(ids))
	for _, id := range ids {
		sk := lf.Skills[id]
		out = append(out, model.InstalledSkill{ID: sk.ID, Source: sk.Source, Install: sk.Install, Exposures: sk.Exposures})
	}
	return out, nil
}

func (s *Store) Remove(scope string, ids []string) (*model.Lockfile, error) {
	lf, err := s.Read(scope)
	if err != nil { return nil, err }
	for _, id := range ids {
		delete(lf.Skills, id)
	}
	return lf, nil
}

func (s *Store) InUse(scope string, id string) bool {
	lf, err := s.Read(scope)
	if err != nil { return false }
	_, ok := lf.Skills[id]
	return ok
}

func (s *Store) CanonicalPathFor(scope, id string) string {
	base := s.Root
	if base == "" { base = "." }
	if scope == "global" { return filepath.Join(userHome(), ".spick", "skills", id) }
	return filepath.Join(base, ".skills", id)
}

func (s *Store) ExposurePathFor(scope, id string) string {
	base := s.Root
	if base == "" { base = "." }
	if scope == "global" { return filepath.Join(userHome(), ".config", "opencode", "skills", id) }
	return filepath.Join(base, ".opencode", "skills", id)
}

func (s *Store) RemoveFiles(scope string, ids []string, purge bool, pruneUnused bool) error {
	lf, err := s.Read(scope)
	if err != nil {
		if os.IsNotExist(err) { return fmt.Errorf("no installed skills") }
		return err
	}
	if len(lf.Skills) == 0 { return fmt.Errorf("no installed skills") }
	for _, id := range ids {
		if _, ok := lf.Skills[id]; !ok { return fmt.Errorf("unknown skill id %q", id) }
		if err := os.RemoveAll(s.ExposurePathFor(scope, id)); err != nil { return err }
		if purge { if err := os.RemoveAll(s.CanonicalPathFor(scope, id)); err != nil { return err } }
		if pruneUnused && !purge {
			if err := os.RemoveAll(s.CanonicalPathFor(scope, id)); err != nil { return err }
		}
		delete(lf.Skills, id)
	}
	if pruneUnused {
		for id := range lf.Skills {
			if _, err := os.Stat(s.ExposurePathFor(scope, id)); err != nil && os.IsNotExist(err) {
				if err := os.RemoveAll(s.CanonicalPathFor(scope, id)); err != nil { return err }
			}
		}
	}
	return s.Write(scope, lf)
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" { return "." }
	return h
}

func normalizeLockfilePaths(root, scope string, lf *model.Lockfile) {
	for id, sk := range lf.Skills {
		sk.Source = normalizeSkillSource(root, scope, sk.Source)
		sk.Install = normalizeSkillInstall(root, scope, sk.Install)
		sk.Exposures = normalizeExposures(root, scope, sk.Exposures)
		lf.Skills[id] = sk
	}
}

func normalizeSkillSource(root, scope string, src *model.SkillSource) *model.SkillSource {
	if src == nil { return nil }
	out := *src
	if scope != "global" {
		if out.Path != "" { out.Path = stableRel(root, out.Path) }
		if out.Locator != "" { out.Locator = stableRel(root, out.Locator) }
	} else {
		if out.Path != "" { out.Path = filepath.Clean(out.Path) }
	}
	return &out
}

func normalizeSkillInstall(root, scope string, inst *model.SkillInstall) *model.SkillInstall {
	if inst == nil { return nil }
	out := *inst
	if scope != "global" && out.CanonicalPath != "" { out.CanonicalPath = stableRel(root, out.CanonicalPath) }
	return &out
}

func normalizeExposures(root, scope string, exposures []model.Exposure) []model.Exposure {
	out := make([]model.Exposure, 0, len(exposures))
	for _, ex := range exposures {
		if scope != "global" && ex.Path != "" { ex.Path = stableRel(root, ex.Path) }
		out = append(out, ex)
	}
	return out
}

func stableRel(root, p string) string {
	if p == "" { return "" }
	if !filepath.IsAbs(p) { return filepath.Clean(p) }
	rel, err := filepath.Rel(root, p)
	if err != nil { return filepath.Clean(p) }
	return rel
}
