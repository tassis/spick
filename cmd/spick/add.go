package spick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
)

var addOpts struct {
	scope          string
	source         string
	all            bool
	skills         []string
	exposureMethod string
	agent          string
	force          bool
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Declare a skill and reconcile exposure",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := appService.Add(app.AddOptions{Scope: config.Scope(addOpts.scope), Source: app.SourceFromLocator(args[0]), All: addOpts.all, Skills: addOpts.skills, ExposureMethod: addOpts.exposureMethod, Agent: addOpts.agent, Force: addOpts.force})
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
	addCmd.Flags().StringVar(&addOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
	addCmd.Flags().StringVar(&addOpts.exposureMethod, "exposure-method", "", "exposure method to use")
	addCmd.Flags().StringVar(&addOpts.agent, "agent", "", "agent to use")
	addCmd.Flags().BoolVar(&addOpts.all, "all", false, "select all skills")
	addCmd.Flags().StringSliceVar(&addOpts.skills, "skill", nil, "skill id to select (repeatable)")
	addCmd.Flags().BoolVar(&addOpts.force, "force", false, "force the operation")
	addCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		_, err := fmt.Fprintf(cmd.OutOrStderr(), "Usage:\n  %s <source> [flags]\n", cmd.CommandPath())
		return err
	})
}
