package model

import "fmt"

func CloneRuntimeEnablement(in map[string]ProjectRuntimeEnablement) map[string]ProjectRuntimeEnablement {
	if len(in) == 0 {
		return map[string]ProjectRuntimeEnablement{}
	}
	out := make(map[string]ProjectRuntimeEnablement, len(in))
	for agent, entry := range in {
		out[agent] = ProjectRuntimeEnablement{
			Skills:  append([]string(nil), entry.Skills...),
			Plugins: append([]string(nil), entry.Plugins...),
			Agents:  append([]string(nil), entry.Agents...),
		}
	}
	return out
}

func NormalizeProjectSkills(in []ProjectSkill) ([]ProjectSkill, error) {
	seen := map[string]struct{}{}
	out := make([]ProjectSkill, 0, len(in))
	for _, item := range in {
		if item.ID == "" || item.Source == "" {
			return nil, fmt.Errorf("invalid project.skills entry")
		}
		if _, ok := seen[item.ID]; ok {
			return nil, fmt.Errorf("duplicate project.skills id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		out = append(out, ProjectSkill{ID: item.ID, Source: item.Source, Ref: item.Ref})
	}
	return out, nil
}

func NormalizeProjectPlugins(in []ProjectPlugin) ([]ProjectPlugin, error) {
	seen := map[string]struct{}{}
	out := make([]ProjectPlugin, 0, len(in))
	for _, item := range in {
		if item.ID == "" || item.Source == "" {
			return nil, fmt.Errorf("invalid project.plugins entry")
		}
		if _, ok := seen[item.ID]; ok {
			return nil, fmt.Errorf("duplicate project.plugins id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		out = append(out, ProjectPlugin{ID: item.ID, Source: item.Source, Ref: item.Ref})
	}
	return out, nil
}

type ProjectRuntimeEnablementRaw struct {
	Skills  []string
	Plugins []string
	Agents  []string
}

func NormalizeProjectRuntimeEnablement(in map[string]ProjectRuntimeEnablementRaw, skills []ProjectSkill, plugins []ProjectPlugin, agents []ProjectAgent) (map[string]ProjectRuntimeEnablement, error) {
	declaredSkills := map[string]struct{}{}
	for _, skill := range skills {
		declaredSkills[skill.ID] = struct{}{}
	}
	declaredPlugins := map[string]struct{}{}
	for _, plugin := range plugins {
		declaredPlugins[plugin.ID] = struct{}{}
	}
	declaredAgents := map[string]struct{}{}
	for _, agent := range agents {
		declaredAgents[agent.ID] = struct{}{}
	}
	out := map[string]ProjectRuntimeEnablement{}
	for agent, entry := range in {
		if agent == "" {
			return nil, fmt.Errorf("invalid project.runtimes entry")
		}
		for _, id := range entry.Skills {
			if _, ok := declaredSkills[id]; !ok {
				return nil, fmt.Errorf("project.runtimes.%s references undeclared skill %q", agent, id)
			}
		}
		for _, id := range entry.Plugins {
			if _, ok := declaredPlugins[id]; !ok {
				return nil, fmt.Errorf("project.runtimes.%s references undeclared plugin %q", agent, id)
			}
		}
		for _, id := range entry.Agents {
			if _, ok := declaredAgents[id]; !ok {
				return nil, fmt.Errorf("project.runtimes.%s references undeclared agent %q", agent, id)
			}
		}
		out[agent] = ProjectRuntimeEnablement{
			Skills:  append([]string(nil), entry.Skills...),
			Plugins: append([]string(nil), entry.Plugins...),
			Agents:  append([]string(nil), entry.Agents...),
		}
	}
	return out, nil
}

func UpsertProjectSkill(items []ProjectSkill, next ProjectSkill) []ProjectSkill {
	for i, item := range items {
		if item.ID == next.ID {
			items[i] = next
			return items
		}
	}
	return append(items, next)
}

func UpsertProjectPlugin(items []ProjectPlugin, next ProjectPlugin) []ProjectPlugin {
	for i, item := range items {
		if item.ID == next.ID {
			items[i] = next
			return items
		}
	}
	return append(items, next)
}

func WithoutProjectSkillIDs(items []ProjectSkill, remove []string) []ProjectSkill {
	if len(remove) == 0 {
		return append([]ProjectSkill(nil), items...)
	}
	rem := map[string]struct{}{}
	for _, id := range remove {
		rem[id] = struct{}{}
	}
	out := items[:0]
	for _, item := range items {
		if _, ok := rem[item.ID]; !ok {
			out = append(out, item)
		}
	}
	return append([]ProjectSkill(nil), out...)
}

func WithoutProjectPluginIDs(items []ProjectPlugin, remove []string) []ProjectPlugin {
	if len(remove) == 0 {
		return append([]ProjectPlugin(nil), items...)
	}
	rem := map[string]struct{}{}
	for _, id := range remove {
		rem[id] = struct{}{}
	}
	out := items[:0]
	for _, item := range items {
		if _, ok := rem[item.ID]; !ok {
			out = append(out, item)
		}
	}
	return append([]ProjectPlugin(nil), out...)
}

func RemoveIDsFromRuntimeEnablements(runtimes map[string]ProjectRuntimeEnablement, skillIDs, pluginIDs []string) map[string]ProjectRuntimeEnablement {
	if len(runtimes) == 0 {
		return CloneRuntimeEnablement(runtimes)
	}
	removeSkills := map[string]struct{}{}
	for _, id := range skillIDs {
		removeSkills[id] = struct{}{}
	}
	removePlugins := map[string]struct{}{}
	for _, id := range pluginIDs {
		removePlugins[id] = struct{}{}
	}
	out := CloneRuntimeEnablement(runtimes)
	for agent, entry := range out {
		skills := entry.Skills[:0]
		for _, id := range entry.Skills {
			if _, ok := removeSkills[id]; !ok {
				skills = append(skills, id)
			}
		}
		entry.Skills = skills
		plugins := entry.Plugins[:0]
		for _, id := range entry.Plugins {
			if _, ok := removePlugins[id]; !ok {
				plugins = append(plugins, id)
			}
		}
		entry.Plugins = plugins
		out[agent] = entry
	}
	return out
}
