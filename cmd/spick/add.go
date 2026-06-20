package spick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/model"
	"github.com/tassis/spick/internal/config"
)

var addOpts struct {
	global         bool
	skill          bool
	plugin         bool
	agent          bool
	all            bool
	skills         []string
	exposureMethod string
	force          bool
}

var addCmd = &cobra.Command{
	Use:   "add <source>",
	Short: "Manage additions",
	Long:  "Manage additions from a single source-oriented add surface. Use --skill, --plugin, or --agent to narrow classification.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope := config.ScopeProject
		if addOpts.global {
			scope = config.ScopeGlobal
		}
		if result, err := ensureProjectInitPreflight(cmd, scope); err != nil {
			return err
		} else if result.stop {
			return nil
		}
		kind := model.ResourceKind("")
		switch {
		case addOpts.plugin:
			kind = model.ResourceKindPlugin
		case addOpts.agent:
			kind = model.ResourceKindAgent
		case addOpts.skill:
			kind = model.ResourceKindResources
		}
		result, err := appService.Add(app.AddOptions{Scope: scope, Source: app.SourceFromLocator(args[0]), ResourceKind: kind, All: addOpts.all, Skills: addOpts.skills, ExposureMethod: model.ExposureMethod(addOpts.exposureMethod), Force: addOpts.force})
		if err != nil {
			return err
		}
		if result.Message != "" {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return err
		}
		ids := make([]string, 0, len(result.Selected))
		for _, skill := range result.Selected {
			ids = append(ids, skill.ID)
		}
		source := result.Source.Locator
		if source == "" {
			source = result.Source.Path
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "added %s -> %s\n", source, strings.Join(ids, ", "))
		return err
	},
}

func init() {
	addCmd.Flags().BoolVarP(&addOpts.global, "global", "g", false, "use global mode")
	addCmd.Flags().BoolVar(&addOpts.skill, "skill", false, "narrow to skills")
	addCmd.Flags().BoolVar(&addOpts.plugin, "plugin", false, "narrow to plugins")
	addCmd.Flags().BoolVar(&addOpts.agent, "agent", false, "narrow to agents")
	addCmd.MarkFlagsMutuallyExclusive("skill", "plugin", "agent")
	addCmd.Flags().StringVar(&addOpts.exposureMethod, "exposure-method", "", "exposure method to use")
	addCmd.Flags().BoolVar(&addOpts.all, "all", false, "select all skills")
	addCmd.Flags().StringSliceVar(&addOpts.skills, "skill-id", nil, "skill id to select (repeatable)")
	addCmd.Flags().BoolVar(&addOpts.force, "force", false, "force the operation")
	addCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		_, err := fmt.Fprintf(cmd.OutOrStderr(), "Usage:\n  %s <source> [flags]\n", cmd.CommandPath())
		return err
	})
}
