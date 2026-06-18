package spick

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
)

var pluginAddOpts struct {
	scope string
	json  bool
	force bool
}

var pluginAddCmd = &cobra.Command{Use: "add", Short: "Add a plugin", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	result, err := appService.AddPlugin(app.AddPluginOptions{Scope: config.Scope(pluginAddOpts.scope), Source: app.SourceFromLocator(args[0]), Force: pluginAddOpts.force})
	if err != nil {
		return err
	}
	if pluginAddOpts.json {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "added plugin %s (%s)\n", result.Plugin.ID, result.Plugin.Entry)
	return err
}}

func init() {
	pluginAddCmd.Flags().StringVar(&pluginAddOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
	pluginAddCmd.Flags().BoolVar(&pluginAddOpts.json, "json", false, "emit JSON")
	pluginAddCmd.Flags().BoolVar(&pluginAddOpts.force, "force", false, "force replacement")
}
