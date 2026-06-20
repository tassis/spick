package model

import (
	"fmt"
	"regexp"
)

type Source struct {
	Locator          string `json:"locator,omitempty"`
	CloneURL         string `json:"cloneURL,omitempty"`
	Path             string `json:"path,omitempty"`
	RequestedVersion string `json:"requestedVersion,omitempty"`
	Ref              string `json:"ref,omitempty"`
}

type ResourceKind string

const (
	ResourceKindPlugin    ResourceKind = "plugin"
	ResourceKindResources ResourceKind = "resources"
	ResourceKindAgent     ResourceKind = "agent"
)

func (k ResourceKind) IsValid() bool {
	return k == ResourceKindPlugin || k == ResourceKindResources || k == ResourceKindAgent
}

func ParseResourceKind(raw string) (ResourceKind, bool) {
	k := ResourceKind(raw)
	return k, k.IsValid()
}

type ExposureMethod string

const (
	ExposureMethodCopy    ExposureMethod = "copy"
	ExposureMethodSymlink ExposureMethod = "symlink"
)

func (m ExposureMethod) IsValid() bool {
	return m == ExposureMethodCopy || m == ExposureMethodSymlink
}

const DefaultExposureMethod ExposureMethod = ExposureMethodSymlink

func ParseExposureMethod(raw string) (ExposureMethod, bool) {
	m := ExposureMethod(raw)
	return m, m.IsValid()
}

type ResourceManifest struct {
	Version   int                 `json:"version" yaml:"version"`
	Kind      ResourceKind        `json:"kind" yaml:"kind"`
	Plugin    *ResourcePlugin     `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	Resources ResourceCollections `json:"resources" yaml:"resources"`
}

type ResourceManifestRaw struct {
	Version   *int                `yaml:"version"`
	Kind      ResourceKind        `yaml:"kind"`
	Plugin    *ResourcePlugin     `yaml:"plugin"`
	Resources ResourceCollections `yaml:"resources"`
}

var resourceSkillIDRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

func NormalizeResourceManifestSkills(manifest *ResourceManifest) ([]ResourceSkill, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest is required")
	}
	if len(manifest.Resources.Skills) == 0 {
		return nil, fmt.Errorf("skills is required")
	}
	out := make([]ResourceSkill, 0, len(manifest.Resources.Skills))
	seen := map[string]bool{}
	for _, skill := range manifest.Resources.Skills {
		if !resourceSkillIDRe.MatchString(skill.ID) {
			return nil, fmt.Errorf("invalid skill id %q", skill.ID)
		}
		if seen[skill.ID] {
			return nil, fmt.Errorf("duplicate skill id %q", skill.ID)
		}
		seen[skill.ID] = true
		out = append(out, skill)
	}
	return out, nil
}

func NormalizeResourceManifestForPersistence(raw ResourceManifest) ResourceManifest {
	normalized := ResourceManifest{
		Version:   raw.Version,
		Kind:      raw.Kind,
		Plugin:    raw.Plugin,
		Resources: raw.Resources,
	}
	if normalized.Version == 0 {
		normalized.Version = 1
	}
	if normalized.Kind == "" {
		switch {
		case normalized.Plugin != nil:
			normalized.Kind = ResourceKindPlugin
		case len(normalized.Resources.Skills) > 0 || len(normalized.Resources.Agents) > 0:
			normalized.Kind = ResourceKindResources
		}
	}
	return normalized
}

func ParseResourceManifest(raw ResourceManifestRaw) (*ResourceManifest, error) {
	res := &ResourceManifest{
		Version:   1,
		Kind:      raw.Kind,
		Plugin:    raw.Plugin,
		Resources: raw.Resources,
	}
	if res.Kind == "" {
		switch {
		case res.Plugin != nil:
			res.Kind = ResourceKindPlugin
		case len(res.Resources.Skills) > 0 || len(res.Resources.Agents) > 0:
			res.Kind = ResourceKindResources
		}
	}
	if res.Kind != "" && !res.Kind.IsValid() {
		return nil, fmt.Errorf("invalid kind %q", res.Kind)
	}
	if raw.Version != nil {
		res.Version = *raw.Version
	}
	return res, nil
}

type ResourcePlugin struct {
	ID      string `json:"id,omitempty" yaml:"id,omitempty"`
	Runtime string `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Entry   string `json:"entry,omitempty" yaml:"entry,omitempty"`
}

type ResourceCollections struct {
	Skills []ResourceSkill `json:"skills,omitempty" yaml:"skills,omitempty"`
	Agents []AgentResource `json:"agents,omitempty" yaml:"agents,omitempty"`
}

type ResourceSkill struct {
	ID   string `json:"id" yaml:"id"`
	Path string `json:"path" yaml:"path"`
}

type AgentResource struct {
	ID     string `json:"id" yaml:"id"`
	Path   string `json:"path" yaml:"path"`
	Format string `json:"format,omitempty" yaml:"format,omitempty"`
	Body   string `json:"body,omitempty" yaml:"body,omitempty"`
}

type CatalogSkill struct {
	ID          string  `json:"id"`
	Name        string  `json:"name,omitempty"`
	Description string  `json:"description,omitempty"`
	Source      *Source `json:"source,omitempty"`
}

type SkillCatalogEntry struct {
	ID          string  `json:"id"`
	Name        string  `json:"name,omitempty"`
	Description string  `json:"description,omitempty"`
	Source      *Source `json:"source,omitempty"`
	Path        string  `json:"path,omitempty"`
}

type PluginMetadata struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type ProjectPlugin struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

type ProjectAgent struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
	Ref    string `json:"ref,omitempty"`
}

type ProjectSkill struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

type ProjectRuntimeEnablement struct {
	Skills  []string `json:"skills,omitempty"`
	Plugins []string `json:"plugins,omitempty"`
	Agents  []string `json:"agents,omitempty"`
}

type ProjectAgentEnablement = ProjectRuntimeEnablement

type PluginProvenance struct {
	Source string `json:"source,omitempty"`
	Ref    string `json:"ref,omitempty"`
	State  string `json:"state,omitempty"`
}

type InstallRequest struct {
	Source  *Source  `json:"source,omitempty"`
	Scope   string   `json:"scope,omitempty"`
	Mode    string   `json:"mode,omitempty"`
	Agent   string   `json:"agent,omitempty"`
	Version string   `json:"version,omitempty"`
	Ref     string   `json:"ref,omitempty"`
	All     bool     `json:"all,omitempty"`
	Force   bool     `json:"force,omitempty"`
	Yes     bool     `json:"yes,omitempty"`
	Skills  []string `json:"skills,omitempty"`
}

type ProjectConfig struct {
	Skills         []ProjectSkill                      `json:"skills,omitempty"`
	Plugins        []ProjectPlugin                     `json:"plugins,omitempty"`
	Agents         []ProjectAgent                      `json:"agents,omitempty"`
	Runtimes       map[string]ProjectRuntimeEnablement `json:"runtimes,omitempty"`
	ExposureMethod ExposureMethod                      `json:"exposureMethod,omitempty"`
	AutoApply      bool                                `json:"autoApply,omitempty"`
}

type InstallResult struct {
	Installed []InstalledSkill `json:"installed,omitempty"`
	Skipped   []string         `json:"skipped,omitempty"`
}

// Lockfile captures the resolved/materialized state for a project.
// It is not the authoritative project declaration source.
type Lockfile struct {
	Version        int                         `json:"version"`
	Scope          string                      `json:"scope,omitempty"`
	ExposureMethod ExposureMethod              `json:"exposureMethod,omitempty"`
	AutoApply      bool                        `json:"autoApply,omitempty"`
	Skills         []LockSkill                 `json:"skills,omitempty"`
	Plugins        []LockPlugin                `json:"plugins,omitempty"`
	Agents         []LockAgent                 `json:"agents,omitempty"`
	Runtimes       map[string]LockRuntimeEntry `json:"runtimes,omitempty"`
}

type LockAgent struct {
	ID     string `json:"id"`
	Source string `json:"source,omitempty"`
	Ref    string `json:"ref,omitempty"`
	Path   string `json:"path,omitempty"`
}

type LockRuntimeEntry struct {
	Skills  []string `json:"skills,omitempty"`
	Plugins []string `json:"plugins,omitempty"`
	Agents  []string `json:"agents,omitempty"`
}

type LockSkill struct {
	ID           string           `json:"id"`
	Declared     LockDeclared     `json:"declared"`
	Resolved     LockResolved     `json:"resolved"`
	Materialized LockMaterialized `json:"materialized"`
	Projected    LockProjected    `json:"projected"`
}

type LockPlugin struct {
	ID           string              `json:"id"`
	Declared     LockDeclared        `json:"declared"`
	Resolved     LockResolved        `json:"resolved"`
	Materialized LockMaterialized    `json:"materialized"`
	Projected    LockPluginProjected `json:"projected"`
}

type LockDeclared struct {
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

type LockResolved struct {
	Source   string `json:"source"`
	Ref      string `json:"ref,omitempty"`
	Revision string `json:"revision,omitempty"`
}

type LockMaterialized struct {
	Path string `json:"path"`
}

type LockProjected struct {
	Mode      string     `json:"mode,omitempty"`
	Exposures []Exposure `json:"exposures,omitempty"`
}

type LockPluginProjected struct {
	Path string `json:"path"`
}

type SkillSource struct {
	Locator          string `json:"locator,omitempty"`
	CloneURL         string `json:"cloneURL,omitempty"`
	Path             string `json:"path,omitempty"`
	RequestedVersion string `json:"requestedVersion,omitempty"`
}

type SkillInstall struct {
	Mode          string `json:"mode,omitempty"`
	CanonicalPath string `json:"canonicalPath,omitempty"`
}

type Exposure struct {
	Agent string `json:"agent,omitempty"`
	Path  string `json:"path,omitempty"`
}

type InstalledSkill struct {
	ID        string        `json:"id"`
	Source    *SkillSource  `json:"source,omitempty"`
	Install   *SkillInstall `json:"install,omitempty"`
	Exposures []Exposure    `json:"exposures,omitempty"`
}
