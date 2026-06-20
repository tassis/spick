package model

import "sort"

type StringSet map[string]struct{}

func NewStringSet(values ...string) StringSet {
	set := StringSet{}
	set.Add(values...)
	return set
}

func (s StringSet) Add(values ...string) {
	for _, value := range values {
		s[value] = struct{}{}
	}

}

func (s StringSet) AddAll(values []string) {
	for _, value := range values {
		s[value] = struct{}{}
	}
}

func (s StringSet) AddSet(other StringSet) {
	for value := range other {
		s[value] = struct{}{}
	}
}

func (s StringSet) Has(value string) bool {
	_, ok := s[value]
	return ok
}

func (s StringSet) Delete(value string) {
	delete(s, value)
}

func (s StringSet) Len() int {
	return len(s)
}

func (s StringSet) Keys() []string {
	out := make([]string, 0, len(s))
	for key := range s {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

type ProjectSkillByID map[string]ProjectSkill

func IndexProjectSkills(skills []ProjectSkill) ProjectSkillByID {
	out := make(ProjectSkillByID, len(skills))
	for _, skill := range skills {
		out[skill.ID] = skill
	}
	return out
}

type ProjectPluginByID map[string]ProjectPlugin

func IndexProjectPlugins(plugins []ProjectPlugin) ProjectPluginByID {
	out := make(ProjectPluginByID, len(plugins))
	for _, plugin := range plugins {
		out[plugin.ID] = plugin
	}
	return out
}

type InstalledSkillByID map[string]InstalledSkill

func IndexInstalledSkills(skills []InstalledSkill) InstalledSkillByID {
	out := make(InstalledSkillByID, len(skills))
	for _, skill := range skills {
		out[skill.ID] = skill
	}
	return out
}

type LockPluginByID map[string]LockPlugin

func IndexLockPlugins(plugins []LockPlugin) LockPluginByID {
	out := make(LockPluginByID, len(plugins))
	for _, plugin := range plugins {
		out[plugin.ID] = plugin
	}
	return out
}

type ExposureByAgent map[string]Exposure

func (s ExposureByAgent) Agents() []string {
	out := make([]string, 0, len(s))
	for agent := range s {
		out = append(out, agent)
	}
	sort.Strings(out)
	return out
}
