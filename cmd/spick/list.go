package spick

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
)

var listOpts struct {
	skill   bool
	plugins bool
	json    bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show project overview",
	Long:  "Show skills, plugins, and agents by default. Use --skill or --plugins to narrow the output.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if listOpts.skill && listOpts.plugins {
			return fmt.Errorf("--skill and --plugins are mutually exclusive")
		}
		result, err := appService.List(app.ListOptions{Scope: config.ScopeProject, JSON: listOpts.json, Skill: listOpts.skill, Plugins: listOpts.plugins})
		if err != nil {
			return err
		}
		if listOpts.json {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return err
		}
		sections := []struct {
			title string
			lines []string
		}{}
		switch {
		case listOpts.skill:
			sections = append(sections, struct {
				title string
				lines []string
			}{"skills", formatSkills(result.Skills)})
		case listOpts.plugins:
			sections = append(sections, struct {
				title string
				lines []string
			}{"plugins", formatPlugins(result.Plugins)})
		default:
			sections = append(sections,
				struct {
					title string
					lines []string
				}{"skills", formatSkills(result.Skills)},
				struct {
					title string
					lines []string
				}{"plugins", formatPlugins(result.Plugins)},
				struct {
					title string
					lines []string
				}{"agents", formatAgents(result.Agents)},
			)
		}
		out := []string{}
		for _, section := range sections {
			out = append(out, section.title)
			if len(section.lines) == 0 {
				out = append(out, "  (none)")
				continue
			}
			for _, line := range section.lines {
				out = append(out, "  "+line)
			}
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(out, "\n"))
		return err
	},
}

func formatSkills(items []model.InstalledSkill) []string {
	out := make([]string, 0, len(items))
	for _, sk := range items {
		out = append(out, sk.ID)
	}
	return out
}

func formatPlugins(items []app.PluginListItem) []string {
	out := make([]string, 0, len(items))
	for _, pl := range items {
		line := pl.ID
		if pl.State != "" {
			line += " (" + pl.State + ")"
		}
		if pl.Warning != "" {
			line += " - " + pl.Warning
		}
		out = append(out, line)
	}
	return out
}

func formatAgents(items []app.AgentListItem) []string {
	out := make([]string, 0, len(items))
	for _, agent := range items {
		line := agent.ID + ": skills=" + fmt.Sprint(agent.Skills) + " plugins=" + fmt.Sprint(agent.Plugins)
		out = append(out, line)
	}
	return out
}

func init() {
	listCmd.Flags().BoolVar(&listOpts.skill, "skill", false, "show skills only")
	listCmd.Flags().BoolVar(&listOpts.plugins, "plugins", false, "show plugins only")
	listCmd.Flags().BoolVar(&listOpts.json, "json", false, "emit JSON")
}
