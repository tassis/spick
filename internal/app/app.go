package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tassis/spick/internal/agents"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/lock"
	"github.com/tassis/spick/internal/model"
	"github.com/tassis/spick/internal/plugins"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

type App struct {
	Prompter  ui.Prompter
	Workspace *workspace.Workspace
	Skills    *skills.Service
	Locks     *lock.Store
}

type SkillReconcileInputs struct {
	Declared     []model.ProjectSkill
	Enabled      map[string]model.ProjectAgentEnablement
	Materialized []model.InstalledSkill
}

type SkillReconcileAction struct {
	ID           string
	Declared     *model.ProjectSkill
	Materialized *model.InstalledSkill
	Agents       []string
}

type PluginReconcileInputs struct {
	Declared     []model.ProjectPlugin
	Enabled      map[string]model.ProjectAgentEnablement
	Materialized []model.LockPlugin
}

type PluginReconcileAction struct {
	ID           string
	Declared     *model.ProjectPlugin
	Materialized *model.LockPlugin
}

type SyncResult struct {
	SkillMessages  []string `json:"skillMessages,omitempty"`
	PluginMessages []string `json:"pluginMessages,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

func New(prompter ui.Prompter, ws *workspace.Workspace, svc *skills.Service) *App {
	return &App{Prompter: prompter, Workspace: ws, Skills: svc, Locks: lock.New(ws.Root)}
}

func SourceFromLocator(locator string) model.Source { return model.Source{Locator: locator} }

func applyRequestedVersion(parsed *workspace.ParsedSource) error {
	if parsed == nil {
		return fmt.Errorf("source is required")
	}
	return nil
}

type PluginSourceKind string

const PluginSourceKindPluginSource = PluginSourceKind("plugin-source")

type PluginSourceResult struct {
	Kind   PluginSourceKind      `json:"kind"`
	Source model.Source          `json:"source"`
	Plugin PluginInspectMetadata `json:"plugin"`
}

type PluginInspectMetadata struct {
	ID          string `json:"id"`
	Runtime     string `json:"runtime"`
	Entry       string `json:"entry"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Revision    string `json:"revision,omitempty"`
}

type PluginInspectOptions struct {
	Scope  config.Scope
	Source model.Source
	JSON   bool
}

type AddPluginOptions struct {
	Scope  config.Scope
	Source model.Source
	Yes    bool
	Force  bool
}

type AddResult struct {
	Source   model.Source                  `json:"source"`
	Catalog  []model.CatalogSkill          `json:"catalog,omitempty"`
	Selected []model.CatalogSkill          `json:"selected,omitempty"`
	Message  string                        `json:"message,omitempty"`
	Added    []skills.InstalledSkillResult `json:"added,omitempty"`
}

func (a *App) projectHasSkillID(projectConfig *workspace.ProjectConfig, id string) bool {
	if projectConfig == nil {
		return false
	}
	for _, skill := range projectConfig.Skills {
		if skill.ID == id {
			return true
		}
	}
	return false
}

func (a *App) projectHasPluginID(projectConfig *workspace.ProjectConfig, id string) bool {
	if projectConfig == nil {
		return false
	}
	for _, plugin := range projectConfig.Plugins {
		if plugin.ID == id {
			return true
		}
	}
	return false
}

func (a *App) Add(opts AddOptions) (*AddResult, error) {
	if a == nil || a.Workspace == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if err := validateAddOptions(opts); err != nil {
		return nil, err
	}
	projectConfig, err := a.Workspace.LoadProjectConfig()
	if err != nil {
		return nil, err
	}
	rawSource := opts.Source.Locator
	if rawSource == "" {
		rawSource = opts.Source.CloneURL
	}
	parsed, err := a.Workspace.ParseSource(rawSource)
	if err != nil {
		return nil, err
	}
	if err := validateHostedOptions(parsed.Kind, opts); err != nil {
		return nil, err
	}
	if err := applyRequestedVersion(parsed); err != nil {
		return nil, err
	}
	opened, err := a.Workspace.OpenSource(parsed.Source)
	if err != nil {
		return nil, err
	}
	originalSource := parsed.Source
	opts.Source = opened
	if opts.Source.Path == "" {
		return nil, fmt.Errorf("local source path required")
	}
	catalog, err := a.Workspace.BuildCatalog(opts.Scope, opts.Source)
	if err != nil {
		return nil, err
	}
	selected, err := selectCatalogSkills(a.Prompter, catalog, opts)
	if err != nil {
		return nil, err
	}
	for _, skill := range selected {
		if a.projectHasSkillID(projectConfig, skill.ID) && !opts.Force {
			return nil, fmt.Errorf("skill %q already declared; use --force to replace", skill.ID)
		}
	}
	declared := append([]model.ProjectSkill(nil), projectConfig.Skills...)
	for _, skill := range selected {
		declared = upsertProjectSkill(declared, model.ProjectSkill{ID: skill.ID, Source: nonEmpty(originalSource.Locator, originalSource.CloneURL, originalSource.Path), Ref: originalSource.RequestedVersion})
	}
	if err := a.Workspace.WriteProjectSkills(declared); err != nil {
		return nil, err
	}
	if projectConfig.AutoApply && len(selected) > 0 {
		for agent, enablement := range projectConfig.Agents {
			ids := append([]string(nil), enablement.Skills...)
			for _, skill := range selected {
				if !containsString(ids, skill.ID) {
					ids = append(ids, skill.ID)
				}
			}
			if err := a.Workspace.WriteProjectAgentEnablement(agent, ids, enablement.Plugins); err != nil {
				return nil, err
			}
		}
	}
	result := &AddResult{Source: opts.Source, Catalog: catalog, Selected: selected}
	if len(selected) == 0 {
		result.Message = "no skills selected"
		return result, nil
	}
	if a.Skills == nil {
		result.Message = "selected skills"
		return result, nil
	}
	agents := effectiveAgents(opts.Agent, projectConfig.Agents)
	autoApply := projectConfig.AutoApply
	added, err := a.Skills.Add(skills.AddOptions{Scope: string(opts.Scope), SourceRoot: opts.Source.Path, All: opts.All, Skills: opts.Skills, Selected: selected, ExposureMethod: nonEmpty(opts.ExposureMethod, projectConfig.ExposureMethod, "symlink"), Agents: agents, Force: opts.Force, AutoApply: &autoApply})
	if err != nil {
		return nil, err
	}
	result.Added = added.Installed
	if a.Locks != nil {
		installed := make([]model.InstalledSkill, 0, len(added.Installed))
		for _, item := range added.Installed {
			installed = append(installed, model.InstalledSkill{ID: item.ID, Source: mergeSkillSource(&originalSource, item.Source), Install: &model.SkillInstall{Mode: item.Mode, CanonicalPath: item.Target}, Exposures: item.Exposures})
		}
		if err := a.Locks.UpsertInstalled(string(opts.Scope), installed); err != nil {
			return nil, err
		}
	}
	if len(added.Installed) > 0 {
		result.Message = "selected and materialized skills"
	}
	return result, nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func upsertProjectSkill(items []model.ProjectSkill, next model.ProjectSkill) []model.ProjectSkill {
	for i, item := range items {
		if item.ID == next.ID {
			items[i] = next
			return items
		}
	}
	return append(items, next)
}

func validateAddOptions(opts AddOptions) error {
	if err := agents.Validate(opts.Agent); err != nil {
		return err
	}
	if opts.ExposureMethod != "" && opts.ExposureMethod != "copy" && opts.ExposureMethod != "symlink" {
		return fmt.Errorf("unsupported exposure method %q", opts.ExposureMethod)
	}
	return nil
}

func effectiveAgents(explicit string, configured map[string]model.ProjectAgentEnablement) []string {
	if explicit != "" {
		return []string{explicit}
	}
	if len(configured) > 0 {
		out := make([]string, 0, len(configured))
		for agent := range configured {
			out = append(out, agent)
		}
		sort.Strings(out)
		return out
	}
	return nil
}

func validateHostedOptions(kind string, opts AddOptions) error {
	if kind == "local" {
	}
	return nil
}

type ListResult struct {
	Scope  string                 `json:"scope"`
	Skills []model.InstalledSkill `json:"skills"`
}

func mergeSkillSource(original, discovered *model.Source) *model.SkillSource {
	if original == nil && discovered == nil {
		return nil
	}
	out := &model.SkillSource{}
	if original != nil {
		out.Locator = original.Locator
		out.CloneURL = original.CloneURL
		out.RequestedVersion = original.RequestedVersion
	}
	if discovered != nil {
		if out.Path == "" {
			out.Path = discovered.Path
		}
		if out.Locator == "" {
			out.Locator = discovered.Locator
		}
		if out.CloneURL == "" {
			out.CloneURL = discovered.CloneURL
		}
		if out.RequestedVersion == "" {
			out.RequestedVersion = discovered.RequestedVersion
		}
	}
	return out
}

func skillReconcileInputs(project *workspace.ProjectConfig, locks *lock.Store, scope string) (*SkillReconcileInputs, error) {
	inputs := &SkillReconcileInputs{}
	if project != nil {
		inputs.Declared = append([]model.ProjectSkill(nil), project.Skills...)
		inputs.Enabled = cloneProjectAgentEnablement(project.Agents)
	}
	if locks == nil {
		return inputs, nil
	}
	items, err := locks.List(scope)
	if err != nil {
		if os.IsNotExist(err) {
			return inputs, nil
		}
		return nil, err
	}
	inputs.Materialized = append([]model.InstalledSkill(nil), items...)
	return inputs, nil
}

func pluginReconcileInputs(project *workspace.ProjectConfig, locks *lock.Store, scope string) (*PluginReconcileInputs, error) {
	inputs := &PluginReconcileInputs{}
	if project != nil {
		inputs.Declared = append([]model.ProjectPlugin(nil), project.Plugins...)
		inputs.Enabled = cloneProjectAgentEnablement(project.Agents)
	}
	if locks == nil {
		return inputs, nil
	}
	items, err := locks.ListPlugins(scope)
	if err != nil {
		if os.IsNotExist(err) {
			return inputs, nil
		}
		return nil, err
	}
	inputs.Materialized = append([]model.LockPlugin(nil), items...)
	return inputs, nil
}

func planSkillReconcile(inputs *SkillReconcileInputs) []SkillReconcileAction {
	if inputs == nil {
		return nil
	}
	declared := map[string]model.ProjectSkill{}
	for _, skill := range inputs.Declared {
		declared[skill.ID] = skill
	}
	materialized := map[string]model.InstalledSkill{}
	for _, skill := range inputs.Materialized {
		materialized[skill.ID] = skill
	}
	ids := make([]string, 0, len(declared)+len(materialized))
	seen := map[string]bool{}
	for id := range declared {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for id := range materialized {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	actions := make([]SkillReconcileAction, 0, len(ids))
	for _, id := range ids {
		action := SkillReconcileAction{ID: id}
		if decl, ok := declared[id]; ok {
			action.Declared = &decl
		}
		if mat, ok := materialized[id]; ok {
			action.Materialized = &mat
		}
		for agent, entry := range inputs.Enabled {
			if containsString(entry.Skills, id) {
				action.Agents = append(action.Agents, agent)
			}
		}
		sort.Strings(action.Agents)
		actions = append(actions, action)
	}
	return actions
}

func planPluginReconcile(inputs *PluginReconcileInputs) []PluginReconcileAction {
	if inputs == nil {
		return nil
	}
	declared := map[string]model.ProjectPlugin{}
	for _, plugin := range inputs.Declared {
		declared[plugin.ID] = plugin
	}
	materialized := map[string]model.LockPlugin{}
	for _, plugin := range inputs.Materialized {
		materialized[plugin.ID] = plugin
	}
	ids := make([]string, 0, len(declared)+len(materialized))
	seen := map[string]bool{}
	for id := range declared {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for id := range materialized {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	actions := make([]PluginReconcileAction, 0, len(ids))
	for _, id := range ids {
		action := PluginReconcileAction{ID: id}
		if decl, ok := declared[id]; ok {
			action.Declared = &decl
		}
		if mat, ok := materialized[id]; ok {
			action.Materialized = &mat
		}
		actions = append(actions, action)
	}
	return actions
}

func (a *App) Sync(scope config.Scope, locked bool) (*SyncResult, error) {
	if a == nil || a.Workspace == nil || a.Locks == nil || a.Skills == nil {
		return nil, fmt.Errorf("workspace, skills, and locks are required")
	}
	project, err := a.Workspace.LoadProjectConfig()
	if err != nil {
		return nil, err
	}
	if locked {
		return a.syncLocked(scope, project)
	}
	return a.syncUnlocked(scope, project)
}

func (a *App) syncUnlocked(scope config.Scope, project *workspace.ProjectConfig) (*SyncResult, error) {
	result := &SyncResult{}
	declaredSkills := append([]model.ProjectSkill(nil), project.Skills...)
	declaredPlugins := append([]model.ProjectPlugin(nil), project.Plugins...)
	openedSkills := map[string]model.Source{}
	for _, skill := range declaredSkills {
		opened, err := a.Workspace.OpenSource(model.Source{Locator: skill.Source})
		if err != nil {
			return nil, err
		}
		opened.Path = resolveWorkspaceSourcePath(a.Workspace.Root, opened.Path)
		openedSkills[skill.ID] = opened
	}
	openedPlugins := map[string]model.Source{}
	for _, plugin := range declaredPlugins {
		opened, err := a.Workspace.OpenSource(model.Source{Locator: plugin.Source})
		if err != nil {
			return nil, err
		}
		opened.Path = resolveWorkspaceSourcePath(a.Workspace.Root, opened.Path)
		if _, err := plugins.LoadManifest(opened.Path); err != nil {
			return nil, err
		}
		openedPlugins[plugin.ID] = opened
	}
	for _, skill := range declaredSkills {
		opened := openedSkills[skill.ID]
		catalog, err := a.Workspace.BuildCatalog(scope, opened)
		if err != nil {
			return nil, err
		}
		selected, err := selectCatalogSkills(a.Prompter, catalog, AddOptions{Scope: scope, Source: opened, Skills: []string{skill.ID}, All: false})
		if err != nil {
			return nil, err
		}
		added, err := a.Skills.Add(skills.AddOptions{Scope: string(scope), SourceRoot: opened.Path, Selected: selected, ExposureMethod: nonEmpty(project.ExposureMethod, "symlink"), Agents: agentsFromEnablement(project.Agents, skill.ID), AutoApply: &project.AutoApply, Force: true})
		if err != nil {
			return nil, err
		}
		installed := make([]model.InstalledSkill, 0, len(added.Installed))
		for _, item := range added.Installed {
			installed = append(installed, model.InstalledSkill{ID: item.ID, Source: mergeSkillSource(&model.Source{Locator: skill.Source}, item.Source), Install: &model.SkillInstall{Mode: item.Mode, CanonicalPath: item.Target}, Exposures: item.Exposures})
		}
		if err := a.Locks.UpsertInstalled(string(scope), installed); err != nil {
			return nil, err
		}
		result.SkillMessages = append(result.SkillMessages, fmt.Sprintf("restored skill %s", skill.ID))
	}
	for _, plugin := range declaredPlugins {
		opened := openedPlugins[plugin.ID]
		mat, err := plugins.Materialize(plugins.MaterializeOptions{WorkspaceRoot: a.Workspace.Root, Scope: string(scope), SourceRoot: opened.Path, ID: plugin.ID, Force: true})
		if err != nil {
			return nil, err
		}
		lockPlugin := model.LockPlugin{ID: plugin.ID, Declared: model.LockDeclared{Source: plugin.Source, Ref: plugin.Ref}, Resolved: model.LockResolved{Source: plugin.Source, Ref: plugin.Ref, Revision: opened.RequestedVersion}, Materialized: model.LockMaterialized{Path: mat.ManagedPath}, Projected: model.LockPluginProjected{Path: mat.RuntimePath}}
		if err := a.Locks.UpsertPlugins(string(scope), []model.LockPlugin{lockPlugin}); err != nil {
			return nil, err
		}
	}
	if lf, err := a.Locks.Read(string(scope)); err == nil {
		for _, sk := range lf.Skills {
			if !containsSkill(declaredSkills, sk.ID) {
				_, _ = a.Locks.Remove(string(scope), []string{sk.ID})
			}
		}
		for _, pl := range lf.Plugins {
			if !containsPlugin(declaredPlugins, pl.ID) {
				_ = a.Locks.RemovePlugins(string(scope), []string{pl.ID})
				result.PluginMessages = append(result.PluginMessages, fmt.Sprintf("unmanaged plugin material present: %s", pl.ID))
			}
		}
	}
	if len(result.SkillMessages) == 0 {
		result.SkillMessages = append(result.SkillMessages, "skills already in sync")
	}
	if len(result.PluginMessages) == 0 {
		result.PluginMessages = append(result.PluginMessages, "plugins already in sync")
	}
	return result, nil
}

func (a *App) syncLocked(scope config.Scope, project *workspace.ProjectConfig) (*SyncResult, error) {
	lf, err := a.Locks.Read(string(scope))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("locked sync requires an existing lockfile")
		}
		return nil, err
	}
	if err := validateLockedSyncInputs(project, lf); err != nil {
		return nil, err
	}
	result := &SyncResult{}
	warnedSkills := map[string][]string{}
	warnedPlugins := map[string][]string{}
	declaredSkills := map[string]struct{}{}
	for _, s := range project.Skills {
		declaredSkills[s.ID] = struct{}{}
	}
	declaredPlugins := map[string]struct{}{}
	for _, p := range project.Plugins {
		declaredPlugins[p.ID] = struct{}{}
	}
	for _, sk := range lf.Skills {
		if _, ok := declaredSkills[sk.ID]; !ok {
			continue
		}
		if sk.Materialized.Path == "" || sk.Projected.Mode == "" || len(sk.Projected.Exposures) == 0 {
			return nil, fmt.Errorf("locked sync requires installed materialization for skill %s", sk.ID)
		}
		managedSkill := resolveWorkspaceSourcePath(a.Workspace.Root, sk.Materialized.Path)
		if _, err := os.Stat(managedSkill); os.IsNotExist(err) {
			opened, err := a.Workspace.OpenSource(model.Source{Locator: sk.Declared.Source})
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for skill %s: %w", sk.ID, err)
			}
			opened.Path = resolveWorkspaceSourcePath(a.Workspace.Root, opened.Path)
			catalog, err := a.Workspace.BuildCatalog(scope, opened)
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for skill %s: %w", sk.ID, err)
			}
			selected, err := selectCatalogSkills(a.Prompter, catalog, AddOptions{Scope: scope, Source: opened, Skills: []string{sk.ID}, All: false})
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for skill %s: %w", sk.ID, err)
			}
			added, err := a.Skills.Add(skills.AddOptions{Scope: string(scope), SourceRoot: opened.Path, Selected: selected, ExposureMethod: sk.Projected.Mode, Agents: agentsFromLockedExposures(sk.Projected.Exposures), Force: true})
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for skill %s: %w", sk.ID, err)
			}
			if len(added.Installed) == 0 || added.Installed[0].Target == "" {
				return nil, fmt.Errorf("locked sync snapshot insufficiency for skill %s", sk.ID)
			}
			managedSkill = added.Installed[0].Target
			if isUnpinnedSource(sk.Declared.Source, sk.Declared.Ref) {
				warnedSkills[sk.Declared.Source] = append(warnedSkills[sk.Declared.Source], sk.ID)
			}
		}
		for _, ex := range sk.Projected.Exposures {
			path := resolveWorkspaceSourcePath(a.Workspace.Root, ex.Path)
			if err := os.RemoveAll(path); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(filepath.Dir(path), managedSkill)
			if err != nil {
				return nil, err
			}
			if err := os.Symlink(rel, path); err != nil {
				return nil, err
			}
		}
		result.SkillMessages = append(result.SkillMessages, fmt.Sprintf("restored skill %s", sk.ID))
	}
	for _, pl := range lf.Plugins {
		if _, ok := declaredPlugins[pl.ID]; !ok {
			continue
		}
		if pl.Materialized.Path == "" || pl.Projected.Path == "" {
			return nil, fmt.Errorf("locked sync snapshot insufficiency for plugin %s", pl.ID)
		}
		managedPlugin := resolveWorkspaceSourcePath(a.Workspace.Root, pl.Materialized.Path)
		if _, err := os.Stat(managedPlugin); os.IsNotExist(err) {
			opened, err := a.Workspace.OpenSource(model.Source{Locator: pl.Declared.Source})
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for plugin %s: %w", pl.ID, err)
			}
			opened.Path = resolveWorkspaceSourcePath(a.Workspace.Root, opened.Path)
			mat, err := plugins.Materialize(plugins.MaterializeOptions{WorkspaceRoot: a.Workspace.Root, Scope: string(scope), SourceRoot: opened.Path, ID: pl.ID, Force: true})
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for plugin %s: %w", pl.ID, err)
			}
			managedPlugin = mat.ManagedPath
			if isUnpinnedSource(pl.Declared.Source, pl.Declared.Ref) {
				warnedPlugins[pl.Declared.Source] = append(warnedPlugins[pl.Declared.Source], pl.ID)
			}
		}
		proj := resolveWorkspaceSourcePath(a.Workspace.Root, pl.Projected.Path)
		if proj != managedPlugin {
			if err := os.RemoveAll(proj); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(filepath.Dir(proj), 0o755); err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(filepath.Dir(proj), managedPlugin)
			if err != nil {
				return nil, err
			}
			if err := os.Symlink(rel, proj); err != nil {
				return nil, err
			}
		}
		result.PluginMessages = append(result.PluginMessages, fmt.Sprintf("restored plugin %s", pl.ID))
	}
	for source, ids := range warnedSkills {
		if len(ids) > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("locked restore refetched unpinned skill source %s for ids: %s", source, strings.Join(ids, ", ")))
		}
	}
	for source, ids := range warnedPlugins {
		if len(ids) > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("locked restore refetched unpinned plugin source %s for ids: %s", source, strings.Join(ids, ", ")))
		}
	}
	if len(result.SkillMessages) == 0 {
		result.SkillMessages = append(result.SkillMessages, "skills already in sync")
	}
	if len(result.PluginMessages) == 0 {
		result.PluginMessages = append(result.PluginMessages, "plugins already in sync")
	}
	return result, nil
}

func containsSkill(skills []model.ProjectSkill, id string) bool {
	for _, s := range skills {
		if s.ID == id {
			return true
		}
	}
	return false
}
func containsPlugin(plugins []model.ProjectPlugin, id string) bool {
	for _, p := range plugins {
		if p.ID == id {
			return true
		}
	}
	return false
}

func agentsFromLockedExposures(exposures []model.Exposure) []string {
	if len(exposures) == 0 {
		return []string{"opencode"}
	}
	out := make([]string, 0, len(exposures))
	for _, ex := range exposures {
		if ex.Agent != "" {
			out = append(out, ex.Agent)
		}
	}
	if len(out) == 0 {
		return []string{"opencode"}
	}
	return out
}

func isUnpinnedSource(source, ref string) bool {
	return source != "" && ref == ""
}

func resolveWorkspaceSourcePath(root, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	if root == "" {
		root = "."
	}
	return filepath.Join(root, p)
}
func agentsFromEnablement(enabled map[string]model.ProjectAgentEnablement, id string) []string {
	var out []string
	for agent, entry := range enabled {
		if containsString(entry.Skills, id) {
			out = append(out, agent)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return []string{"opencode"}
	}
	return out
}

func cloneProjectAgentEnablement(in map[string]model.ProjectAgentEnablement) map[string]model.ProjectAgentEnablement {
	if len(in) == 0 {
		return map[string]model.ProjectAgentEnablement{}
	}
	out := make(map[string]model.ProjectAgentEnablement, len(in))
	for k, v := range in {
		out[k] = model.ProjectAgentEnablement{Skills: append([]string(nil), v.Skills...), Plugins: append([]string(nil), v.Plugins...)}
	}
	return out
}

func validateLockedSyncInputs(project *workspace.ProjectConfig, lf *model.Lockfile) error {
	if lf == nil {
		return fmt.Errorf("locked sync requires an existing lockfile")
	}
	declaredSkills := map[string]model.ProjectSkill{}
	for _, skill := range project.Skills {
		declaredSkills[skill.ID] = skill
	}
	lockedSkills := map[string]model.LockSkill{}
	for _, skill := range lf.Skills {
		lockedSkills[skill.ID] = skill
	}
	for id, decl := range declaredSkills {
		locked, ok := lockedSkills[id]
		if !ok {
			return fmt.Errorf("locked sync requires matching lock entry for skill %s", id)
		}
		if decl.Source == "" || locked.Declared.Source == "" {
			return fmt.Errorf("locked sync requires declared lock info for skill %s", id)
		}
		if decl.Ref != "" && locked.Declared.Ref != "" && decl.Ref != locked.Declared.Ref {
			return fmt.Errorf("locked sync requires config/lock match for skill %s", id)
		}
	}
	for id := range lockedSkills {
		if _, ok := declaredSkills[id]; !ok {
			return fmt.Errorf("locked sync requires config/lock match for skill %s", id)
		}
	}
	declaredPlugins := map[string]model.ProjectPlugin{}
	for _, plugin := range project.Plugins {
		declaredPlugins[plugin.ID] = plugin
	}
	lockedPlugins := map[string]model.LockPlugin{}
	for _, plugin := range lf.Plugins {
		lockedPlugins[plugin.ID] = plugin
	}
	for id, decl := range declaredPlugins {
		locked, ok := lockedPlugins[id]
		if !ok {
			return fmt.Errorf("locked sync requires matching lock entry for plugin %s", id)
		}
		if decl.Source == "" || locked.Declared.Source == "" || locked.Materialized.Path == "" || locked.Projected.Path == "" {
			return fmt.Errorf("locked sync requires snapshot lock info for plugin %s", id)
		}
	}
	for id := range lockedPlugins {
		if _, ok := declaredPlugins[id]; !ok {
			return fmt.Errorf("locked sync requires config/lock match for plugin %s", id)
		}
	}
	return nil
}

func validateLockedMaterialization(inputs *SkillReconcileInputs, lf *model.Lockfile) error {
	if inputs == nil || lf == nil {
		return nil
	}
	for _, materialized := range inputs.Materialized {
		if materialized.Install == nil || materialized.Install.CanonicalPath == "" {
			return fmt.Errorf("locked sync requires installed materialization for %s", materialized.ID)
		}
	}
	return nil
}

func (a *App) List(opts ListOptions) (*ListResult, error) {
	if a == nil || a.Locks == nil {
		return &ListResult{Scope: string(opts.Scope)}, nil
	}
	items, err := a.Locks.List(string(opts.Scope))
	if err != nil {
		if os.IsNotExist(err) {
			return &ListResult{Scope: string(opts.Scope)}, nil
		}
		return nil, err
	}
	return &ListResult{Scope: string(opts.Scope), Skills: items}, nil
}

type RemoveResult struct {
	Removed  []string `json:"removed,omitempty"`
	Purged   []string `json:"purged,omitempty"`
	Pruned   []string `json:"pruned,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Message  string   `json:"message,omitempty"`
}

type PluginListItem struct {
	ID        string               `json:"id"`
	Declared  *model.ProjectPlugin `json:"declared,omitempty"`
	Resolved  *model.LockPlugin    `json:"resolved,omitempty"`
	Installed *model.LockPlugin    `json:"installed,omitempty"`
	State     string               `json:"state"`
	Drift     bool                 `json:"drift,omitempty"`
	Warning   string               `json:"warning,omitempty"`
}

type PluginListResult struct {
	Scope   string           `json:"scope"`
	Plugins []PluginListItem `json:"plugins"`
}

type RemovePluginOptions struct {
	Scope config.Scope
	IDs   []string
}

func (a *App) Remove(opts RemoveOptions) (*RemoveResult, error) {
	if a == nil || a.Locks == nil {
		return &RemoveResult{Message: "no lock store"}, nil
	}
	if a.Workspace != nil {
		if err := a.Workspace.RemoveProjectSkills(opts.Skills); err != nil {
			return nil, err
		}
	}
	if err := a.Locks.RemoveFiles(string(opts.Scope), opts.Skills, opts.PruneUnused); err != nil {
		return nil, err
	}
	return &RemoveResult{Removed: opts.Skills, Purged: opts.Skills, Message: "removed skills from canonical storage and exposures"}, nil
}

func (a *App) ListPlugins(opts ListOptions) (*PluginListResult, error) {
	if a == nil || a.Workspace == nil {
		return &PluginListResult{Scope: string(opts.Scope)}, nil
	}
	project, err := a.Workspace.LoadProjectConfig()
	if err != nil {
		return nil, err
	}
	var locks []model.LockPlugin
	if a.Locks != nil {
		locks, err = a.Locks.ListPlugins(string(opts.Scope))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	byID := map[string]model.LockPlugin{}
	bySource := map[string]model.LockPlugin{}
	byPath := map[string]model.LockPlugin{}
	for _, pl := range locks {
		byID[pl.ID] = pl
		for _, key := range pluginMatchCandidates(a.Workspace.Root, pl.Declared.Source, pl.Materialized.Path, pl.Projected.Path, pl.Projected.Path) {
			if _, ok := bySource[key]; !ok {
				bySource[key] = pl
			}
			if _, ok := byPath[key]; !ok {
				byPath[key] = pl
			}
		}
	}
	seenIDs := map[string]bool{}
	seenSources := map[string]bool{}
	seenPaths := map[string]bool{}
	out := make([]PluginListItem, 0, len(project.Plugins)+len(locks))
	for _, decl := range project.Plugins {
		item := PluginListItem{ID: decl.Source, Declared: &decl, State: "declared"}
		matched := false
		for _, key := range pluginMatchCandidates(a.Workspace.Root, decl.Source) {
			if locked, ok := bySource[key]; ok {
				item.ID = locked.ID
				item.Resolved = &locked
				item.Installed = &locked
				item.State = "installed"
				seenIDs[locked.ID] = true
				seenSources[key] = true
				matched = true
				break
			}
			if locked, ok := byPath[key]; ok {
				item.ID = locked.ID
				item.Resolved = &locked
				item.Installed = &locked
				item.State = "installed"
				seenIDs[locked.ID] = true
				seenPaths[key] = true
				matched = true
				break
			}
		}
		if !matched {
			item.State = "missing"
			item.Drift = true
			item.Warning = "not installed"
		}
		if item.Installed != nil && item.Installed.Projected.Path != "" {
			checkPath := item.Installed.Projected.Path
			if a.Workspace != nil && !strings.HasPrefix(checkPath, string(os.PathSeparator)) {
				checkPath = filepath.Join(a.Workspace.Root, checkPath)
			}
			if _, err := os.Stat(checkPath); os.IsNotExist(err) {
				item.State = "missing"
				item.Drift = true
				item.Warning = "managed runtime material is missing"
			}
		}
		out = append(out, item)
		seenSources[decl.Source] = true
	}
	for _, locked := range locks {
		if seenIDs[locked.ID] {
			continue
		}
		extra := true
		if locked.Declared.Source != "" {
			for _, key := range pluginMatchCandidates(a.Workspace.Root, locked.Declared.Source, locked.Materialized.Path, locked.Materialized.Path, locked.Materialized.Path) {
				if seenSources[key] || seenPaths[key] {
					seenIDs[locked.ID] = true
					extra = false
					break
				}
			}
		}
		if !extra {
			continue
		}
		l := locked
		out = append(out, PluginListItem{ID: l.ID, Resolved: &l, Installed: &l, State: "extra"})
	}
	return &PluginListResult{Scope: string(opts.Scope), Plugins: out}, nil
}

func pluginMatchCandidates(root string, values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values)*2)
	for _, value := range values {
		for _, candidate := range normalizedPluginCandidates(root, value) {
			if candidate != "" && !seen[candidate] {
				seen[candidate] = true
				out = append(out, candidate)
			}
		}
	}
	return out
}

func normalizedPluginCandidates(root, value string) []string {
	if value == "" {
		return nil
	}
	out := []string{filepath.Clean(value)}
	if root != "" && !filepath.IsAbs(value) {
		out = append(out, filepath.Clean(filepath.Join(root, value)))
	}
	return out
}

func (a *App) RemovePlugin(opts RemovePluginOptions) (*RemoveResult, error) {
	if a == nil || a.Workspace == nil || a.Locks == nil {
		return nil, fmt.Errorf("workspace and locks are required")
	}
	if len(opts.IDs) == 0 {
		return &RemoveResult{Message: "no plugins selected"}, nil
	}
	listed, err := a.ListPlugins(ListOptions{Scope: opts.Scope})
	if err != nil {
		return nil, err
	}
	decls := map[string]string{}
	for _, item := range listed.Plugins {
		if item.Declared != nil {
			decls[item.Declared.Source] = item.ID
		}
		decls[item.ID] = item.ID
	}
	projectIDs := make([]string, 0, len(opts.IDs))
	lockIDs := make([]string, 0, len(opts.IDs))
	for _, q := range opts.IDs {
		if id, ok := decls[q]; ok {
			projectIDs = append(projectIDs, id)
			lockIDs = append(lockIDs, id)
			continue
		}
		projectIDs = append(projectIDs, q)
		lockIDs = append(lockIDs, q)
	}
	warn := []string{}
	for _, id := range opts.IDs {
		for _, item := range listed.Plugins {
			if item.ID == id && item.Declared != nil && item.Warning != "" {
				warn = append(warn, id+": "+item.Warning)
			}
		}
	}
	if err := a.Workspace.RemoveProjectPlugins(projectIDs); err != nil {
		return nil, err
	}
	if err := a.Locks.RemovePlugins(string(opts.Scope), lockIDs); err != nil {
		return nil, err
	}
	return &RemoveResult{Removed: projectIDs, Warnings: warn, Message: "removed plugins"}, nil
}

type InspectResult struct {
	Source  model.Source         `json:"source"`
	Skills  []model.CatalogSkill `json:"skills"`
	Message string               `json:"message,omitempty"`
}

func (a *App) Inspect(opts InspectOptions) (*InspectResult, error) {
	if a == nil || a.Workspace == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	rawSource := opts.Source.Locator
	if rawSource == "" {
		rawSource = opts.Source.CloneURL
	}
	parsed, err := a.Workspace.ParseSource(rawSource)
	if err != nil {
		return nil, err
	}
	if err := validateHostedOptions(parsed.Kind, AddOptions{}); err != nil {
		return nil, err
	}
	if err := applyRequestedVersion(parsed); err != nil {
		return nil, err
	}
	opened, err := a.Workspace.OpenSource(parsed.Source)
	if err != nil {
		return nil, err
	}
	opts.Source = opened
	if opts.Source.Path == "" {
		return nil, fmt.Errorf("local source path required")
	}
	skills, err := a.Workspace.BuildCatalog(opts.Scope, opts.Source)
	if err != nil {
		return nil, err
	}
	return &InspectResult{Source: opts.Source, Skills: skills}, nil
}

func (a *App) InspectPlugin(opts PluginInspectOptions) (*PluginSourceResult, error) {
	if a == nil || a.Workspace == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	_, opened, err := resolvePluginSource(a.Workspace, opts.Source)
	if err != nil {
		return nil, err
	}
	manifest, err := plugins.LoadManifest(opened.Path)
	if err != nil {
		return nil, err
	}
	meta := plugins.MetadataFromManifest(manifest)
	return &PluginSourceResult{Kind: PluginSourceKindPluginSource, Source: opened, Plugin: PluginInspectMetadata{ID: manifest.Plugin.ID, Runtime: manifest.Plugin.Runtime, Entry: manifest.Plugin.Entry, Name: meta.Name, Description: meta.Description, Revision: opened.RequestedVersion}}, nil
}

func (a *App) AddPlugin(opts AddPluginOptions) (*PluginSourceResult, error) {
	if a == nil || a.Workspace == nil || a.Locks == nil {
		return nil, fmt.Errorf("workspace and locks are required")
	}
	_, opened, err := resolvePluginSource(a.Workspace, opts.Source)
	if err != nil {
		return nil, err
	}
	manifest, err := plugins.LoadManifest(opened.Path)
	if err != nil {
		return nil, err
	}
	project, err := a.Workspace.LoadProjectConfig()
	if err != nil {
		return nil, err
	}
	if a.projectHasPluginID(project, manifest.Plugin.ID) && !opts.Force {
		return nil, fmt.Errorf("plugin %q already declared; use --force to replace", manifest.Plugin.ID)
	}
	declared := append([]model.ProjectPlugin(nil), project.Plugins...)
	declared = upsertProjectPlugin(declared, model.ProjectPlugin{ID: manifest.Plugin.ID, Source: nonEmpty(opts.Source.Locator, opts.Source.CloneURL, opened.Locator, opened.CloneURL), Ref: opts.Source.RequestedVersion})
	if err := a.Workspace.WriteProjectPlugins(declared); err != nil {
		return nil, err
	}
	if enablement, ok := project.Agents["opencode"]; ok {
		ids := append([]string(nil), enablement.Plugins...)
		if !containsString(ids, manifest.Plugin.ID) {
			ids = append(ids, manifest.Plugin.ID)
		}
		if err := a.Workspace.WriteProjectAgentEnablement("opencode", enablement.Skills, ids); err != nil {
			return nil, err
		}
	}
	if a.Locks != nil && a.Skills != nil {
		if _, err := a.Sync(opts.Scope, false); err != nil {
			return nil, err
		}
	}
	meta := plugins.MetadataFromManifest(manifest)
	return &PluginSourceResult{Kind: PluginSourceKindPluginSource, Source: opened, Plugin: PluginInspectMetadata{ID: manifest.Plugin.ID, Runtime: manifest.Plugin.Runtime, Entry: manifest.Plugin.Entry, Name: meta.Name, Description: meta.Description, Revision: opened.RequestedVersion}}, nil
}

func resolvePluginSource(w *workspace.Workspace, source model.Source) (*workspace.ParsedSource, model.Source, error) {
	raw := source.Locator
	if raw == "" {
		raw = source.CloneURL
	}
	parsed, err := w.ParseSource(raw)
	if err != nil {
		return nil, model.Source{}, err
	}
	if err := applyRequestedVersion(parsed); err != nil {
		return nil, model.Source{}, err
	}
	opened, err := w.OpenSource(parsed.Source)
	if err != nil {
		return nil, model.Source{}, err
	}
	if opened.Path == "" {
		return nil, model.Source{}, fmt.Errorf("local source path required")
	}
	return parsed, opened, nil
}

func upsertProjectPlugin(items []model.ProjectPlugin, next model.ProjectPlugin) []model.ProjectPlugin {
	for i, item := range items {
		if item.ID == next.ID {
			items[i] = next
			return items
		}
	}
	return append(items, next)
}

func nonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

type ApplyResult struct {
	Applied []skills.InstalledSkillResult `json:"applied,omitempty"`
}

func (a *App) Apply(opts ApplyOptions) (*ApplyResult, error) {
	if a == nil || a.Workspace == nil || a.Locks == nil || a.Skills == nil {
		return nil, fmt.Errorf("workspace, skills, and locks are required")
	}
	if err := agents.Validate(opts.Agent); err != nil {
		return nil, err
	}
	projectConfig, err := a.Workspace.LoadProjectConfig()
	if err != nil {
		return nil, err
	}
	items, err := a.Locks.List(string(opts.Scope))
	if err != nil {
		if os.IsNotExist(err) {
			return &ApplyResult{}, nil
		}
		return nil, err
	}
	declaredSkills := map[string]model.ProjectSkill{}
	declaredOrder := make([]string, 0, len(projectConfig.Skills))
	for _, skill := range projectConfig.Skills {
		declaredSkills[skill.ID] = skill
		declaredOrder = append(declaredOrder, skill.ID)
	}
	selectedIDs := append([]string(nil), opts.Skills...)
	if len(selectedIDs) == 0 {
		selectedIDs = declaredOrder
	}
	for _, id := range selectedIDs {
		if _, ok := declaredSkills[id]; !ok {
			return nil, fmt.Errorf("unknown declared skill ids: %s", id)
		}
	}
	selected, err := selectInstalledSkills(items, selectedIDs)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return &ApplyResult{}, nil
	}
	agentsList := effectiveAgents(opts.Agent, projectConfig.Agents)
	if len(opts.Agents) > 0 {
		agentsList = append([]string(nil), opts.Agents...)
	}
	if len(agentsList) == 0 {
		agentsList = []string{"opencode"}
	}
	for _, agent := range agentsList {
		enablement := projectConfig.Agents[agent]
		ids := append([]string(nil), enablement.Skills...)
		for _, skill := range selected {
			if !containsString(ids, skill.ID) {
				ids = append(ids, skill.ID)
			}
		}
		if err := a.Workspace.WriteProjectAgentEnablement(agent, ids, enablement.Plugins); err != nil {
			return nil, err
		}
	}
	exposureMethod := opts.ExposureMethod
	if exposureMethod == "" {
		exposureMethod = projectConfig.ExposureMethod
	}
	if exposureMethod == "" {
		exposureMethod = "symlink"
	}
	applied, err := a.Skills.Apply(skills.ApplyOptions{Scope: string(opts.Scope), Skills: selected, ExposureMethod: exposureMethod, Agents: agentsList, Agent: opts.Agent, Force: opts.Force})
	if err != nil {
		return nil, err
	}
	installed := make([]model.InstalledSkill, 0, len(applied.Applied))
	selectedByID := make(map[string]model.InstalledSkill, len(selected))
	for _, item := range selected {
		selectedByID[item.ID] = item
	}
	for _, item := range applied.Applied {
		existing := selectedByID[item.ID]
		install := existing.Install
		if install == nil {
			install = &model.SkillInstall{}
		}
		installed = append(installed, model.InstalledSkill{ID: item.ID, Source: existing.Source, Install: &model.SkillInstall{Mode: item.Mode, CanonicalPath: install.CanonicalPath}, Exposures: item.Exposures})
	}
	if err := a.Locks.UpsertInstalled(string(opts.Scope), installed); err != nil {
		return nil, err
	}
	return &ApplyResult{Applied: applied.Applied}, nil
}

func selectCatalogSkills(p ui.Prompter, catalog []model.CatalogSkill, opts AddOptions) ([]model.CatalogSkill, error) {
	if opts.All {
		return catalog, nil
	}
	if len(opts.Skills) > 0 {
		wanted := map[string]bool{}
		for _, id := range opts.Skills {
			wanted[id] = true
		}
		out := make([]model.CatalogSkill, 0, len(opts.Skills))
		for _, skill := range catalog {
			if wanted[skill.ID] {
				out = append(out, skill)
			}
			delete(wanted, skill.ID)
		}
		if len(wanted) > 0 {
			missing := make([]string, 0, len(wanted))
			for id := range wanted {
				missing = append(missing, id)
			}
			sort.Strings(missing)
			return nil, fmt.Errorf("unknown skill ids: %s", strings.Join(missing, ", "))
		}
		return out, nil
	}
	if len(catalog) == 1 {
		return catalog, nil
	}
	if len(catalog) > 1 && p != nil {
		indexes, err := p.MultiSelect("Select skills", catalogOptions(catalog), nil)
		if err != nil {
			return nil, err
		}
		out := make([]model.CatalogSkill, 0, len(indexes))
		for _, i := range indexes {
			if i >= 0 && i < len(catalog) {
				out = append(out, catalog[i])
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("no skills available")
}

func selectInstalledSkills(items []model.InstalledSkill, ids []string) ([]model.InstalledSkill, error) {
	if len(ids) == 0 {
		return items, nil
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	out := make([]model.InstalledSkill, 0, len(ids))
	for _, item := range items {
		if wanted[item.ID] {
			out = append(out, item)
			delete(wanted, item.ID)
		}
	}
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for id := range wanted {
			missing = append(missing, id)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("unknown skill ids: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

func filterSelectedSkills(items []model.InstalledSkill, picked map[int][]int) []model.InstalledSkill {
	if len(picked) == 0 {
		return nil
	}
	selected := map[int]bool{}
	for row := range picked {
		selected[row] = true
	}
	out := make([]model.InstalledSkill, 0, len(selected))
	for idx, item := range items {
		if selected[idx] {
			out = append(out, item)
		}
	}
	return out
}

func filterSelectedAgents(agents []string, picked map[int][]int) []string {
	if len(picked) == 0 {
		return nil
	}
	selected := map[int]bool{}
	for _, cols := range picked {
		for _, col := range cols {
			selected[col] = true
		}
	}
	out := make([]string, 0, len(selected))
	for idx, agent := range agents {
		if selected[idx] {
			out = append(out, agent)
		}
	}
	return out
}

func catalogOptions(catalog []model.CatalogSkill) []ui.Option {
	out := make([]ui.Option, 0, len(catalog))
	for _, skill := range catalog {
		out = append(out, ui.Option{Label: skill.ID})
	}
	return out
}

type AddOptions struct {
	Scope          config.Scope
	Source         model.Source
	All            bool
	Skills         []string
	ExposureMethod string
	Agent          string
	Force          bool
}

type RemoveOptions struct {
	Scope       config.Scope
	Skills      []string
	PruneUnused bool
	Force       bool
	Yes         bool
}

type ListOptions struct {
	Scope config.Scope
	JSON  bool
	All   bool
}

type InspectOptions struct {
	Scope  config.Scope
	Source model.Source
	JSON   bool
}

type ApplyOptions struct {
	Scope          config.Scope
	Skills         []string
	Agents         []string
	ExposureMethod string
	Agent          string
	Force          bool
	JSON           bool
}
