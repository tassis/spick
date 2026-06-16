package model

type Source struct {
	Locator         string `json:"locator,omitempty"`
	Path            string `json:"path,omitempty"`
	RequestedVersion string `json:"requestedVersion,omitempty"`
	Ref             string `json:"ref,omitempty"`
}

type CatalogSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Source      *Source `json:"source,omitempty"`
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

type InstallResult struct {
	Installed []InstalledSkill `json:"installed,omitempty"`
	Skipped   []string         `json:"skipped,omitempty"`
}

type Lockfile struct {
	Version int              `json:"version"`
	Scope   string           `json:"scope,omitempty"`
	Skills  map[string]Skill `json:"skills,omitempty"`
}

type Skill struct {
	ID        string        `json:"id"`
	Source    *SkillSource  `json:"source,omitempty"`
	Install   *SkillInstall `json:"install,omitempty"`
	Exposures []Exposure    `json:"exposures,omitempty"`
}

type SkillSource struct {
	Locator         string `json:"locator,omitempty"`
	Path            string `json:"path,omitempty"`
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
