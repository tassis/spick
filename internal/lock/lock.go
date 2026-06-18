package lock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tassis/spick/internal/agents"
	"github.com/tassis/spick/internal/config"
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
	if root == "" {
		root = "."
	}
	return filepath.Join(root, "spick.lock")
}

// Read loads the materialized lock snapshot for a scope.
func (s *Store) Read(scope string) (*model.Lockfile, error) {
	data, err := os.ReadFile(s.pathFor(scope))
	if err != nil {
		return nil, err
	}
	lf, err := parseLockfile(data)
	if err != nil {
		return nil, fmt.Errorf("parse lockfile: %w", err)
	}
	if lf.Version == 0 {
		lf.Version = 1
	}
	if lf.Skills == nil {
		lf.Skills = []model.LockSkill{}
	}
	if lf.Plugins == nil {
		lf.Plugins = []model.LockPlugin{}
	}
	if lf.Scope == "" {
		lf.Scope = scope
	}
	return lf, nil
}

type lockfileWire struct {
	Version int             `json:"version"`
	Scope   string          `json:"scope,omitempty"`
	Skills  json.RawMessage `json:"skills,omitempty"`
	Plugins json.RawMessage `json:"plugins,omitempty"`
}

type legacySkillRecord struct {
	ID        string              `json:"id"`
	Source    *model.SkillSource  `json:"source,omitempty"`
	Install   *model.SkillInstall `json:"install,omitempty"`
	Exposures []model.Exposure    `json:"exposures,omitempty"`
}

func parseLockfile(data []byte) (*model.Lockfile, error) {
	var wire lockfileWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, err
	}

	lf := &model.Lockfile{Version: wire.Version, Scope: wire.Scope}

	if len(wire.Skills) > 0 && string(wire.Skills) != "null" {
		skills, err := parseSkills(wire.Skills)
		if err != nil {
			return nil, err
		}
		lf.Skills = skills
	}

	if len(wire.Plugins) > 0 && string(wire.Plugins) != "null" {
		if err := json.Unmarshal(wire.Plugins, &lf.Plugins); err != nil {
			return nil, err
		}
	}

	return lf, nil
}

func parseSkills(raw json.RawMessage) ([]model.LockSkill, error) {
	var skills []model.LockSkill
	if err := json.Unmarshal(raw, &skills); err == nil {
		return skills, nil
	}

	var legacy map[string]legacySkillRecord
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(legacy))
	for id := range legacy {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	converted := make([]model.LockSkill, 0, len(keys))
	for _, id := range keys {
		rec := legacy[id]
		skillID := rec.ID
		if skillID == "" {
			skillID = id
		}
		out := model.LockSkill{ID: skillID}
		if rec.Source != nil {
			source := firstNonEmpty(rec.Source.CloneURL, rec.Source.Locator, rec.Source.Path)
			out.Declared = model.LockDeclared{Source: source, Ref: rec.Source.RequestedVersion}
			out.Resolved = model.LockResolved{Source: source, Ref: rec.Source.RequestedVersion, Revision: rec.Source.RequestedVersion}
		}
		if rec.Install != nil {
			out.Materialized = model.LockMaterialized{Path: rec.Install.CanonicalPath}
			out.Projected = model.LockProjected{Mode: rec.Install.Mode, Exposures: rec.Exposures}
		} else {
			out.Projected = model.LockProjected{Exposures: rec.Exposures}
		}
		converted = append(converted, out)
	}

	return converted, nil
}

// Write persists the resolved/materialized lock snapshot.
func (s *Store) Write(scope string, lf *model.Lockfile) error {
	if lf == nil {
		return fmt.Errorf("lockfile is required")
	}
	if lf.Version == 0 {
		lf.Version = 1
	}
	if lf.Scope == "" {
		lf.Scope = scope
	}
	normalizeLockfilePaths(s.Root, scope, lf)
	path := s.pathFor(scope)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".spick-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (s *Store) UpsertInstalled(scope string, skills []model.InstalledSkill) error {
	lf, err := s.Read(scope)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if lf == nil {
		lf = &model.Lockfile{Version: 1, Scope: scope}
	}
	byID := map[string]int{}
	for i, sk := range lf.Skills {
		byID[sk.ID] = i
	}
	for _, sk := range skills {
		rec := model.LockSkill{ID: sk.ID, Declared: model.LockDeclared{}, Resolved: model.LockResolved{}, Materialized: model.LockMaterialized{Path: stableRel(s.Root, sk.Install.CanonicalPath)}, Projected: model.LockProjected{Mode: sk.Install.Mode, Exposures: normalizeExposures(s.Root, scope, sk.Exposures)}}
		if sk.Source != nil {
			rec.Declared = model.LockDeclared{Source: firstNonEmpty(sk.Source.CloneURL, sk.Source.Locator, sk.Source.Path), Ref: sk.Source.RequestedVersion}
			rec.Resolved = model.LockResolved{Source: firstNonEmpty(sk.Source.CloneURL, sk.Source.Locator, sk.Source.Path), Ref: sk.Source.RequestedVersion, Revision: sk.Source.RequestedVersion}
		}
		if idx, ok := byID[sk.ID]; ok {
			lf.Skills[idx] = rec
		} else {
			lf.Skills = append(lf.Skills, rec)
		}
	}
	return s.Write(scope, lf)
}

func (s *Store) UpsertPlugins(scope string, plugins []model.LockPlugin) error {
	lf, err := s.Read(scope)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if lf == nil {
		lf = &model.Lockfile{Version: 1, Scope: scope}
	}
	byID := map[string]int{}
	for i, pl := range lf.Plugins {
		byID[pl.ID] = i
	}
	for _, pl := range plugins {
		if idx, ok := byID[pl.ID]; ok {
			lf.Plugins[idx] = pl
		} else {
			lf.Plugins = append(lf.Plugins, pl)
		}
	}
	return s.Write(scope, lf)
}

func (s *Store) ListPlugins(scope string) ([]model.LockPlugin, error) {
	lf, err := s.Read(scope)
	if err != nil {
		return nil, err
	}
	out := make([]model.LockPlugin, 0, len(lf.Plugins))
	for _, pl := range lf.Plugins {
		out = append(out, pl)
	}
	return out, nil
}

func (s *Store) RemovePlugins(scope string, ids []string) error {
	lf, err := s.Read(scope)
	if err != nil {
		return err
	}
	keep := make([]model.LockPlugin, 0, len(lf.Plugins))
	for _, pl := range lf.Plugins {
		remove := false
		for _, id := range ids {
			if pl.ID == id || pl.Declared.Source == id {
				remove = true
				break
			}
		}
		if !remove {
			keep = append(keep, pl)
		}
	}
	lf.Plugins = keep
	return s.Write(scope, lf)
}

func (s *Store) List(scope string) ([]model.InstalledSkill, error) {
	lf, err := s.Read(scope)
	if err != nil {
		return nil, err
	}
	out := make([]model.InstalledSkill, 0, len(lf.Skills))
	for _, sk := range lf.Skills {
		out = append(out, model.InstalledSkill{ID: sk.ID, Install: &model.SkillInstall{CanonicalPath: sk.Materialized.Path, Mode: sk.Projected.Mode}, Exposures: sk.Projected.Exposures})
	}
	return out, nil
}

func (s *Store) Remove(scope string, ids []string) (*model.Lockfile, error) {
	lf, err := s.Read(scope)
	if err != nil {
		return nil, err
	}
	keep := make([]model.LockSkill, 0, len(lf.Skills))
	for _, sk := range lf.Skills {
		remove := false
		for _, id := range ids {
			if sk.ID == id {
				remove = true
				break
			}
		}
		if !remove {
			keep = append(keep, sk)
		}
	}
	lf.Skills = keep
	return lf, nil
}

func (s *Store) InUse(scope string, id string) bool {
	lf, err := s.Read(scope)
	if err != nil {
		return false
	}
	for _, sk := range lf.Skills {
		if sk.ID == id {
			return true
		}
	}
	return false
}

func (s *Store) ManagedSkillPathFor(scope, id string) string {
	base := s.Root
	if base == "" {
		base = "."
	}
	if scope == "global" {
		return filepath.Join(userHome(), ".spick", "skills", id)
	}
	return filepath.Join(base, ".spick", "skills", id)
}

func (s *Store) ManagedPluginPathFor(scope, id string) string {
	base := s.Root
	if base == "" {
		base = "."
	}
	if scope == "global" {
		return filepath.Join(userHome(), ".spick", "plugins", id)
	}
	return filepath.Join(base, ".spick", "plugins", id)
}

func (s *Store) ExposurePathFor(scope, id string) string {
	base := s.Root
	if base == "" {
		base = "."
	}
	root, err := agents.ExposureRoot(config.Scope(scope), "opencode")
	if err != nil {
		return ""
	}
	if scope == "global" {
		return filepath.Join(userHome(), root, "skills", id)
	}
	return filepath.Join(base, root, "skills", id)
}

func (s *Store) ExposurePathForAgent(scope, agent, id string) string {
	base := s.Root
	if base == "" {
		base = "."
	}
	root, err := agents.ExposureRoot(config.Scope(scope), agent)
	if err != nil {
		return ""
	}
	if scope == "global" {
		return filepath.Join(userHome(), root, "skills", id)
	}
	return filepath.Join(base, root, "skills", id)
}

func (s *Store) RemoveFiles(scope string, ids []string, pruneUnused bool) error {
	lf, err := s.Read(scope)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no installed skills")
		}
		return err
	}
	if len(lf.Skills) == 0 {
		return fmt.Errorf("no installed skills")
	}
	for _, id := range ids {
		idx := -1
		for i := range lf.Skills {
			if lf.Skills[i].ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("unknown skill id %q", id)
		}
		sk := lf.Skills[idx]
		for _, ex := range sk.Projected.Exposures {
			if err := os.RemoveAll(resolveStoredPath(s.Root, scope, ex.Path)); err != nil {
				return err
			}
		}
		if err := os.RemoveAll(resolveStoredPath(s.Root, scope, s.ManagedSkillPathFor(scope, id))); err != nil {
			return err
		}
		lf.Skills = append(lf.Skills[:idx], lf.Skills[idx+1:]...)
	}
	if pruneUnused {
		keep := lf.Skills[:0]
		for _, sk := range lf.Skills {
			if _, err := os.Stat(s.ExposurePathFor(scope, sk.ID)); err != nil && os.IsNotExist(err) {
				if err := os.RemoveAll(resolveStoredPath(s.Root, scope, s.ManagedSkillPathFor(scope, sk.ID))); err != nil {
					return err
				}
				continue
			}
			keep = append(keep, sk)
		}
		lf.Skills = keep
	}
	return s.Write(scope, lf)
}

func (s *Store) ExposureCount(scope, id string) int {
	lf, err := s.Read(scope)
	if err != nil {
		return 0
	}
	for _, sk := range lf.Skills {
		if sk.ID == id {
			return len(sk.Projected.Exposures)
		}
	}
	return 0
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "."
	}
	return h
}

func normalizeLockfilePaths(root, scope string, lf *model.Lockfile) {
	for i := range lf.Skills {
		lf.Skills[i] = normalizeLockSkill(root, scope, lf.Skills[i])
	}
	for i := range lf.Plugins {
		lf.Plugins[i] = normalizeLockPlugin(root, scope, lf.Plugins[i])
	}
}

func normalizeLockSkill(root, scope string, sk model.LockSkill) model.LockSkill {
	sk.Materialized.Path = stableRel(root, sk.Materialized.Path)
	sk.Projected.Exposures = normalizeExposures(root, scope, sk.Projected.Exposures)
	return sk
}

func normalizeLockPlugin(root, scope string, pl model.LockPlugin) model.LockPlugin {
	pl.Materialized.Path = stableRel(root, pl.Materialized.Path)
	pl.Projected.Path = stableRel(root, pl.Projected.Path)
	return pl
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func normalizeSkillSource(root, scope string, src *model.SkillSource) *model.SkillSource {
	if src == nil {
		return nil
	}
	out := *src
	if scope != "global" {
		if out.Path != "" {
			out.Path = stableRel(root, out.Path)
		}
		if out.Locator != "" {
			out.Locator = stableRel(root, out.Locator)
		}
		if out.CloneURL != "" && !isURLLike(out.CloneURL) {
			out.CloneURL = stableRel(root, out.CloneURL)
		}
	} else {
		if out.Path != "" {
			out.Path = filepath.Clean(out.Path)
		}
	}
	return &out
}

func normalizeSkillInstall(root, scope string, inst *model.SkillInstall) *model.SkillInstall {
	if inst == nil {
		return nil
	}
	out := *inst
	if scope != "global" && out.CanonicalPath != "" {
		out.CanonicalPath = stableRel(root, out.CanonicalPath)
	}
	return &out
}

func normalizeExposures(root, scope string, exposures []model.Exposure) []model.Exposure {
	out := make([]model.Exposure, 0, len(exposures))
	for _, ex := range exposures {
		if scope != "global" && ex.Path != "" {
			ex.Path = stableRel(root, ex.Path)
		}
		out = append(out, ex)
	}
	return out
}

func stableRel(root, p string) string {
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return filepath.Clean(p)
	}
	return rel
}

func isURLLike(s string) bool {
	return strings.Contains(s, "://") || strings.HasPrefix(s, "git@")
}

func resolveStoredPath(root, scope, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if scope == "global" {
		return filepath.Join(userHome(), p)
	}
	if root == "" {
		root = "."
	}
	return filepath.Join(root, p)
}
