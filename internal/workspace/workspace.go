package workspace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

type ResourceManifest = model.ResourceManifest

type projectAssetDecl struct {
	ID     string `yaml:"id"`
	Source string `yaml:"source"`
	Path   string `yaml:"path"`
	Ref    string `yaml:"ref"`
}

type projectAgentDecls []projectAssetDecl

func (d *projectAgentDecls) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var items []projectAssetDecl
		if err := value.Decode(&items); err != nil {
			return err
		}
		*d = items
		return nil
	case yaml.MappingNode:
		var m map[string]projectAssetDecl
		if err := value.Decode(&m); err != nil {
			return err
		}
		items := make([]projectAssetDecl, 0, len(m))
		for id, item := range m {
			item.ID = id
			items = append(items, item)
		}
		*d = items
		return nil
	default:
		return nil
	}
}

type agentEntry struct {
	Skills  []string `yaml:"skills"`
	Plugins []string `yaml:"plugins"`
	Agents  []string `yaml:"agents"`
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
	manifest, err := l.LoadResourceCatalogManifest()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return l.DiscoverCatalog()
		}
		return nil, err
	}
	if len(manifest.Resources.Skills) == 0 {
		return l.DiscoverCatalog()
	}
	return l.NormalizeResourceCatalog(manifest)
}

func (l *Loader) LoadResourceManifest() (*model.ResourceManifest, error) {
	return l.LoadResourceCatalogManifest()
}

func (l *Loader) LoadResourceCatalogManifest() (*model.ResourceManifest, error) {
	manifestPath := filepath.Join(l.Root, "spick.res.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	return parseResourceManifest(data)
}

func (w *Workspace) LoadProjectConfig() (*model.ProjectConfig, error) {
	return w.LoadProjectConfigForScope(config.ScopeProject)
}

func (w *Workspace) LoadProjectConfigForScope(scope config.Scope) (*model.ProjectConfig, error) {
	loader := &Loader{Root: w.rootForScope(scope)}
	manifest, err := loader.LoadProjectManifest()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &model.ProjectConfig{AutoApply: true, Agents: []model.ProjectAgent{}, Runtimes: map[string]model.ProjectRuntimeEnablement{}}, nil
		}
		return nil, err
	}
	declaredAgents := make([]model.ProjectAgent, 0, len(manifest.Agents))
	for _, agent := range manifest.Agents {
		declaredAgents = append(declaredAgents, model.ProjectAgent{ID: agent.ID, Source: agent.Source, Path: agent.Path, Ref: agent.Ref})
	}
	return &model.ProjectConfig{Skills: append([]model.ProjectSkill(nil), manifest.Skills...), Plugins: append([]model.ProjectPlugin(nil), manifest.Plugins...), Agents: declaredAgents, Runtimes: model.CloneRuntimeEnablement(manifest.Runtimes), ExposureMethod: manifest.ExposureMethod, AutoApply: manifest.AutoApply}, nil
}

func (w *Workspace) LoadResourceManifest() (*model.ResourceManifest, error) {
	return (&Loader{Root: w.Root}).LoadResourceManifest()
}

func (w *Workspace) DiscoverAgentResources() ([]model.AgentResource, error) {
	return (&Loader{Root: w.Root}).DiscoverAgentResources()
}

func (w *Workspace) WriteProjectPlugins(plugins []model.ProjectPlugin) error {
	path := filepath.Join(w.Root, "spick.yaml")
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         projectAgentDecls     `yaml:"agents"`
			Runtimes       map[string]agentEntry `yaml:"runtimes"`
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
			Agents         projectAgentDecls     `yaml:"agents"`
			Runtimes       map[string]agentEntry `yaml:"runtimes"`
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
			Agents         projectAgentDecls     `yaml:"agents"`
			Runtimes       map[string]agentEntry `yaml:"runtimes"`
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
	if raw.Project.Runtimes == nil {
		raw.Project.Runtimes = map[string]agentEntry{}
	}
	raw.Project.Runtimes[agent] = agentEntry{Skills: append([]string(nil), skills...), Plugins: append([]string(nil), plugins...)}
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

func (w *Workspace) WriteProjectConfig(project model.ProjectConfig) error {
	return w.WriteProjectConfigForScope(config.ScopeProject, project)
}

func (w *Workspace) WriteProjectConfigForScope(scope config.Scope, project model.ProjectConfig) error {
	path := filepath.Join(w.rootForScope(scope), "spick.yaml")
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         projectAgentDecls     `yaml:"agents"`
			Runtimes       map[string]agentEntry `yaml:"runtimes"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
	}
	for _, skill := range project.Skills {
		raw.Project.Skills = append(raw.Project.Skills, projectAssetDecl{ID: skill.ID, Source: skill.Source, Ref: skill.Ref})
	}
	for _, plugin := range project.Plugins {
		raw.Project.Plugins = append(raw.Project.Plugins, projectAssetDecl{ID: plugin.ID, Source: plugin.Source, Ref: plugin.Ref})
	}
	if len(project.Agents) > 0 {
		for _, agent := range project.Agents {
			raw.Project.Agents = append(raw.Project.Agents, projectAssetDecl{ID: agent.ID, Source: agent.Source, Path: agent.Path, Ref: agent.Ref})
		}
	}
	if len(project.Runtimes) > 0 {
		raw.Project.Runtimes = map[string]agentEntry{}
		for name, runtime := range project.Runtimes {
			raw.Project.Runtimes[name] = agentEntry{Skills: append([]string(nil), runtime.Skills...), Plugins: append([]string(nil), runtime.Plugins...), Agents: append([]string(nil), runtime.Agents...)}
		}
	}
	raw.Project.ExposureMethod = string(project.ExposureMethod)
	raw.Project.AutoApply = &project.AutoApply
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

func (w *Workspace) WriteResourceManifest(manifest model.ResourceManifest) error {
	path := filepath.Join(w.Root, "spick.res.yaml")
	normalized := model.NormalizeResourceManifestForPersistence(manifest)
	var raw struct {
		Version   *int                      `yaml:"version"`
		Kind      model.ResourceKind        `yaml:"kind"`
		Plugin    *model.ResourcePlugin     `yaml:"plugin"`
		Resources model.ResourceCollections `yaml:"resources"`
	}
	raw.Version = &normalized.Version
	raw.Kind = normalized.Kind
	raw.Plugin = normalized.Plugin
	raw.Resources = model.ResourceCollections{
		Skills: append([]model.ResourceSkill(nil), normalized.Resources.Skills...),
		Agents: append([]model.AgentResource(nil), normalized.Resources.Agents...),
	}
	out, err := yaml.Marshal(&raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func (w *Workspace) rootForScope(scope config.Scope) string {
	if scope == config.ScopeGlobal {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return filepath.Join(".", ".spick")
		}
		return filepath.Join(home, ".spick")
	}
	return w.Root
}

func (w *Workspace) RemoveProjectSkills(ids []string) error {
	path := filepath.Join(w.Root, "spick.yaml")
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         projectAgentDecls     `yaml:"agents"`
			Runtimes       map[string]agentEntry `yaml:"runtimes"`
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
	remove := model.NewStringSet()
	for _, id := range ids {
		remove.Add(id)
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
	for agent, entry := range raw.Project.Runtimes {
		entry.Skills = filterStrings(entry.Skills, remove)
		raw.Project.Runtimes[agent] = entry
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
			Agents         projectAgentDecls     `yaml:"agents"`
			Runtimes       map[string]agentEntry `yaml:"runtimes"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
		Catalog struct {
			Skills []model.CatalogSkill `yaml:"skills"`
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
	remove := model.NewStringSet()
	for _, id := range ids {
		remove.Add(id)
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
	for agent, entry := range raw.Project.Runtimes {
		entry.Plugins = filterStrings(entry.Plugins, remove)
		raw.Project.Runtimes[agent] = entry
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

func filterStrings(values []string, remove model.StringSet) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:0]
	for _, v := range values {
		if !remove.Has(v) {
			out = append(out, v)
		}
	}
	return out
}

func (l *Loader) LoadProjectManifest() (*model.ProjectConfig, error) {
	manifestPath := filepath.Join(l.Root, "spick.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	return parseProjectManifest(data)
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
		if prefix == "" && filepath.Base(root) == "agents" {
			continue
		}
		relDir := filepath.Join(prefix, entry.Name())
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "SKILL.md")); err == nil {
			out = append(out, l.skillFromDir(entry.Name(), relDir))
		}
	}
	return out
}

func (l *Loader) DiscoverAgentResources() ([]model.AgentResource, error) {
	agentsRoot := filepath.Join(l.Root, "agents")
	entries, err := os.ReadDir(agentsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []model.AgentResource
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		out = append(out, model.AgentResource{ID: id, Path: filepath.Join("agents", entry.Name()), Format: "markdown"})
	}
	return out, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (l *Loader) NormalizeResourceCatalog(m *model.ResourceManifest) ([]model.CatalogSkill, error) {
	skills, err := model.NormalizeResourceManifestSkills(m)
	if err != nil {
		return nil, err
	}
	out := make([]model.CatalogSkill, 0, len(skills))
	for _, skill := range skills {
		resolved, err := l.resolveResourceSkill(skill)
		if err != nil {
			return nil, err
		}
		out = append(out, resolved)
	}
	return out, nil
}

func (l *Loader) resolveResourceSkill(skill model.ResourceSkill) (model.CatalogSkill, error) {
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
	cs := model.CatalogSkill{ID: skill.ID, Source: &model.Source{Path: cleaned}}
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

func parseResourceManifest(data []byte) (*model.ResourceManifest, error) {
	var raw model.ResourceManifestRaw
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return model.ParseResourceManifest(raw)
}

func parseProjectManifest(data []byte) (*model.ProjectConfig, error) {
	var raw struct {
		Version *int `yaml:"version"`
		Project struct {
			Skills         []projectAssetDecl    `yaml:"skills"`
			Plugins        []projectAssetDecl    `yaml:"plugins"`
			Agents         projectAgentDecls     `yaml:"agents"`
			Runtimes       map[string]agentEntry `yaml:"runtimes"`
			ExposureMethod string                `yaml:"exposureMethod"`
			AutoApply      *bool                 `yaml:"autoApply"`
		} `yaml:"project"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	exposureMethod := model.ExposureMethod("")
	if raw.Project.ExposureMethod != "" {
		parsed, ok := model.ParseExposureMethod(raw.Project.ExposureMethod)
		if !ok {
			return nil, fmt.Errorf("invalid project.exposureMethod %q", raw.Project.ExposureMethod)
		}
		exposureMethod = parsed
	}
	declSkills := make([]model.ProjectSkill, 0, len(raw.Project.Skills))
	for _, item := range raw.Project.Skills {
		declSkills = append(declSkills, model.ProjectSkill{ID: item.ID, Source: item.Source, Ref: item.Ref})
	}
	skills, err := model.NormalizeProjectSkills(declSkills)
	if err != nil {
		return nil, err
	}
	declPlugins := make([]model.ProjectPlugin, 0, len(raw.Project.Plugins))
	for _, item := range raw.Project.Plugins {
		declPlugins = append(declPlugins, model.ProjectPlugin{ID: item.ID, Source: item.Source, Ref: item.Ref})
	}
	plugins, err := model.NormalizeProjectPlugins(declPlugins)
	if err != nil {
		return nil, err
	}
	declaredAgents := make([]model.ProjectAgent, 0, len(raw.Project.Agents))
	for _, agent := range raw.Project.Agents {
		declaredAgents = append(declaredAgents, model.ProjectAgent{ID: agent.ID, Source: agent.Source, Path: agent.Path, Ref: agent.Ref})
	}
	runtimeRaw := make(map[string]model.ProjectRuntimeEnablementRaw, len(raw.Project.Runtimes))
	for agent, item := range raw.Project.Runtimes {
		runtimeRaw[agent] = model.ProjectRuntimeEnablementRaw{
			Skills:  append([]string(nil), item.Skills...),
			Plugins: append([]string(nil), item.Plugins...),
			Agents:  append([]string(nil), item.Agents...),
		}
	}
	runtimes, err := model.NormalizeProjectRuntimeEnablement(runtimeRaw, skills, plugins, declaredAgents)
	if err != nil {
		return nil, err
	}
	autoApply := true
	if raw.Project.AutoApply != nil {
		autoApply = *raw.Project.AutoApply
	}
	project := model.ProjectConfig{Skills: skills, Plugins: plugins, Agents: declaredAgents, Runtimes: runtimes, ExposureMethod: exposureMethod, AutoApply: autoApply}
	return &project, nil
}
