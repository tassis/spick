package spick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/ui"
)

var pluginRmOpts struct{ scope string }

var pluginRmCmd = &cobra.Command{Use: "rm", Short: "Remove plugins", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
	ids := args
	if len(ids) == 0 {
		listed, err := appService.ListPlugins(app.ListOptions{Scope: config.Scope(pluginRmOpts.scope)})
		if err != nil {
			return err
		}
		managed := make([]ui.Option, 0)
		for _, pl := range listed.Plugins {
			if pl.Declared != nil {
				managed = append(managed, ui.Option{Label: pl.ID})
			}
		}
		if len(managed) == 0 {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "no managed plugins")
			return err
		}
		picked, err := appService.Prompter.MultiSelect("Remove plugins", managed, nil)
		if err != nil {
			return err
		}
		for _, idx := range picked {
			if idx >= 0 && idx < len(managed) {
				ids = append(ids, managed[idx].Label)
			}
		}
	}
	result, err := appService.RemovePlugin(app.RemovePluginOptions{Scope: config.Scope(pluginRmOpts.scope), IDs: ids})
	if err != nil {
		return err
	}
	for _, w := range result.Warnings {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), w)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed plugins: %s\n", strings.Join(result.Removed, ", "))
	return err
}}

func init() {
	pluginRmCmd.Flags().StringVar(&pluginRmOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
}
