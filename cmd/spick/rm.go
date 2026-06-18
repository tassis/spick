package spick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/ui"
)

var rmOpts struct {
	scope       string
	skills      []string
	pruneUnused bool
}

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove declared skills and enablement",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		skills := args
		if len(skills) == 0 {
			listed, err := appService.List(app.ListOptions{Scope: config.Scope(rmOpts.scope)})
			if err != nil {
				return err
			}
			if len(listed.Skills) == 0 {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "no installed skills")
				return err
			}
			options := make([]ui.Option, 0, len(listed.Skills))
			for _, sk := range listed.Skills {
				options = append(options, ui.Option{Label: sk.ID})
			}
			picked, err := appService.Prompter.MultiSelect("Remove skills", options, nil)
			if err != nil {
				return err
			}
			skills = make([]string, 0, len(picked))
			for _, idx := range picked {
				if idx >= 0 && idx < len(listed.Skills) {
					skills = append(skills, listed.Skills[idx].ID)
				}
			}
		}
		result, err := appService.Remove(app.RemoveOptions{Scope: config.Scope(rmOpts.scope), Skills: skills, PruneUnused: rmOpts.pruneUnused})
		if err != nil {
			return err
		}
		msg := strings.Join(result.Removed, ", ")
		if result.Message != "" {
			msg = result.Message + ": " + msg
		}
		if rmOpts.pruneUnused {
			msg = "pruned-unused: " + msg
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed fully: %s\n", msg)
		return err
	},
}

func init() {
	rmCmd.Flags().StringVar(&rmOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
	rmCmd.Flags().BoolVar(&rmOpts.pruneUnused, "prune-unused", false, "prune unused skills")
}
