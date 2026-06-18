package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tassis/spick/internal/agents"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
)

type Service struct {
	WorkspaceRoot string
}

func New(workspaceRoot string) *Service { return &Service{WorkspaceRoot: workspaceRoot} }

func (s *Service) Add(opts AddOptions) (*AddResult, error) {
	if len(opts.Selected) == 0 {
		return &AddResult{}, nil
	}
	out := make([]InstalledSkillResult, 0, len(opts.Selected))
	for _, skill := range opts.Selected {
		materialized, err := s.materializeSkill(opts, skill)
		if err != nil {
			return nil, err
		}
		out = append(out, materialized)
	}
	return &AddResult{Installed: out}, nil
}

func (s *Service) Apply(opts ApplyOptions) (*ApplyResult, error) {
	if len(opts.Skills) == 0 {
		return &ApplyResult{}, nil
	}
	out := make([]InstalledSkillResult, 0, len(opts.Skills))
	for _, skill := range opts.Skills {
		materialized, err := s.applySkill(opts, skill)
		if err != nil {
			return nil, err
		}
		out = append(out, materialized)
	}
	return &ApplyResult{Applied: out}, nil
}

type AddResult struct {
	Installed []InstalledSkillResult `json:"installed,omitempty"`
	Message   string                 `json:"message,omitempty"`
}

type ApplyResult struct {
	Applied []InstalledSkillResult `json:"applied,omitempty"`
}

type InstalledSkillResult struct {
	ID        string           `json:"id"`
	Path      string           `json:"path,omitempty"`
	Target    string           `json:"target,omitempty"`
	Mode      string           `json:"mode,omitempty"`
	Source    *model.Source    `json:"source,omitempty"`
	Exposures []model.Exposure `json:"exposures,omitempty"`
}

type SelectedSkill struct {
	Catalog model.CatalogSkill
	Mode    string
	Agent   string
}

type AddOptions struct {
	Scope          string
	SourceRoot     string
	All            bool
	Skills         []string
	Selected       []model.CatalogSkill
	ExposureMethod string
	Agents         []string
	Agent          string
	Version        string
	Ref            string
	Force          bool
	Yes            bool
	AutoApply      *bool
}

type ApplyOptions struct {
	Scope          string
	SourceRoot     string
	Skills         []model.InstalledSkill
	ExposureMethod string
	Agents         []string
	Agent          string
	Force          bool
}

type RemoveOptions struct {
	Scope  string
	All    bool
	Skills []string
	Force  bool
	Yes    bool
}

type ListOptions struct {
	Scope string
	JSON  bool
	All   bool
}

type InspectOptions struct {
	Scope  string
	Skills []string
	JSON   bool
}

func (s *Service) materializeSkill(opts AddOptions, skill model.CatalogSkill) (InstalledSkillResult, error) {
	mode := opts.ExposureMethod
	if mode == "" {
		mode = "symlink"
	}
	targetDir, _, err := s.resolvePaths(opts.Scope, opts.Agent, skill.ID)
	if err != nil {
		return InstalledSkillResult{}, err
	}
	sourceDir := filepath.Join(opts.SourceRoot, catalogSourcePath(skill))
	if !opts.Force {
		if _, err := os.Lstat(targetDir); err == nil {
			return InstalledSkillResult{}, fmt.Errorf("destination exists: %s", targetDir)
		}
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return InstalledSkillResult{}, err
	}
	if err := copyDir(sourceDir, targetDir); err != nil {
		return InstalledSkillResult{}, err
	}
	if mode != "copy" && mode != "symlink" {
		return InstalledSkillResult{}, fmt.Errorf("unsupported exposure method %q", mode)
	}
	if opts.AutoApply != nil && !*opts.AutoApply {
		return InstalledSkillResult{ID: skill.ID, Path: targetDir, Target: targetDir, Mode: mode, Source: skill.Source}, nil
	}
	if len(opts.Agents) == 0 {
		opts.Agents = []string{defaultAgent(opts.Agent)}
	}
	exposures := make([]model.Exposure, 0, len(opts.Agents))
	for _, agent := range opts.Agents {
		exposurePath, err := exposurePathForAgent(s.WorkspaceRoot, opts.Scope, agent, skill.ID)
		if err != nil {
			return InstalledSkillResult{}, err
		}
		if !opts.Force {
			if _, err := os.Lstat(exposurePath); err == nil {
				return InstalledSkillResult{}, fmt.Errorf("exposure exists: %s", exposurePath)
			}
		}
		if err := os.RemoveAll(exposurePath); err != nil {
			return InstalledSkillResult{}, err
		}
		if err := os.MkdirAll(filepath.Dir(exposurePath), 0o755); err != nil {
			return InstalledSkillResult{}, err
		}
		if mode == "copy" {
			if err := copyDir(targetDir, exposurePath); err != nil {
				return InstalledSkillResult{}, err
			}
		} else {
			exposureTarget, err := filepath.Rel(filepath.Dir(exposurePath), targetDir)
			if err != nil {
				return InstalledSkillResult{}, err
			}
			if err := os.Symlink(exposureTarget, exposurePath); err != nil {
				return InstalledSkillResult{}, err
			}
		}
		exposures = append(exposures, model.Exposure{Agent: agent, Path: exposurePath})
	}
	return InstalledSkillResult{ID: skill.ID, Path: targetDir, Target: targetDir, Mode: mode, Source: skill.Source, Exposures: exposures}, nil
}

func (s *Service) applySkill(opts ApplyOptions, skill model.InstalledSkill) (InstalledSkillResult, error) {
	mode := opts.ExposureMethod
	if mode == "" {
		mode = "symlink"
	}
	if mode != "copy" && mode != "symlink" {
		return InstalledSkillResult{}, fmt.Errorf("unsupported exposure method %q", mode)
	}
	if skill.Install == nil || skill.Install.CanonicalPath == "" {
		return InstalledSkillResult{}, fmt.Errorf("skill %q missing canonical path", skill.ID)
	}
	canonical := skill.Install.CanonicalPath
	if !filepath.IsAbs(canonical) {
		canonical = filepath.Join(s.WorkspaceRoot, canonical)
	}
	if info, err := os.Stat(canonical); err != nil || !info.IsDir() {
		return InstalledSkillResult{}, fmt.Errorf("skill %q missing canonical directory", skill.ID)
	}
	if len(opts.Agents) == 0 {
		opts.Agents = []string{defaultAgent(opts.Agent)}
	}
	existing := map[string]model.Exposure{}
	for _, exposure := range skill.Exposures {
		exposure.Path = resolveWorkspacePath(s.WorkspaceRoot, exposure.Path)
		existing[exposure.Agent] = exposure
	}
	desired := map[string]bool{}
	for _, agent := range opts.Agents {
		desired[agent] = true
		if current, ok := existing[agent]; ok {
			if sameExposure(s.WorkspaceRoot, current.Path, canonical, mode) {
				continue
			}
			if !opts.Force {
				return InstalledSkillResult{}, fmt.Errorf("exposure exists: %s", current.Path)
			}
			if err := os.RemoveAll(current.Path); err != nil {
				return InstalledSkillResult{}, err
			}
		}
		exposurePath, err := exposurePathForAgent(s.WorkspaceRoot, opts.Scope, agent, skill.ID)
		if err != nil {
			return InstalledSkillResult{}, err
		}
		if err := materializeExposure(exposurePath, canonical, mode); err != nil {
			return InstalledSkillResult{}, err
		}
		existing[agent] = model.Exposure{Agent: agent, Path: exposurePath}
	}
	for agent, exposure := range existing {
		if desired[agent] {
			continue
		}
		if err := os.RemoveAll(resolveWorkspacePath(s.WorkspaceRoot, exposure.Path)); err != nil {
			return InstalledSkillResult{}, err
		}
		delete(existing, agent)
	}
	exposures := make([]model.Exposure, 0, len(existing))
	for _, agent := range opts.Agents {
		if exposure, ok := existing[agent]; ok {
			exposures = append(exposures, exposure)
		}
	}
	return InstalledSkillResult{ID: skill.ID, Path: canonical, Target: canonical, Mode: mode, Exposures: exposures}, nil
}

func sameExposure(workspaceRoot, path, canonical, mode string) bool {
	if path != "" && !filepath.IsAbs(path) {
		path = filepath.Join(workspaceRoot, path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if mode == "copy" {
		return info.IsDir()
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	absTarget := target
	if !filepath.IsAbs(target) {
		absTarget = filepath.Clean(filepath.Join(filepath.Dir(path), target))
	}
	return absTarget == canonical
}

func materializeExposure(path, canonical, mode string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if mode == "copy" {
		return copyDir(canonical, path)
	}
	rel, err := filepath.Rel(filepath.Dir(path), canonical)
	if err != nil {
		return err
	}
	return os.Symlink(rel, path)
}

func resolveWorkspacePath(workspaceRoot, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workspaceRoot, path)
}

func (s *Service) resolvePaths(scope string, agent string, id string) (string, string, error) {
	base := s.WorkspaceRoot
	if base == "" {
		base = "."
	}
	projectBase := filepath.Join(base, ".spick", "skills")
	globalBase := filepath.Join(userHome(), ".spick", "skills")
	exposedBase, err := exposureBase(scope, defaultAgent(agent), base)
	if err != nil {
		return "", "", err
	}
	if scope == "global" {
		projectBase = globalBase
	}
	return filepath.Join(projectBase, id), exposedBase, nil
}

func exposurePathForAgent(workspaceRoot string, scope string, agent string, id string) (string, error) {
	root, err := agents.ExposureRoot(config.Scope(scope), agent)
	if err != nil {
		return "", err
	}
	if scope == "global" {
		return filepath.Join(userHome(), root, "skills", id), nil
	}
	return filepath.Join(workspaceRoot, root, "skills", id), nil
}

func exposureBase(scope string, agent string, workspaceRoot string) (string, error) {
	root, err := agents.ExposureRoot(config.Scope(scope), agent)
	if err != nil {
		return "", err
	}
	if scope == "global" {
		return filepath.Join(userHome(), root, "skills"), nil
	}
	return filepath.Join(workspaceRoot, root, "skills"), nil
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "."
	}
	return h
}

func (s *Service) DefaultAgentPath(scope string, id string) (string, error) {
	path, _, err := s.resolvePaths(scope, "", id)
	return path, err
}

func catalogSourcePath(c model.CatalogSkill) string {
	if c.Source == nil {
		return ""
	}
	if c.Source.Path != "" {
		return c.Source.Path
	}
	return c.Source.Locator
}

func defaultAgent(agent string) string {
	if agent != "" {
		return agent
	}
	return "opencode"
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel != "." {
			parts := strings.Split(rel, string(filepath.Separator))
			for _, part := range parts {
				if part == ".spick" || part == ".opencode" {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if rel == "spick.lock" {
				return nil
			}
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
