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
	Agents   []model.AgentResource         `json:"agents,omitempty"`
	Picked   []model.AgentResource         `json:"picked,omitempty"`
	Kind     string                        `json:"kind,omitempty"`
	Message  string                        `json:"message,omitempty"`
	Added    []skills.InstalledSkillResult `json:"added,omitempty"`
}

func (a *App) projectHasSkillID(projectConfig *model.ProjectConfig, id string) bool {
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

func (a *App) projectHasPluginID(projectConfig *model.ProjectConfig, id string) bool {
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
	originalSource, openedSource, err := a.resolveAddSource(opts)
	if err != nil {
		return nil, err
	}
	opts.Source = openedSource
	if opts.Source.Path == "" {
		return nil, fmt.Errorf("local source path required")
	}
	manifest, err := a.Workspace.LoadResourceManifest()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	kind, err := a.resolveAddKind(opts, manifest)
	if err != nil {
		return nil, err
	}
	result := &AddResult{Source: opts.Source, Kind: string(kind)}
	if kind == model.ResourceKindPlugin {
		return a.addPlugin(opts, result)
	}
	catalog, agents, err := a.loadCatalogAndAgents(opts)
	if err != nil {
		return nil, err
	}
	if kind == model.ResourceKindAgent {
		result.Agents = agents
		if len(agents) == 0 {
			return nil, fmt.Errorf("no valid agent resource found")
		}
		picked, err := selectAgentResources(a.Prompter, result.Agents, opts)
		if err != nil {
			return nil, err
		}
		result.Picked = picked
		if len(picked) == 0 {
			result.Message = "no resources selected"
			return result, nil
		}
		result.Message = "selected agents"
		return result, nil
	}
	if kind == model.ResourceKindResources && len(catalog) == 0 {
		return nil, fmt.Errorf("no valid skill resource found")
	}
	selected, err := selectCatalogSkills(a.Prompter, catalog, opts)
	if err != nil {
		return nil, err
	}
	if err := a.applySelectedSkillsToProjectConfig(projectConfig, selected, originalSource, opts.Force); err != nil {
		return nil, err
	}
	a.updateProjectRuntimeAutoApply(projectConfig, selected)
	if err := a.Workspace.WriteProjectConfig(*projectConfig); err != nil {
		return nil, err
	}
	result.Catalog = catalog
	result.Selected = selected
	if err := a.materializeSkills(result, opts, projectConfig, originalSource, selected); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) resolveAddSource(opts AddOptions) (model.Source, model.Source, error) {
	rawSource := opts.Source.Locator
	if rawSource == "" {
		rawSource = opts.Source.CloneURL
	}
	parsed, err := a.Workspace.ParseSource(rawSource)
	if err != nil {
		return model.Source{}, model.Source{}, err
	}
	if err := validateHostedOptions(parsed.Kind, opts); err != nil {
		return model.Source{}, model.Source{}, err
	}
	if err := applyRequestedVersion(parsed); err != nil {
		return model.Source{}, model.Source{}, err
	}
	opened, err := a.Workspace.OpenSource(parsed.Source)
	if err != nil {
		return model.Source{}, model.Source{}, err
	}
	return parsed.Source, opened, nil
}

func (a *App) resolveAddKind(opts AddOptions, manifest *model.ResourceManifest) (model.ResourceKind, error) {
	kind := opts.ResourceKind
	if kind == "" {
		if manifest != nil {
			kind = manifest.Kind
		} else {
			kind = model.ResourceKindResources
		}
	}
	if kind == model.ResourceKindPlugin && manifest == nil {
		return "", fmt.Errorf("no valid plugin resource found")
	}
	return kind, nil
}

func (a *App) addPlugin(opts AddOptions, result *AddResult) (*AddResult, error) {
	if result == nil {
		result = &AddResult{Source: opts.Source}
	}
	pluginResult, err := a.AddPlugin(AddPluginOptions{Scope: opts.Scope, Source: opts.Source, Force: opts.Force})
	if err != nil {
		return nil, err
	}
	if pluginResult != nil {
		result.Source = pluginResult.Source
	}
	result.Message = "selected plugin package"
	return result, nil
}

func (a *App) loadCatalogAndAgents(opts AddOptions) ([]model.CatalogSkill, []model.AgentResource, error) {
	catalog, err := a.Workspace.BuildCatalog(opts.Scope, opts.Source)
	if err != nil {
		return nil, nil, err
	}
	agentsRaw, err := a.Workspace.DiscoverAgentResources()
	if err != nil {
		return nil, nil, err
	}
	agents := make([]model.AgentResource, 0, len(agentsRaw))
	for _, ar := range agentsRaw {
		agents = append(agents, model.AgentResource{ID: ar.ID, Path: ar.Path, Format: ar.Format, Body: ar.Body})
	}
	return catalog, agents, nil
}

func (a *App) applySelectedSkillsToProjectConfig(projectConfig *model.ProjectConfig, selected []model.CatalogSkill, originalSource model.Source, force bool) error {
	if projectConfig == nil {
		projectConfig = &model.ProjectConfig{}
	}
	for _, skill := range selected {
		if a.projectHasSkillID(projectConfig, skill.ID) && !force {
			return fmt.Errorf("skill %q already declared; use --force to replace", skill.ID)
		}
		projectConfig.Skills = model.UpsertProjectSkill(append([]model.ProjectSkill(nil), projectConfig.Skills...), model.ProjectSkill{ID: skill.ID, Source: nonEmpty(originalSource.Locator, originalSource.CloneURL, originalSource.Path), Ref: originalSource.RequestedVersion})
	}
	return nil
}

func (a *App) updateProjectRuntimeAutoApply(projectConfig *model.ProjectConfig, selected []model.CatalogSkill) {
	if !projectConfig.AutoApply || len(selected) == 0 {
		return
	}
	for runtime, enablement := range projectConfig.Runtimes {
		ids := append([]string(nil), enablement.Skills...)
		for _, skill := range selected {
			if !containsString(ids, skill.ID) {
				ids = append(ids, skill.ID)
			}
		}
		projectConfig.Runtimes[runtime] = model.ProjectRuntimeEnablement{Skills: ids, Plugins: append([]string(nil), enablement.Plugins...), Agents: append([]string(nil), enablement.Agents...)}
	}
}

func (a *App) materializeSkills(result *AddResult, opts AddOptions, projectConfig *model.ProjectConfig, originalSource model.Source, selected []model.CatalogSkill) error {
	if len(selected) == 0 {
		result.Message = "no resources selected"
		return nil
	}
	if a.Skills == nil {
		result.Message = "selected skills"
		return nil
	}
	agents := effectiveAgents(opts.Agent, projectConfig.Runtimes)
	autoApply := projectConfig.AutoApply
	added, err := a.Skills.Add(skills.AddOptions{Scope: string(opts.Scope), SourceRoot: opts.Source.Path, All: opts.All, Skills: opts.Skills, Selected: selected, ExposureMethod: effectiveExposureMethod(opts.ExposureMethod, projectConfig.ExposureMethod), Agents: agents, Force: opts.Force, AutoApply: &autoApply})
	if err != nil {
		return err
	}
	result.Added = added.Installed
	if a.Locks != nil {
		installed := make([]model.InstalledSkill, 0, len(added.Installed))
		for _, item := range added.Installed {
			installed = append(installed, model.InstalledSkill{ID: item.ID, Source: mergeSkillSource(&originalSource, item.Source), Install: &model.SkillInstall{Mode: item.Mode, CanonicalPath: item.Target}, Exposures: item.Exposures})
		}
		if err := a.Locks.UpsertInstalled(string(opts.Scope), installed); err != nil {
			return err
		}
	}
	if len(added.Installed) > 0 {
		result.Message = "selected and materialized skills"
	}
	return nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func validateAddOptions(opts AddOptions) error {
	if err := agents.Validate(opts.Agent); err != nil {
		return err
	}
	if opts.ExposureMethod != "" && !opts.ExposureMethod.IsValid() {
		return fmt.Errorf("unsupported exposure method %q", opts.ExposureMethod)
	}
	return nil
}

func effectiveAgents(explicit string, configured map[string]model.ProjectRuntimeEnablement) []string {
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
	Scope   string                 `json:"scope"`
	Skills  []model.InstalledSkill `json:"skills,omitempty"`
	Plugins []PluginListItem       `json:"plugins,omitempty"`
	Agents  []AgentListItem        `json:"agents,omitempty"`
}

type AgentListItem struct {
	ID      string   `json:"id"`
	Skills  []string `json:"skills,omitempty"`
	Plugins []string `json:"plugins,omitempty"`
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

func skillReconcileInputs(project *model.ProjectConfig, locks *lock.Store, scope string) (*SkillReconcileInputs, error) {
	inputs := &SkillReconcileInputs{}
	if project != nil {
		inputs.Declared = append([]model.ProjectSkill(nil), project.Skills...)
		inputs.Enabled = model.CloneRuntimeEnablement(project.Runtimes)
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

func pluginReconcileInputs(project *model.ProjectConfig, locks *lock.Store, scope string) (*PluginReconcileInputs, error) {
	inputs := &PluginReconcileInputs{}
	if project != nil {
		inputs.Declared = append([]model.ProjectPlugin(nil), project.Plugins...)
		inputs.Enabled = model.CloneRuntimeEnablement(project.Runtimes)
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
	declared := model.IndexProjectSkills(inputs.Declared)
	materialized := model.IndexInstalledSkills(inputs.Materialized)
	ids := model.NewStringSet()
	for id := range declared {
		ids.Add(id)
	}
	for id := range materialized {
		ids.Add(id)
	}
	all := ids.Keys()
	// sort.Strings already applied by model.StringSet.Keys
	actions := make([]SkillReconcileAction, 0, len(all))
	for _, id := range all {
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
	declared := model.IndexProjectPlugins(inputs.Declared)
	materialized := model.IndexLockPlugins(inputs.Materialized)
	ids := model.NewStringSet()
	for id := range declared {
		ids.Add(id)
	}
	for id := range materialized {
		ids.Add(id)
	}
	all := ids.Keys()
	// sort.Strings already applied by model.StringSet.Keys
	actions := make([]PluginReconcileAction, 0, len(all))
	for _, id := range all {
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
	if locked {
		return a.syncLocked(scope)
	}
	project, err := a.Workspace.LoadProjectConfigForScope(scope)
	if err != nil {
		return nil, err
	}
	return a.syncUnlocked(scope, project)
}

func (a *App) syncUnlocked(scope config.Scope, project *model.ProjectConfig) (*SyncResult, error) {
	result := &SyncResult{}
	declaredSkills := append([]model.ProjectSkill(nil), project.Skills...)
	declaredPlugins := append([]model.ProjectPlugin(nil), project.Plugins...)
	declaredAgents := make([]model.LockAgent, 0, len(project.Agents))
	for _, agent := range project.Agents {
		declaredAgents = append(declaredAgents, model.LockAgent{ID: agent.ID, Path: agent.Path})
	}
	openedSkills := map[string]model.Source{}
	for _, skill := range declaredSkills {
		opened, err := a.Workspace.OpenSource(model.Source{Locator: skill.Source})
		if err != nil {
			return nil, err
		}
		opened.Path = resolveScopePath(a.Workspace.Root, scope, opened.Path)
		openedSkills[skill.ID] = opened
	}
	openedPlugins := map[string]model.Source{}
	for _, plugin := range declaredPlugins {
		opened, err := a.Workspace.OpenSource(model.Source{Locator: plugin.Source, RequestedVersion: plugin.Ref})
		if err != nil {
			return nil, err
		}
		opened.Path = resolveScopePath(a.Workspace.Root, scope, opened.Path)
		if _, err := plugins.LoadManifest(opened.Path); err != nil {
			return nil, err
		}
		openedPlugins[plugin.ID] = opened
	}
	installedSkills := make([]model.InstalledSkill, 0, len(declaredSkills))
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
		added, err := a.Skills.Add(skills.AddOptions{Scope: string(scope), SourceRoot: opened.Path, Selected: selected, ExposureMethod: effectiveExposureMethod(project.ExposureMethod), Agents: agentsFromEnablement(project.Runtimes, skill.ID), AutoApply: &project.AutoApply, Force: true})
		if err != nil {
			return nil, err
		}
		for _, item := range added.Installed {
			installedSkills = append(installedSkills, model.InstalledSkill{ID: item.ID, Source: mergeSkillSource(&model.Source{Locator: skill.Source}, item.Source), Install: &model.SkillInstall{Mode: item.Mode, CanonicalPath: item.Target}, Exposures: item.Exposures})
		}
		result.SkillMessages = append(result.SkillMessages, fmt.Sprintf("restored skill %s", skill.ID))
	}
	lockPlugins := make([]model.LockPlugin, 0, len(declaredPlugins))
	for _, plugin := range declaredPlugins {
		opened := openedPlugins[plugin.ID]
		mat, err := plugins.Materialize(plugins.MaterializeOptions{WorkspaceRoot: a.Workspace.Root, Scope: string(scope), SourceRoot: opened.Path, ID: plugin.ID, Force: true})
		if err != nil {
			return nil, err
		}
		lockPlugins = append(lockPlugins, model.LockPlugin{ID: plugin.ID, Declared: model.LockDeclared{Source: plugin.Source, Ref: plugin.Ref}, Resolved: model.LockResolved{Source: plugin.Source, Ref: plugin.Ref, Revision: opened.RequestedVersion}, Materialized: model.LockMaterialized{Path: mat.ManagedPath}, Projected: model.LockPluginProjected{Path: mat.RuntimePath}})
	}
	lockSnapshot := buildSyncLockSnapshot(string(scope), installedSkills, lockPlugins, declaredAgents)
	lockSnapshot.ExposureMethod = effectiveExposureMethod(project.ExposureMethod)
	lockSnapshot.AutoApply = project.AutoApply
	lockSnapshot.Runtimes = map[string]model.LockRuntimeEntry{}
	for id, runtime := range project.Runtimes {
		lockSnapshot.Runtimes[id] = model.LockRuntimeEntry{Skills: append([]string(nil), runtime.Skills...), Plugins: append([]string(nil), runtime.Plugins...), Agents: append([]string(nil), runtime.Agents...)}
	}
	if err := a.Locks.Write(string(scope), lockSnapshot); err != nil {
		return nil, err
	}
	if len(result.SkillMessages) == 0 {
		result.SkillMessages = append(result.SkillMessages, "skills already in sync")
	}
	if len(result.PluginMessages) == 0 {
		result.PluginMessages = append(result.PluginMessages, "plugins already in sync")
	}
	return result, nil
}

func buildSyncLockSnapshot(scope string, skills []model.InstalledSkill, plugins []model.LockPlugin, agents []model.LockAgent) *model.Lockfile {
	lf := &model.Lockfile{Version: 1, Scope: scope}
	for _, skill := range skills {
		lf.Skills = append(lf.Skills, model.LockSkill{ID: skill.ID, Declared: model.LockDeclared{Source: nonEmpty(skill.Source.Locator, skill.Source.CloneURL, skill.Source.Path), Ref: skill.Source.RequestedVersion}, Resolved: model.LockResolved{Source: nonEmpty(skill.Source.Locator, skill.Source.CloneURL, skill.Source.Path), Ref: skill.Source.RequestedVersion, Revision: skill.Source.RequestedVersion}, Materialized: model.LockMaterialized{Path: skill.Install.CanonicalPath}, Projected: model.LockProjected{Mode: skill.Install.Mode, Exposures: skill.Exposures}})
	}
	lf.Plugins = append(lf.Plugins, plugins...)
	lf.Agents = append(lf.Agents, agents...)
	return lf
}

func enabledRuntimeIDs(runtimes map[string]model.ProjectRuntimeEnablement) ([]string, []string) {
	plugIDs := model.NewStringSet()
	agentIDs := model.NewStringSet()
	for _, runtime := range runtimes {
		for _, id := range runtime.Plugins {
			plugIDs.Add(id)
		}
		for _, id := range runtime.Agents {
			agentIDs.Add(id)
		}
	}
	plugins := plugIDs.Keys()
	agents := agentIDs.Keys()
	return plugins, agents
}

func resolveScopePath(root string, scope config.Scope, p string) string {
	if scope == config.ScopeGlobal {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = "."
		}
		return resolveWorkspaceSourcePath(filepath.Join(home, ".spick"), p)
	}
	return resolveWorkspaceSourcePath(root, p)
}

func (a *App) syncLocked(scope config.Scope) (*SyncResult, error) {
	lf, err := a.Locks.Read(string(scope))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("locked sync requires an existing lockfile")
		}
		return nil, err
	}
	result := &SyncResult{}
	warnedSkills := map[string][]string{}
	warnedPlugins := map[string][]string{}
	runtimeMembers := lf.Runtimes
	for _, sk := range lf.Skills {
		if sk.Materialized.Path == "" {
			return nil, fmt.Errorf("locked sync requires installed materialization for skill %s", sk.ID)
		}
		managedSkill := resolveScopePath(a.Workspace.Root, scope, sk.Materialized.Path)
		if _, err := os.Stat(managedSkill); os.IsNotExist(err) {
			restoreSource := model.Source{Locator: sk.Resolved.Source, RequestedVersion: sk.Resolved.Ref}
			if restoreSource.Locator == "" {
				restoreSource = model.Source{Locator: sk.Declared.Source, RequestedVersion: sk.Declared.Ref}
			}
			opened, err := a.Workspace.OpenSource(restoreSource)
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for skill %s: %w", sk.ID, err)
			}
			opened.Path = resolveScopePath(a.Workspace.Root, scope, opened.Path)
			catalog, err := a.Workspace.BuildCatalog(scope, opened)
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for skill %s: %w", sk.ID, err)
			}
			selected, err := selectCatalogSkills(a.Prompter, catalog, AddOptions{Scope: scope, Source: opened, Skills: []string{sk.ID}, All: false})
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for skill %s: %w", sk.ID, err)
			}
			exposureMethod := effectiveExposureMethod(lf.ExposureMethod)
			added, err := a.Skills.Add(skills.AddOptions{Scope: string(scope), SourceRoot: opened.Path, Selected: selected, ExposureMethod: exposureMethod, Agents: agentsFromLockRuntimeMembership(runtimeMembers, sk.ID), Force: true})
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for skill %s: %w", sk.ID, err)
			}
			if len(added.Installed) == 0 || added.Installed[0].Target == "" {
				return nil, fmt.Errorf("locked sync snapshot insufficiency for skill %s", sk.ID)
			}
			managedSkill = added.Installed[0].Target
			if isUnpinnedSource(restoreSource.Locator, restoreSource.RequestedVersion) {
				warnedSkills[restoreSource.Locator] = append(warnedSkills[restoreSource.Locator], sk.ID)
			}
		}
		for _, ex := range skillExposurePaths(scope, lf, sk.ID) {
			path := resolveScopePath(a.Workspace.Root, scope, ex.Path)
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
		if pl.Materialized.Path == "" {
			return nil, fmt.Errorf("locked sync snapshot insufficiency for plugin %s", pl.ID)
		}
		managedPlugin := resolveScopePath(a.Workspace.Root, scope, pl.Materialized.Path)
		if _, err := os.Stat(managedPlugin); os.IsNotExist(err) {
			restoreSource := model.Source{Locator: pl.Resolved.Source, RequestedVersion: pl.Resolved.Ref}
			if restoreSource.Locator == "" {
				restoreSource = model.Source{Locator: pl.Declared.Source, RequestedVersion: pl.Declared.Ref}
			}
			opened, err := a.Workspace.OpenSource(restoreSource)
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for plugin %s: %w", pl.ID, err)
			}
			opened.Path = resolveScopePath(a.Workspace.Root, scope, opened.Path)
			mat, err := plugins.Materialize(plugins.MaterializeOptions{WorkspaceRoot: a.Workspace.Root, Scope: string(scope), SourceRoot: opened.Path, ID: pl.ID, Force: true})
			if err != nil {
				return nil, fmt.Errorf("locked sync source fetch failed for plugin %s: %w", pl.ID, err)
			}
			managedPlugin = mat.ManagedPath
			if isUnpinnedSource(restoreSource.Locator, restoreSource.RequestedVersion) {
				warnedPlugins[restoreSource.Locator] = append(warnedPlugins[restoreSource.Locator], pl.ID)
			}
		}
		proj := resolveScopePath(a.Workspace.Root, scope, pl.Materialized.Path)
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

func skillExposurePaths(scope config.Scope, lf *model.Lockfile, skillID string) []model.Exposure {
	paths := []model.Exposure{}
	seen := model.NewStringSet()
	for runtimeID, entry := range lf.Runtimes {
		if !containsString(entry.Skills, skillID) {
			continue
		}
		root, err := agents.ExposureRoot(scope, runtimeID)
		if err != nil {
			continue
		}
		p := filepath.Join(root, "skills", skillID)
		if seen.Has(p) {
			continue
		}
		seen.Add(p)
		paths = append(paths, model.Exposure{Agent: runtimeID, Path: p})
	}
	return paths
}

func agentsFromLockRuntimeMembership(runtimes map[string]model.LockRuntimeEntry, skillID string) []string {
	out := []string{}
	for agent, entry := range runtimes {
		if containsString(entry.Skills, skillID) {
			out = append(out, agent)
		}
	}
	sort.Strings(out)
	return out
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
func agentsFromEnablement(enabled map[string]model.ProjectRuntimeEnablement, id string) []string {
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

// validateLockedSyncInputs removed: locked sync is snapshot-authoritative.

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
	result := &ListResult{Scope: string(opts.Scope), Skills: items}
	project, err := a.Workspace.LoadProjectConfig()
	if err != nil {
		return nil, err
	}
	if project != nil {
		pluginResult, err := a.ListPlugins(opts)
		if err != nil {
			return nil, err
		}
		result.Plugins = append(result.Plugins, pluginResult.Plugins...)
		result.Agents = append(result.Agents, listProjectAgents(project)...)
	}
	return result, nil
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

type RemoveSelectionMode string

const (
	RemoveSelectionModeAuto   RemoveSelectionMode = ""
	RemoveSelectionModeSkill  RemoveSelectionMode = "skill"
	RemoveSelectionModePlugin RemoveSelectionMode = "plugin"
)

type RemoveSelectionOptions struct {
	Scope config.Scope
	Mode  RemoveSelectionMode
}

type RemoveSelection struct {
	Skills  []string
	Plugins []string
}

func (a *App) SelectRemovals(opts RemoveSelectionOptions) (*RemoveSelection, error) {
	if a == nil {
		return nil, fmt.Errorf("app is required")
	}
	listed, err := a.List(ListOptions{Scope: opts.Scope})
	if err != nil {
		return nil, err
	}
	mode := string(opts.Mode)
	if mode != string(RemoveSelectionModeAuto) && mode != string(RemoveSelectionModeSkill) && mode != string(RemoveSelectionModePlugin) {
		return nil, fmt.Errorf("invalid remove mode: %q", opts.Mode)
	}
	if mode == string(RemoveSelectionModeSkill) {
		skills, err := selectRemoveSkillIDs(listed.Skills, a.Prompter)
		if err != nil {
			return nil, err
		}
		return &RemoveSelection{Skills: skills}, nil
	}
	if mode == string(RemoveSelectionModePlugin) {
		plugins, err := selectRemovePluginIDs(listed.Plugins, a.Prompter)
		if err != nil {
			return nil, err
		}
		return &RemoveSelection{Plugins: plugins}, nil
	}
	if len(listed.Skills) == 0 && len(listed.Plugins) == 0 {
		return nil, fmt.Errorf("no removable resource found")
	}
	if len(listed.Plugins) == 0 {
		skills, err := selectRemoveSkillIDs(listed.Skills, a.Prompter)
		if err != nil {
			return nil, err
		}
		return &RemoveSelection{Skills: skills}, nil
	}
	if len(listed.Skills) == 0 {
		plugins, err := selectRemovePluginIDs(listed.Plugins, a.Prompter)
		if err != nil {
			return nil, err
		}
		return &RemoveSelection{Plugins: plugins}, nil
	}
	options := make([]ui.Option, 0, len(listed.Skills)+len(listed.Plugins))
	labels := make([]string, 0, len(options))
	for _, skill := range listed.Skills {
		options = append(options, ui.Option{Label: "skill: " + skill.ID})
		labels = append(labels, "skill:"+skill.ID)
	}
	for _, plugin := range listed.Plugins {
		options = append(options, ui.Option{Label: "plugin: " + plugin.ID})
		labels = append(labels, "plugin:"+plugin.ID)
	}
	if a.Prompter == nil {
		return nil, fmt.Errorf("no removable resource found")
	}
	indexes, err := a.Prompter.MultiSelect("Remove resources", options, nil)
	if err != nil {
		return nil, err
	}
	var selectedSkills, selectedPlugins []string
	for _, idx := range indexes {
		if idx < 0 || idx >= len(labels) {
			continue
		}
		if strings.HasPrefix(labels[idx], "skill:") {
			selectedSkills = append(selectedSkills, strings.TrimPrefix(labels[idx], "skill:"))
		}
		if strings.HasPrefix(labels[idx], "plugin:") {
			selectedPlugins = append(selectedPlugins, strings.TrimPrefix(labels[idx], "plugin:"))
		}
	}
	return &RemoveSelection{Skills: selectedSkills, Plugins: selectedPlugins}, nil
}

func selectRemoveSkillIDs(items []model.InstalledSkill, p ui.Prompter) ([]string, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no valid skill resource found")
	}
	if len(items) == 1 || p == nil {
		return []string{items[0].ID}, nil
	}
	options := make([]ui.Option, 0, len(items))
	for _, it := range items {
		options = append(options, ui.Option{Label: it.ID})
	}
	indexes, err := p.MultiSelect("Remove skills", options, nil)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		if idx >= 0 && idx < len(items) {
			ids = append(ids, items[idx].ID)
		}
	}
	return ids, nil
}

func selectRemovePluginIDs(items []PluginListItem, p ui.Prompter) ([]string, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no valid plugin resource found")
	}
	if len(items) == 1 || p == nil {
		return []string{items[0].ID}, nil
	}
	options := make([]ui.Option, 0, len(items))
	for _, it := range items {
		options = append(options, ui.Option{Label: it.ID})
	}
	indexes, err := p.MultiSelect("Remove plugins", options, nil)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		if idx >= 0 && idx < len(items) {
			ids = append(ids, items[idx].ID)
		}
	}
	return ids, nil
}

func (a *App) Remove(opts RemoveOptions) (*RemoveResult, error) {
	if a == nil || a.Locks == nil {
		return &RemoveResult{Message: "no lock store"}, nil
	}
	if a.Workspace != nil {
		projectConfig, err := a.Workspace.LoadProjectConfig()
		if err != nil {
			return nil, err
		}
		projectConfig.Skills = model.WithoutProjectSkillIDs(projectConfig.Skills, opts.Skills)
		projectConfig.Runtimes = model.RemoveIDsFromRuntimeEnablements(projectConfig.Runtimes, opts.Skills, nil)
		if err := a.Workspace.WriteProjectConfig(*projectConfig); err != nil {
			return nil, err
		}
	}
	if err := a.Locks.RemoveFiles(string(opts.Scope), opts.Skills, opts.PruneUnused); err != nil {
		return nil, err
	}
	return &RemoveResult{Removed: opts.Skills, Purged: opts.Skills, Message: "removed skills from canonical storage and exposures"}, nil
}

func listProjectPlugins(project *model.ProjectConfig) []PluginListItem {
	if project == nil {
		return nil
	}
	items := make([]PluginListItem, 0, len(project.Plugins))
	for _, decl := range project.Plugins {
		items = append(items, PluginListItem{ID: decl.ID, Declared: &decl, State: "declared"})
	}
	return items
}

func listProjectAgents(project *model.ProjectConfig) []AgentListItem {
	if project == nil || len(project.Agents) == 0 {
		return nil
	}
	out := make([]AgentListItem, 0, len(project.Agents))
	for _, agent := range project.Agents {
		out = append(out, AgentListItem{ID: agent.ID})
	}
	return out
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
	project, err := a.Workspace.LoadProjectConfig()
	if err != nil {
		return nil, err
	}
	project.Plugins = model.WithoutProjectPluginIDs(project.Plugins, projectIDs)
	project.Runtimes = model.RemoveIDsFromRuntimeEnablements(project.Runtimes, nil, projectIDs)
	if err := a.Workspace.WriteProjectConfig(*project); err != nil {
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
	Kind    string               `json:"kind,omitempty"`
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
	loader := &workspace.Loader{Root: opts.Source.Path}
	if _, err := loader.LoadResourceCatalogManifest(); err == nil {
		skills, err := loader.LoadCatalog()
		if err != nil {
			return nil, err
		}
		return &InspectResult{Source: opts.Source, Skills: skills, Kind: "manifest"}, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	skills, err := loader.DiscoverCatalog()
	if err != nil {
		return nil, fmt.Errorf("no spick.res.yaml found and no exported resources discovered; plugin repos require an explicit spick.res.yaml with kind: plugin")
	}
	return &InspectResult{Source: opts.Source, Skills: skills, Kind: "resources"}, nil
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
	project.Plugins = model.UpsertProjectPlugin(append([]model.ProjectPlugin(nil), project.Plugins...), model.ProjectPlugin{ID: manifest.Plugin.ID, Source: nonEmpty(opened.Locator, opened.CloneURL, opts.Source.Locator, opts.Source.CloneURL), Ref: opened.RequestedVersion})
	if err := a.Workspace.WriteProjectConfig(*project); err != nil {
		return nil, err
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

func nonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func effectiveExposureMethod(methods ...model.ExposureMethod) model.ExposureMethod {
	for _, method := range methods {
		if method != "" {
			return method
		}
	}
	return model.DefaultExposureMethod
}

type ApplyResult struct {
	Applied []skills.InstalledSkillResult `json:"applied,omitempty"`
}

type ApplyOptions struct {
	Scope     config.Scope
	Skills    []string
	Plugins   []string
	Agents    []string
	Agent     string
	Global    bool
	Runtime   string
	Skill     bool
	Plugin    bool
	AgentMode bool
	Force     bool
	JSON      bool
}

func (a *App) Apply(opts ApplyOptions) (*ApplyResult, error) {
	if a == nil || a.Workspace == nil || a.Locks == nil || a.Skills == nil {
		return nil, fmt.Errorf("workspace, skills, and locks are required")
	}
	if err := agents.Validate(opts.Agent); err != nil {
		return nil, err
	}
	selectedScope := opts.Scope
	if opts.Global {
		selectedScope = config.ScopeGlobal
	}
	if selectedScope == "" {
		selectedScope = config.ScopeProject
	}
	projectConfig, err := a.Workspace.LoadProjectConfigForScope(selectedScope)
	if err != nil {
		return nil, err
	}
	if len(opts.Agents) == 0 && opts.Agent != "" {
		opts.Agents = []string{opts.Agent}
	}
	declaredSkillIDs := model.NewStringSet()
	for _, skill := range projectConfig.Skills {
		declaredSkillIDs.Add(skill.ID)
	}
	declaredPluginIDs := model.NewStringSet()
	for _, plugin := range projectConfig.Plugins {
		declaredPluginIDs.Add(plugin.ID)
	}
	declaredAgentIDs := model.NewStringSet()
	for _, agent := range projectConfig.Agents {
		declaredAgentIDs.Add(agent.ID)
	}
	if len(opts.Skills) > 0 {
		for _, id := range opts.Skills {
			if !declaredSkillIDs.Has(id) {
				return nil, fmt.Errorf("unknown declared skill ids: %s", id)
			}
		}
	}
	if len(opts.Plugins) > 0 {
		for _, id := range opts.Plugins {
			if !declaredPluginIDs.Has(id) {
				return nil, fmt.Errorf("unknown declared plugin ids: %s", id)
			}
		}
	}
	if opts.Runtime != "" {
		if _, ok := projectConfig.Runtimes[opts.Runtime]; !ok {
			return nil, fmt.Errorf("unknown runtime ids: %s", opts.Runtime)
		}
	}
	if len(opts.Agents) > 0 {
		for _, id := range opts.Agents {
			if !declaredAgentIDs.Has(id) {
				return nil, fmt.Errorf("apply --agent may only target declared agents: %s", id)
			}
		}
	}
	runtimeID, err := resolveApplyRuntime(a.Prompter, projectConfig, opts.Runtime)
	if err != nil {
		return nil, err
	}
	if err := a.writeApplyDesiredState(selectedScope, projectConfig, runtimeID, opts); err != nil {
		return nil, err
	}
	if _, err := a.Sync(selectedScope, false); err != nil {
		return nil, err
	}
	return &ApplyResult{}, nil
}

func resolveApplyRuntime(p ui.Prompter, projectConfig *model.ProjectConfig, explicit string) (string, error) {
	if explicit != "" {
		if projectConfig == nil || projectConfig.Runtimes == nil {
			return "", fmt.Errorf("unknown runtime ids: %s", explicit)
		}
		if _, ok := projectConfig.Runtimes[explicit]; !ok {
			return "", fmt.Errorf("unknown runtime ids: %s", explicit)
		}
		return explicit, nil
	}
	if projectConfig == nil || len(projectConfig.Runtimes) == 0 {
		return "", nil
	}
	if len(projectConfig.Runtimes) == 1 {
		for runtime := range projectConfig.Runtimes {
			return runtime, nil
		}
	}
	if p == nil {
		return "", fmt.Errorf("runtime selection required")
	}
	ids := make([]string, 0, len(projectConfig.Runtimes))
	for runtime := range projectConfig.Runtimes {
		ids = append(ids, runtime)
	}
	sort.Strings(ids)
	idx, err := p.Select("Select runtime", func() []ui.Option {
		out := make([]ui.Option, 0, len(ids))
		for _, id := range ids {
			out = append(out, ui.Option{Label: id})
		}
		return out
	}(), 0)
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(ids) {
		return "", fmt.Errorf("invalid runtime selection")
	}
	return ids[idx], nil
}

func (a *App) writeApplyDesiredState(scope config.Scope, projectConfig *model.ProjectConfig, runtimeID string, opts ApplyOptions) error {
	if projectConfig.Runtimes == nil {
		projectConfig.Runtimes = model.CloneRuntimeEnablement(nil)
	}
	if runtimeID == "" {
		return a.Workspace.WriteProjectConfigForScope(scope, *projectConfig)
	}
	enablement := projectConfig.Runtimes[runtimeID]
	if !opts.Skill && len(opts.Skills) == 0 && a.Prompter != nil {
		declared := make([]ui.Option, 0, len(projectConfig.Skills))
		defaults := []int{}
		for i, sk := range projectConfig.Skills {
			declared = append(declared, ui.Option{Label: sk.ID})
			if containsString(enablement.Skills, sk.ID) {
				defaults = append(defaults, i)
			}
		}
		idxs, err := a.Prompter.MultiSelect("Select runtime skills", declared, defaults)
		if err != nil {
			return err
		}
		selected := make([]string, 0, len(idxs))
		for _, idx := range idxs {
			if idx >= 0 && idx < len(projectConfig.Skills) {
				selected = append(selected, projectConfig.Skills[idx].ID)
			}
		}
		enablement.Skills = selected
	} else if opts.Skill || len(opts.Skills) > 0 {
		enablement.Skills = append([]string(nil), opts.Skills...)
	}
	if !opts.Plugin && len(opts.Plugins) == 0 && a.Prompter != nil {
		declared := make([]ui.Option, 0, len(projectConfig.Plugins))
		defaults := []int{}
		for i, pl := range projectConfig.Plugins {
			declared = append(declared, ui.Option{Label: pl.ID})
			if containsString(enablement.Plugins, pl.ID) {
				defaults = append(defaults, i)
			}
		}
		idxs, err := a.Prompter.MultiSelect("Select runtime plugins", declared, defaults)
		if err != nil {
			return err
		}
		selected := make([]string, 0, len(idxs))
		for _, idx := range idxs {
			if idx >= 0 && idx < len(projectConfig.Plugins) {
				selected = append(selected, projectConfig.Plugins[idx].ID)
			}
		}
		enablement.Plugins = selected
	} else if opts.Plugin || len(opts.Plugins) > 0 {
		enablement.Plugins = append([]string(nil), opts.Plugins...)
	}
	if !opts.AgentMode && len(opts.Agents) == 0 && a.Prompter != nil {
		declared := make([]ui.Option, 0, len(projectConfig.Agents))
		defaults := []int{}
		for i, ag := range projectConfig.Agents {
			declared = append(declared, ui.Option{Label: ag.ID})
			if containsString(enablement.Agents, ag.ID) {
				defaults = append(defaults, i)
			}
		}
		idxs, err := a.Prompter.MultiSelect("Select runtime agents", declared, defaults)
		if err != nil {
			return err
		}
		selected := make([]string, 0, len(idxs))
		for _, idx := range idxs {
			if idx >= 0 && idx < len(projectConfig.Agents) {
				selected = append(selected, projectConfig.Agents[idx].ID)
			}
		}
		enablement.Agents = selected
	} else if opts.AgentMode || len(opts.Agents) > 0 {
		enablement.Agents = append([]string(nil), opts.Agents...)
	}
	projectConfig.Runtimes[runtimeID] = enablement
	return a.Workspace.WriteProjectConfigForScope(scope, *projectConfig)
}

func selectCatalogSkills(p ui.Prompter, catalog []model.CatalogSkill, opts AddOptions) ([]model.CatalogSkill, error) {
	if len(catalog) == 0 {
		return nil, fmt.Errorf("no valid skill resource found")
	}
	if opts.All {
		return catalog, nil
	}
	if len(opts.Skills) > 0 {
		wanted := model.NewStringSet()
		for _, id := range opts.Skills {
			wanted.Add(id)
		}
		out := make([]model.CatalogSkill, 0, len(opts.Skills))
		for _, skill := range catalog {
			if wanted.Has(skill.ID) {
				out = append(out, skill)
			}
			wanted.Delete(skill.ID)
		}
		if wanted.Len() > 0 {
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
	return nil, fmt.Errorf("no valid skill resource found")
}

func selectAgentResources(p ui.Prompter, agents []model.AgentResource, opts AddOptions) ([]model.AgentResource, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("no valid agent resource found")
	}
	if len(agents) == 1 {
		return agents, nil
	}
	if p != nil {
		indexes, err := p.MultiSelect("Select agents", agentOptions(agents), nil)
		if err != nil {
			return nil, err
		}
		out := make([]model.AgentResource, 0, len(indexes))
		for _, i := range indexes {
			if i >= 0 && i < len(agents) {
				out = append(out, agents[i])
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("no valid agent resource found")
}

func agentOptions(agents []model.AgentResource) []ui.Option {
	out := make([]ui.Option, 0, len(agents))
	for _, agent := range agents {
		out = append(out, ui.Option{Label: agent.ID})
	}
	return out
}

func selectInstalledSkills(items []model.InstalledSkill, ids []string) ([]model.InstalledSkill, error) {
	if len(ids) == 0 {
		return items, nil
	}
	wanted := model.NewStringSet()
	for _, id := range ids {
		wanted.Add(id)
	}
	out := make([]model.InstalledSkill, 0, len(ids))
	for _, item := range items {
		if wanted.Has(item.ID) {
			out = append(out, item)
			wanted.Delete(item.ID)
		}
	}
	if wanted.Len() > 0 {
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
	ResourceKind   model.ResourceKind
	All            bool
	Skills         []string
	ExposureMethod model.ExposureMethod
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
	Scope   config.Scope
	JSON    bool
	All     bool
	Skill   bool
	Plugins bool
}

type InspectOptions struct {
	Scope  config.Scope
	Source model.Source
	JSON   bool
}
