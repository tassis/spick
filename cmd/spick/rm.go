package spick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
)

var rmOpts struct{ global, skill, plugin, agent, pruneUnused bool }

var rmCmd = &cobra.Command{Use: "rm", Short: "Manage removals", Long: "Manage removals from a single root-level surface. Use --skill, --plugin, or --agent to narrow selection.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
	scope := config.ScopeProject
	if rmOpts.global {
		scope = config.ScopeGlobal
	}
	if result, err := ensureProjectInitPreflight(cmd, scope); err != nil {
		return err
	} else if result.stop {
		return nil
	}
	mode := ""
	switch {
	case rmOpts.skill:
		mode = "skill"
	case rmOpts.plugin:
		mode = "plugin"
	case rmOpts.agent:
		return fmt.Errorf("no valid agent resource found")
	}
	selectedMode := app.RemoveSelectionModeAuto
	if mode == "skill" {
		selectedMode = app.RemoveSelectionModeSkill
	}
	if mode == "plugin" {
		selectedMode = app.RemoveSelectionModePlugin
	}
	selected, err := appService.SelectRemovals(app.RemoveSelectionOptions{Scope: scope, Mode: selectedMode})
	if err != nil {
		return err
	}
	selectedSkills := selected.Skills
	selectedPlugins := selected.Plugins
	if len(selectedSkills) > 0 {
		if _, err := appService.Remove(app.RemoveOptions{Scope: scope, Skills: selectedSkills, PruneUnused: rmOpts.pruneUnused}); err != nil {
			return err
		}
	}
	if len(selectedPlugins) > 0 {
		if _, err := appService.RemovePlugin(app.RemovePluginOptions{Scope: scope, IDs: selectedPlugins}); err != nil {
			return err
		}
	}
	removed := append([]string{}, selectedSkills...)
	removed = append(removed, selectedPlugins...)
	if len(removed) == 0 {
		return nil
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed fully: %s\n", strings.Join(removed, ", "))
	return err
}}

func init() {
	rmCmd.Flags().BoolVarP(&rmOpts.global, "global", "g", false, "use global mode")
	rmCmd.Flags().BoolVar(&rmOpts.skill, "skill", false, "narrow to skills")
	rmCmd.Flags().BoolVar(&rmOpts.plugin, "plugin", false, "narrow to plugins")
	rmCmd.Flags().BoolVar(&rmOpts.agent, "agent", false, "narrow to agents")
	rmCmd.Flags().BoolVar(&rmOpts.pruneUnused, "prune-unused", false, "prune unused skills")
	rmCmd.MarkFlagsMutuallyExclusive("skill", "plugin", "agent")
}
