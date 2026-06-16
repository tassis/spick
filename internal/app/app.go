package app

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tassis/spick/internal/model"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/lock"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

type App struct {
	Prompter ui.Prompter
	Workspace *workspace.Workspace
	Skills    *skills.Service
	Locks     *lock.Store
}

func New(prompter ui.Prompter, ws *workspace.Workspace, svc *skills.Service) *App {
	return &App{Prompter: prompter, Workspace: ws, Skills: svc, Locks: lock.New(ws.Root)}
}

func SourceFromLocator(locator string) model.Source { return model.Source{Locator: locator} }

type AddResult struct {
	Source   model.Source         `json:"source"`
	Catalog  []model.CatalogSkill `json:"catalog,omitempty"`
	Selected []model.CatalogSkill `json:"selected,omitempty"`
	Message  string               `json:"message,omitempty"`
	Added    []skills.InstalledSkillResult `json:"added,omitempty"`
}

func (a *App) Add(opts AddOptions) (*AddResult, error) {
	if a == nil || a.Workspace == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if err := validateAddOptions(opts); err != nil {
		return nil, err
	}
	parsed, err := a.Workspace.ParseSource(opts.Source.Locator)
	if err != nil {
		return nil, err
	}
	if err := validateHostedOptions(parsed.Kind, opts); err != nil {
		return nil, err
	}
	if opts.Ref != "" {
		parsed.Source.RequestedVersion = opts.Ref
	}
	originalSource := parsed.Source
	opened, err := a.Workspace.OpenSource(parsed.Source)
	if err != nil {
		return nil, err
	}
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
	result := &AddResult{Source: opts.Source, Catalog: catalog, Selected: selected}
	if len(selected) == 0 {
		result.Message = "no skills selected"
		return result, nil
	}
	if a.Skills == nil {
		result.Message = "selected skills"
		return result, nil
	}
	added, err := a.Skills.Add(skills.AddOptions{Scope: string(opts.Scope), SourceRoot: opts.Source.Path, All: opts.All, Skills: opts.Skills, Selected: selected, Mode: opts.Mode, Agent: opts.Agent, Version: opts.Version, Ref: opts.Ref, Force: opts.Force, Yes: opts.Yes})
	if err != nil {
		return nil, err
	}
	result.Added = added.Installed
	if a.Locks != nil {
		installed := make([]model.InstalledSkill, 0, len(added.Installed))
		for _, item := range added.Installed {
			installed = append(installed, model.InstalledSkill{ID: item.ID, Source: mergeSkillSource(&originalSource, item.Source), Install: &model.SkillInstall{Mode: item.Mode, CanonicalPath: item.Target}, Exposures: item.Exposures})
		}
		if err := a.Locks.UpsertInstalled(string(opts.Scope), installed); err != nil { return nil, err }
	}
	if len(added.Installed) > 0 {
		result.Message = "selected and materialized skills"
	}
	return result, nil
}

func validateAddOptions(opts AddOptions) error {
	if opts.Agent != "" && opts.Agent != "opencode" {
		return fmt.Errorf("unsupported agent %q", opts.Agent)
	}
	if opts.Mode != "" && opts.Mode != "copy" && opts.Mode != "symlink" {
		return fmt.Errorf("unsupported mode %q", opts.Mode)
	}
	if opts.Version != "" {
		return fmt.Errorf("version is not yet supported")
	}
	return nil
}

func validateHostedOptions(kind string, opts AddOptions) error {
	if kind == "local" {
		if opts.Ref != "" {
			return fmt.Errorf("ref is not yet supported for local sources")
		}
	}
	return nil
}

type ListResult struct {
	Scope  string                `json:"scope"`
	Skills []model.InstalledSkill `json:"skills"`
}

func mergeSkillSource(original, discovered *model.Source) *model.SkillSource {
	if original == nil && discovered == nil { return nil }
	out := &model.SkillSource{}
	if original != nil {
		out.Locator = original.Locator
		out.RequestedVersion = original.RequestedVersion
	}
	if discovered != nil {
		if out.Path == "" { out.Path = discovered.Path }
		if out.Locator == "" { out.Locator = discovered.Locator }
		if out.RequestedVersion == "" { out.RequestedVersion = discovered.RequestedVersion }
	}
	return out
}

func (a *App) List(opts ListOptions) (*ListResult, error) {
	if a == nil || a.Locks == nil { return &ListResult{Scope: string(opts.Scope)}, nil }
	items, err := a.Locks.List(string(opts.Scope))
	if err != nil {
		if os.IsNotExist(err) { return &ListResult{Scope: string(opts.Scope)}, nil }
		return nil, err
	}
	return &ListResult{Scope: string(opts.Scope), Skills: items}, nil
}

type RemoveResult struct {
	Removed []string `json:"removed,omitempty"`
	Purged []string `json:"purged,omitempty"`
	Pruned []string `json:"pruned,omitempty"`
	Message string `json:"message,omitempty"`
}

func (a *App) Remove(opts RemoveOptions) (*RemoveResult, error) {
	if a == nil || a.Locks == nil { return &RemoveResult{Message: "no lock store"}, nil }
	if err := a.Locks.RemoveFiles(string(opts.Scope), opts.Skills, opts.Purge, opts.PruneUnused); err != nil { return nil, err }
	return &RemoveResult{Removed: opts.Skills, Purged: func() []string { if opts.Purge { return opts.Skills }; return nil }(), Message: "removed skills"}, nil
}

type InspectResult struct {
	Source  model.Source      `json:"source"`
	Skills  []model.CatalogSkill `json:"skills"`
	Message string            `json:"message,omitempty"`
}

func (a *App) Inspect(opts InspectOptions) (*InspectResult, error) {
	if a == nil || a.Workspace == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	parsed, err := a.Workspace.ParseSource(opts.Source.Locator)
	if err != nil {
		return nil, err
	}
	if err := validateHostedOptions(parsed.Kind, AddOptions{Ref: opts.Ref}); err != nil {
		return nil, err
	}
	if opts.Ref != "" {
		parsed.Source.RequestedVersion = opts.Ref
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

func selectCatalogSkills(p ui.Prompter, catalog []model.CatalogSkill, opts AddOptions) ([]model.CatalogSkill, error) {
	if opts.All {
		return catalog, nil
	}
	if len(opts.Skills) > 0 {
		wanted := map[string]bool{}
		for _, id := range opts.Skills { wanted[id] = true }
		out := make([]model.CatalogSkill, 0, len(opts.Skills))
		for _, skill := range catalog {
			if wanted[skill.ID] { out = append(out, skill) }
			delete(wanted, skill.ID)
		}
		if len(wanted) > 0 {
			missing := make([]string, 0, len(wanted))
			for id := range wanted { missing = append(missing, id) }
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

func catalogOptions(catalog []model.CatalogSkill) []ui.Option {
	out := make([]ui.Option, 0, len(catalog))
	for _, skill := range catalog { out = append(out, ui.Option{Label: skill.ID}) }
	return out
}

type AddOptions struct {
	Scope  config.Scope
	Source model.Source
	All    bool
	Skills []string
	Mode   string
	Agent  string
	Version string
	Ref    string
	Force  bool
	Yes    bool
}

type RemoveOptions struct {
	Scope       config.Scope
	Skills      []string
	Purge       bool
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
	Ref    string
	JSON   bool
}
