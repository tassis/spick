package spick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
)

var applyOpts struct {
	scope          string
	skills         []string
	agent          string
	exposureMethod string
	force          bool
	json           bool
}

var applyCmd = &cobra.Command{
	Use:   "apply [skill-id...]",
	Short: "Update skill enablement for agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		skills := append([]string(nil), applyOpts.skills...)
		if len(args) > 0 {
			skills = append(skills, args...)
		}
		result, err := appService.Apply(app.ApplyOptions{Scope: config.Scope(applyOpts.scope), Skills: skills, Agent: applyOpts.agent, ExposureMethod: applyOpts.exposureMethod, Force: applyOpts.force, JSON: applyOpts.json})
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(result.Applied))
		for _, skill := range result.Applied {
			ids = append(ids, skill.ID)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "applied %s\n", strings.Join(ids, ", "))
		return err
	},
}

func init() {
	applyCmd.Flags().StringVar(&applyOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
	applyCmd.Flags().StringVar(&applyOpts.agent, "agent", "", "agent to use")
	applyCmd.Flags().StringVar(&applyOpts.exposureMethod, "exposure-method", "", "exposure method to use")
	applyCmd.Flags().BoolVar(&applyOpts.force, "force", false, "force the operation")
	applyCmd.Flags().BoolVar(&applyOpts.json, "json", false, "emit JSON")
}
