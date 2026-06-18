package model

type Source struct {
	Locator          string `json:"locator,omitempty"`
	CloneURL         string `json:"cloneURL,omitempty"`
	Path             string `json:"path,omitempty"`
	RequestedVersion string `json:"requestedVersion,omitempty"`
	Ref              string `json:"ref,omitempty"`
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

type ProjectSkill struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

type ProjectAgentEnablement struct {
	Skills  []string `json:"skills,omitempty"`
	Plugins []string `json:"plugins,omitempty"`
}

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
	Skills         []ProjectSkill                    `json:"skills,omitempty"`
	Plugins        []ProjectPlugin                   `json:"plugins,omitempty"`
	Agents         map[string]ProjectAgentEnablement `json:"agents,omitempty"`
	ExposureMethod string                            `json:"exposureMethod,omitempty"`
	AutoApply      bool                              `json:"autoApply,omitempty"`
}

type InstallResult struct {
	Installed []InstalledSkill `json:"installed,omitempty"`
	Skipped   []string         `json:"skipped,omitempty"`
}

// Lockfile captures the resolved/materialized state for a project.
// It is not the authoritative project declaration source.
type Lockfile struct {
	Version int          `json:"version"`
	Scope   string       `json:"scope,omitempty"`
	Skills  []LockSkill  `json:"skills,omitempty"`
	Plugins []LockPlugin `json:"plugins,omitempty"`
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
