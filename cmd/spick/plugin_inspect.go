package spick

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
)

var pluginInspectOpts struct {
	scope string
	json  bool
}

var pluginInspectCmd = &cobra.Command{Use: "inspect", Short: "Inspect a plugin source", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	result, err := appService.InspectPlugin(app.PluginInspectOptions{Scope: config.Scope(pluginInspectOpts.scope), Source: app.SourceFromLocator(args[0])})
	if err != nil {
		return err
	}
	if pluginInspectOpts.json {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	}
	if result.Plugin.Revision != "" {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s @ %s\n", result.Plugin.ID, result.Plugin.Runtime, result.Plugin.Entry, result.Plugin.Revision)
	} else {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", result.Plugin.ID, result.Plugin.Runtime, result.Plugin.Entry)
	}
	return err
}}

func init() {
	pluginInspectCmd.Flags().StringVar(&pluginInspectOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
	pluginInspectCmd.Flags().BoolVar(&pluginInspectOpts.json, "json", false, "emit JSON")
}
