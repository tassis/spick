package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
func (s *Service) Remove(opts RemoveOptions) error { return nil }
func (s *Service) List(opts ListOptions) ([]model.CatalogSkill, error) { return nil, nil }
func (s *Service) Inspect(opts InspectOptions) (*model.CatalogSkill, error) { return nil, nil }

type AddResult struct {
	Installed []InstalledSkillResult `json:"installed,omitempty"`
	Message   string                 `json:"message,omitempty"`
}

type InstalledSkillResult struct {
	ID     string           `json:"id"`
	Path   string           `json:"path,omitempty"`
	Target string           `json:"target,omitempty"`
	Mode   string           `json:"mode,omitempty"`
	Source *model.Source    `json:"source,omitempty"`
	Exposures []model.Exposure `json:"exposures,omitempty"`
}

type SelectedSkill struct {
	Catalog model.CatalogSkill
	Mode    string
	Agent   string
}

type AddOptions struct {
	Scope   string
	SourceRoot string
	All     bool
	Skills  []string
	Selected []model.CatalogSkill
	Mode    string
	Agent   string
	Version string
	Ref     string
	Force   bool
	Yes     bool
}

type RemoveOptions struct {
	Scope string
	All   bool
	Skills []string
	Force bool
	Yes   bool
}

type ListOptions struct {
	Scope string
	JSON  bool
	All   bool
}

type InspectOptions struct {
	Scope string
	Skills []string
	JSON   bool
}

func (s *Service) materializeSkill(opts AddOptions, skill model.CatalogSkill) (InstalledSkillResult, error) {
	mode := opts.Mode
	if mode == "" {
		mode = "symlink"
	}
	targetDir, exposureDir, err := s.resolvePaths(opts.Scope, opts.Agent, mode, skill.ID)
	if err != nil {
		return InstalledSkillResult{}, err
	}
	sourceDir := filepath.Join(opts.SourceRoot, catalogSourcePath(skill))
	if !opts.Force {
		if _, err := os.Lstat(targetDir); err == nil { return InstalledSkillResult{}, fmt.Errorf("destination exists: %s", targetDir) }
		if _, err := os.Lstat(exposureDir); err == nil { return InstalledSkillResult{}, fmt.Errorf("exposure exists: %s", exposureDir) }
	}
	if err := os.RemoveAll(targetDir); err != nil { return InstalledSkillResult{}, err }
	if err := copyDir(sourceDir, targetDir); err != nil { return InstalledSkillResult{}, err }
	if mode != "copy" && mode != "symlink" { return InstalledSkillResult{}, fmt.Errorf("unsupported mode %q", mode) }
	exposurePath := exposureDir
	if err := os.RemoveAll(exposurePath); err != nil { return InstalledSkillResult{}, err }
	if err := os.MkdirAll(filepath.Dir(exposurePath), 0o755); err != nil { return InstalledSkillResult{}, err }
	if mode == "copy" {
		if err := copyDir(targetDir, exposurePath); err != nil { return InstalledSkillResult{}, err }
	} else {
		exposureTarget, err := filepath.Rel(filepath.Dir(exposurePath), targetDir)
		if err != nil { return InstalledSkillResult{}, err }
		if err := os.Symlink(exposureTarget, exposurePath); err != nil { return InstalledSkillResult{}, err }
	}
	return InstalledSkillResult{ID: skill.ID, Path: targetDir, Target: targetDir, Mode: mode, Source: skill.Source, Exposures: []model.Exposure{{Agent: defaultAgent(opts.Agent), Path: exposurePath}}}, nil
}

func (s *Service) resolvePaths(scope string, agent string, mode string, id string) (string, string, error) {
	base := s.WorkspaceRoot
	if base == "" {
		base = "."
	}
	projectBase := filepath.Join(base, ".skills")
	globalBase := filepath.Join(userHome(), ".spick", "skills")
	exposedBase := filepath.Join(base, ".opencode", "skills")
	if scope == "global" {
		projectBase = globalBase
		exposedBase = filepath.Join(userHome(), ".config", "opencode", "skills")
	}
	return filepath.Join(projectBase, id), filepath.Join(exposedBase, id), nil
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "."
	}
	return h
}

func (s *Service) DefaultAgentPath(scope string, id string) (string, error) {
	path, _, err := s.resolvePaths(scope, "", "copy", id)
	return path, err
}

func catalogSourcePath(c model.CatalogSkill) string {
	if c.Source == nil { return "" }
	if c.Source.Path != "" { return c.Source.Path }
	return c.Source.Locator
}

func defaultAgent(agent string) string {
	if agent != "" { return agent }
	return "opencode"
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		rel, err := filepath.Rel(src, path)
		if err != nil { return err }
		if rel != "." {
			parts := strings.Split(rel, string(filepath.Separator))
			for _, part := range parts {
				if part == ".skills" || part == ".opencode" {
					if info.IsDir() { return filepath.SkipDir }
					return nil
				}
			}
			if rel == "spick.lock" { return nil }
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil { return err }
			return os.Symlink(link, target)
		}
		data, err := os.ReadFile(path)
		if err != nil { return err }
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
