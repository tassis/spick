package workspace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
	"gopkg.in/yaml.v3"
)

type Workspace struct {
	Root string
}

type SourceRepo struct {
	Root string
}

type ManifestDiscovery struct {
	Repo *SourceRepo
}

type Catalog struct {
	Root string
}

type Loader struct {
	Root string
}

type ProjectConfig struct {
	Skills         []model.ProjectSkill
	Plugins        []model.ProjectPlugin
	Agents         map[string]model.ProjectAgentEnablement
	ExposureMethod string
	AutoApply      bool
}

type Manifest struct {
	Version int                 `yaml:"version"`
	Project model.ProjectConfig `yaml:"project"`
	Skills  []ManifestSkill     `yaml:"skills"`
}

type ManifestSkill struct {
	ID          string `yaml:"id"`
	Path        string `yaml:"path"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type ManifestPlugin struct {
	Source string `yaml:"source"`
	Ref    string `yaml:"ref"`
}

type projectAssetDecl struct {
	ID     string `yaml:"id"`
	Source string `yaml:"source"`
	Ref    string `yaml:"ref"`
}

type agentEntry struct {
	Skills  []string `yaml:"skills"`
	Plugins []string `yaml:"plugins"`
}

type ParsedSource struct {
	Kind   string
	Source model.Source
}

func New(root string) *Workspace { return &Workspace{Root: root} }

func (w *Workspace) Resolve(scope config.Scope) (string, error) { return w.Root, nil }
func (w *Workspace) SourceRepo(scope config.Scope) *SourceRepo  { return &SourceRepo{Root: w.Root} }
func (w *Workspace) Discover(scope config.Scope, source model.Source) (*Catalog, error) {
	return &Catalog{Root: w.Root}, nil
}
func (w *Workspace) BuildCatalog(scope config.Scope, source model.Source) ([]model.CatalogSkill, error) {
	if source.Path == "" {
		return nil, fmt.Errorf("local source path required")
	}
	loader := &Loader{Root: source.Path}
	return loader.LoadCatalog()
}

func (w *Workspace) OpenSource(source model.Source) (model.Source, error) {
	if source.Locator == "" && source.CloneURL == "" {
		return source, nil
	}
	if source.Path != "" {
		return source, nil
	}
	raw := source.Locator
	if raw == "" {
		raw = source.CloneURL
	}
	parsed, err := w.ParseSource(raw)
	if err != nil {
		return model.Source{}, err
	}
	if source.RequestedVersion != "" && parsed.Source.RequestedVersion == "" {
		parsed.Source.RequestedVersion = source.RequestedVersion
	}
	if parsed.Kind == "local" {
		return parsed.Source, nil
	}
	checkout, err := w.fetchHosted(parsed, source.RequestedVersion)
	if err != nil {
		return model.Source{}, err
	}
	parsed.Source.Path = checkout
	return parsed.Source, nil
}

func (w *Workspace) fetchHosted(parsed *ParsedSource, ref string) (string, error) {
	root, err := os.MkdirTemp("", "spick-checkout-*")
	if err != nil {
		return "", err
	}
	url := hostedURL(parsed)
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, root)
	gitBin, err := gitBinary()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(gitBin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", hostedCloneError(url, ref, out, err)
	}
	return root, nil
}

func gitBinary() (string, error) {
	if bin := os.Getenv("SPICK_GIT_BIN"); bin != "" {
		return bin, nil
	}
	bin, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git is required for hosted sources: %w", err)
	}
	return bin, nil
}

func hostedCloneError(url, ref string, out []byte, err error) error {
	msg := strings.TrimSpace(string(out))
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "remote branch") || strings.Contains(lower, "not found in upstream origin") {
		return fmt.Errorf("hosted ref %q not found for %s", ref, url)
	}
	if msg == "" {
		msg = err.Error()
	}
	if ref != "" {
		return fmt.Errorf("failed to clone hosted source %s at ref %q: %s", url, ref, msg)
	}
	return fmt.Errorf("failed to clone hosted source %s: %s", url, msg)
}

func hostedURL(parsed *ParsedSource) string {
	if parsed.Source.CloneURL != "" {
		return parsed.Source.CloneURL
	}
	if base := os.Getenv("SPICK_GIT_BASE_URL"); base != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimPrefix(parsed.Source.Locator, parsed.Kind+":"))
	}
	path := strings.TrimPrefix(parsed.Source.Locator, parsed.Kind+":")
	if parsed.Kind == "github" {
		return fmt.Sprintf("https://github.com/%s.git", path)
	}
	return fmt.Sprintf("https://gitlab.com/%s.git", path)
}

func (l *Loader) LoadCatalog() ([]model.CatalogSkill, error) {
	manifest, err := l.LoadSkillCatalogManifest()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return l.DiscoverCatalog()
		}
		return nil, err
	}
	if len(manifest.Skills) == 0 {
		return l.DiscoverCatalog()
	}
	return l.NormalizeCatalog(manifest)
}

func (l *Loader) LoadSkillCatalogManifest() (*Manifest, error) {
	manifestPath := filepath.Join(l.Root, "spick.skill.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	return parseManifest(data)
}

func (w *Workspace) LoadProjectConfig() (*ProjectConfig, error) {
	loader := &Loader{Root: w.Root}
	manifest, err := loader.LoadProjectManifest()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ProjectConfig{AutoApply: true, Agents: map[string]model.ProjectAgentEnablement{}}, nil
		}
		return nil, err
	}
	return &ProjectConfig{Skills: append([]model.ProjectSkill(nil), manifest.Project.Skills...), Plugins: append([]model.ProjectPlugin(nil), manifest.Project.Plugins...), Agents: cloneAgentEnablement(manifest.Project.Agents), ExposureMethod: manifest.Project.ExposureMethod, AutoApply: manifest.Project.AutoApply}, nil
}

func (w *Workspace) WriteProjectPlugins(plugins []model.ProjectPlugin) error {
	path := filepath.Join(w.Root, "spick.yaml")
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         map[string]agentEntry `yaml:"agents"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	raw.Project.Plugins = make([]projectAssetDecl, 0, len(plugins))
	for _, plugin := range plugins {
		raw.Project.Plugins = append(raw.Project.Plugins, projectAssetDecl{ID: plugin.ID, Source: plugin.Source, Ref: plugin.Ref})
	}
	if raw.Version == nil {
		v := 1
		raw.Version = &v
	}
	out, err := yaml.Marshal(&raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func (w *Workspace) WriteProjectSkills(skills []model.ProjectSkill) error {
	path := filepath.Join(w.Root, "spick.yaml")
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         map[string]agentEntry `yaml:"agents"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	raw.Project.Skills = make([]projectAssetDecl, 0, len(skills))
	for _, skill := range skills {
		raw.Project.Skills = append(raw.Project.Skills, projectAssetDecl{ID: skill.ID, Source: skill.Source, Ref: skill.Ref})
	}
	if raw.Version == nil {
		v := 1
		raw.Version = &v
	}
	out, err := yaml.Marshal(&raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func (w *Workspace) WriteProjectAgentEnablement(agent string, skills []string, plugins []string) error {
	path := filepath.Join(w.Root, "spick.yaml")
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         map[string]agentEntry `yaml:"agents"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if raw.Project.Agents == nil {
		raw.Project.Agents = map[string]agentEntry{}
	}
	raw.Project.Agents[agent] = agentEntry{Skills: append([]string(nil), skills...), Plugins: append([]string(nil), plugins...)}
	if raw.Version == nil {
		v := 1
		raw.Version = &v
	}
	out, err := yaml.Marshal(&raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func (w *Workspace) RemoveProjectSkills(ids []string) error {
	path := filepath.Join(w.Root, "spick.yaml")
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         map[string]agentEntry `yaml:"agents"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	remove := map[string]struct{}{}
	for _, id := range ids {
		remove[id] = struct{}{}
	}
	if len(raw.Project.Skills) > 0 {
		keep := raw.Project.Skills[:0]
		for _, sk := range raw.Project.Skills {
			if _, ok := remove[sk.ID]; !ok {
				keep = append(keep, sk)
			}
		}
		raw.Project.Skills = keep
	}
	for agent, entry := range raw.Project.Agents {
		entry.Skills = filterStrings(entry.Skills, remove)
		raw.Project.Agents[agent] = entry
	}
	if raw.Version == nil {
		v := 1
		raw.Version = &v
	}
	out, err := yaml.Marshal(&raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func (w *Workspace) RemoveProjectPlugins(ids []string) error {
	path := filepath.Join(w.Root, "spick.yaml")
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         map[string]agentEntry `yaml:"agents"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
		Catalog struct {
			Skills []ManifestSkill `yaml:"skills"`
		} `yaml:"catalog"`
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	remove := map[string]struct{}{}
	for _, id := range ids {
		remove[id] = struct{}{}
	}
	if len(raw.Project.Plugins) > 0 {
		keep := raw.Project.Plugins[:0]
		for _, pl := range raw.Project.Plugins {
			if _, ok := remove[pl.ID]; !ok {
				keep = append(keep, pl)
			}
		}
		raw.Project.Plugins = keep
	}
	for agent, entry := range raw.Project.Agents {
		entry.Plugins = filterStrings(entry.Plugins, remove)
		raw.Project.Agents[agent] = entry
	}
	if raw.Version == nil {
		v := 1
		raw.Version = &v
	}
	out, err := yaml.Marshal(&raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func filterStrings(values []string, remove map[string]struct{}) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:0]
	for _, v := range values {
		if _, ok := remove[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}

func (l *Loader) LoadProjectManifest() (*Manifest, error) {
	manifestPath := filepath.Join(l.Root, "spick.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	return parseManifest(data)
}

func (l *Loader) DiscoverCatalog() ([]model.CatalogSkill, error) {
	if skillsRoot := filepath.Join(l.Root, "skills"); dirExists(skillsRoot) {
		return l.discoverUnder(skillsRoot)
	}
	entries, err := os.ReadDir(l.Root)
	if err != nil {
		return nil, err
	}
	out := l.discoverEntries(entries, l.Root, "")
	if len(out) == 0 {
		return nil, fmt.Errorf("no manifest or skills found")
	}
	return out, nil
}

func (l *Loader) discoverUnder(root string) ([]model.CatalogSkill, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	prefix, err := filepath.Rel(l.Root, root)
	if err != nil {
		return nil, err
	}
	if prefix == "." {
		prefix = ""
	}
	return l.discoverEntries(entries, root, prefix), nil
}

func (l *Loader) discoverEntries(entries []os.DirEntry, root, prefix string) []model.CatalogSkill {
	var out []model.CatalogSkill
	for _, entry := range entries {
		if !entry.IsDir() || excludedDir(entry.Name()) {
			continue
		}
		relDir := filepath.Join(prefix, entry.Name())
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "SKILL.md")); err == nil {
			out = append(out, l.skillFromDir(entry.Name(), relDir))
		}
	}
	return out
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (l *Loader) NormalizeCatalog(m *Manifest) ([]model.CatalogSkill, error) {
	if m == nil {
		return nil, fmt.Errorf("manifest is required")
	}
	if len(m.Skills) == 0 {
		return nil, fmt.Errorf("skills is required")
	}
	seen := map[string]bool{}
	out := make([]model.CatalogSkill, 0, len(m.Skills))
	for _, skill := range m.Skills {
		if !skillIDRe.MatchString(skill.ID) {
			return nil, fmt.Errorf("invalid skill id %q", skill.ID)
		}
		if seen[skill.ID] {
			return nil, fmt.Errorf("duplicate skill id %q", skill.ID)
		}
		seen[skill.ID] = true
		resolved, err := l.resolveSkill(skill)
		if err != nil {
			return nil, err
		}
		out = append(out, resolved)
	}
	return out, nil
}

func (l *Loader) resolveSkill(skill ManifestSkill) (model.CatalogSkill, error) {
	if skill.Path == "" {
		return model.CatalogSkill{}, fmt.Errorf("skill %q path is required", skill.ID)
	}
	cleaned := filepath.Clean(skill.Path)
	if filepath.IsAbs(cleaned) {
		return model.CatalogSkill{}, fmt.Errorf("skill %q path must be relative", skill.ID)
	}
	joined := filepath.Join(l.Root, cleaned)
	if rel, err := filepath.Rel(l.Root, joined); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return model.CatalogSkill{}, fmt.Errorf("skill %q path escapes repo root", skill.ID)
	}
	if _, err := os.Stat(filepath.Join(joined, "SKILL.md")); err != nil {
		return model.CatalogSkill{}, fmt.Errorf("skill %q missing SKILL.md", skill.ID)
	}
	cs := model.CatalogSkill{ID: skill.ID, Name: skill.Name, Description: skill.Description, Source: &model.Source{Path: cleaned}}
	return cs, nil
}

func (l *Loader) skillFromDir(id, relDir string) model.CatalogSkill {
	return model.CatalogSkill{ID: id, Name: id, Source: &model.Source{Path: relDir}}
}

func excludedDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".spick", ".opencode":
		return true
	default:
		return false
	}
}

func cloneAgentEnablement(in map[string]model.ProjectAgentEnablement) map[string]model.ProjectAgentEnablement {
	if len(in) == 0 {
		return map[string]model.ProjectAgentEnablement{}
	}
	out := make(map[string]model.ProjectAgentEnablement, len(in))
	for k, v := range in {
		out[k] = model.ProjectAgentEnablement{Skills: append([]string(nil), v.Skills...), Plugins: append([]string(nil), v.Plugins...)}
	}
	return out
}

func normalizeProjectSkills(in []projectAssetDecl) ([]model.ProjectSkill, error) {
	seen := map[string]struct{}{}
	out := make([]model.ProjectSkill, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Source) == "" {
			return nil, fmt.Errorf("invalid project.skills entry")
		}
		if _, ok := seen[item.ID]; ok {
			return nil, fmt.Errorf("duplicate project.skills id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		out = append(out, model.ProjectSkill{ID: item.ID, Source: item.Source, Ref: item.Ref})
	}
	return out, nil
}

func normalizeProjectPlugins(in []projectAssetDecl) ([]model.ProjectPlugin, error) {
	seen := map[string]struct{}{}
	out := make([]model.ProjectPlugin, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Source) == "" {
			return nil, fmt.Errorf("invalid project.plugins entry")
		}
		if _, ok := seen[item.ID]; ok {
			return nil, fmt.Errorf("duplicate project.plugins id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		out = append(out, model.ProjectPlugin{ID: item.ID, Source: item.Source, Ref: item.Ref})
	}
	return out, nil
}

func normalizeAgentEnablement(in map[string]agentEntry, skills []model.ProjectSkill, plugins []model.ProjectPlugin) (map[string]model.ProjectAgentEnablement, error) {
	declaredSkills := map[string]struct{}{}
	for _, s := range skills {
		declaredSkills[s.ID] = struct{}{}
	}
	declaredPlugins := map[string]struct{}{}
	for _, p := range plugins {
		declaredPlugins[p.ID] = struct{}{}
	}
	out := map[string]model.ProjectAgentEnablement{}
	for agent, entry := range in {
		if strings.TrimSpace(agent) == "" {
			return nil, fmt.Errorf("invalid project.agents entry")
		}
		for _, id := range entry.Skills {
			if _, ok := declaredSkills[id]; !ok {
				return nil, fmt.Errorf("project.agents.%s references undeclared skill %q", agent, id)
			}
		}
		for _, id := range entry.Plugins {
			if _, ok := declaredPlugins[id]; !ok {
				return nil, fmt.Errorf("project.agents.%s references undeclared plugin %q", agent, id)
			}
		}
		out[agent] = model.ProjectAgentEnablement{Skills: append([]string(nil), entry.Skills...), Plugins: append([]string(nil), entry.Plugins...)}
	}
	return out, nil
}

func (w *Workspace) ParseSource(raw string) (*ParsedSource, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("source is required")
	}
	if strings.HasPrefix(raw, "github:") {
		return parseHostedSource("github", strings.TrimPrefix(raw, "github:"), 2, 2)
	}
	if strings.HasPrefix(raw, "gitlab:") {
		return parseHostedSource("gitlab", strings.TrimPrefix(raw, "gitlab:"), 2, 0)
	}
	if isRawHostedURL(raw) {
		return &ParsedSource{Kind: "hosted", Source: model.Source{CloneURL: raw}}, nil
	}
	if strings.Contains(raw, ":") && !strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "./") && !strings.HasPrefix(raw, "../") {
		return nil, fmt.Errorf("unsupported source: %s", raw)
	}
	return &ParsedSource{Kind: "local", Source: model.Source{Path: filepath.Clean(raw)}}, nil
}

func (w *Workspace) ResolveSource(raw string) (model.Source, error) {
	parsed, err := w.ParseSource(raw)
	if err != nil {
		return model.Source{}, err
	}
	return parsed.Source, nil
}

func parseHostedSource(kind, value string, minSegments int, maxSegments int) (*ParsedSource, error) {
	locator, ref, ok := strings.Cut(value, "@")
	if ok {
		if strings.Contains(ref, "@") || strings.TrimSpace(ref) == "" {
			return nil, fmt.Errorf("invalid %s source", kind)
		}
		value = locator
	}
	parts := splitHostedPath(value)
	if len(parts) < minSegments {
		return nil, fmt.Errorf("invalid %s source", kind)
	}
	if maxSegments > 0 && len(parts) > maxSegments {
		return nil, fmt.Errorf("invalid %s source", kind)
	}
	source := model.Source{Locator: fmt.Sprintf("%s:%s", kind, strings.Join(parts, "/"))}
	if ok {
		source.RequestedVersion = ref
	}
	return &ParsedSource{Kind: kind, Source: source}, nil
}

func isRawHostedURL(raw string) bool {
	return strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "ssh://") || strings.HasPrefix(raw, "git@")
}

func splitHostedPath(value string) []string {
	raw := strings.Trim(value, "/")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

var skillIDRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

func parseManifest(data []byte) (*Manifest, error) {
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         map[string]agentEntry `yaml:"agents"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
		Skills []ManifestSkill `yaml:"skills"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if raw.Project.ExposureMethod != "" {
		switch raw.Project.ExposureMethod {
		case "symlink", "copy":
		default:
			return nil, fmt.Errorf("invalid project.exposureMethod %q", raw.Project.ExposureMethod)
		}
	}
	skills, err := normalizeProjectSkills(raw.Project.Skills)
	if err != nil {
		return nil, err
	}
	plugins, err := normalizeProjectPlugins(raw.Project.Plugins)
	if err != nil {
		return nil, err
	}
	agents, err := normalizeAgentEnablement(raw.Project.Agents, skills, plugins)
	if err != nil {
		return nil, err
	}
	m := &Manifest{Version: 1, Skills: raw.Skills}
	autoApply := true
	if raw.Project.AutoApply != nil {
		autoApply = *raw.Project.AutoApply
	}
	m.Project = model.ProjectConfig{Skills: skills, Plugins: plugins, Agents: agents, ExposureMethod: raw.Project.ExposureMethod, AutoApply: autoApply}
	if raw.Version != nil {
		m.Version = *raw.Version
	}
	return m, nil
}
