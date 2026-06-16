package spick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
)

var addOpts struct {
	scope   string
	source  string
	all     bool
	skills  []string
	mode    string
	agent   string
	version string
	ref     string
	force   bool
	yes     bool
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if addOpts.yes {
			return fmt.Errorf("--yes is not supported for add")
		}
		result, err := appService.Add(app.AddOptions{Scope: config.Scope(addOpts.scope), Source: app.SourceFromLocator(args[0]), All: addOpts.all, Skills: addOpts.skills, Mode: addOpts.mode, Agent: addOpts.agent, Version: addOpts.version, Ref: addOpts.ref, Force: addOpts.force, Yes: addOpts.yes})
		if err != nil { return err }
		if result.Message != "" {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return err
		}
		ids := make([]string, 0, len(result.Selected))
		for _, skill := range result.Selected { ids = append(ids, skill.ID) }
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
	addCmd.Flags().StringVar(&addOpts.mode, "mode", "", "mode to use")
	addCmd.Flags().StringVar(&addOpts.agent, "agent", "", "agent to use")
	addCmd.Flags().StringVar(&addOpts.version, "version", "", "version to use")
	addCmd.Flags().StringVar(&addOpts.ref, "ref", "", "reference to use")
	addCmd.Flags().BoolVar(&addOpts.all, "all", false, "select all skills")
	addCmd.Flags().StringSliceVar(&addOpts.skills, "skill", nil, "skill id to select (repeatable)")
	addCmd.Flags().BoolVar(&addOpts.force, "force", false, "force the operation")
	addCmd.Flags().BoolVar(&addOpts.yes, "yes", false, "skip confirmation")
	addCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		_, err := fmt.Fprintf(cmd.OutOrStderr(), "Usage:\n  %s <source> [flags]\n", cmd.CommandPath())
		return err
	})
}
