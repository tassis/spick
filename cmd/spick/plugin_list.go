package spick

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
)

var pluginListOpts struct {
	scope string
	json  bool
}

var pluginListCmd = &cobra.Command{Use: "list", Short: "List plugins", RunE: func(cmd *cobra.Command, args []string) error {
	result, err := appService.ListPlugins(app.ListOptions{Scope: config.Scope(pluginListOpts.scope), JSON: pluginListOpts.json})
	if err != nil {
		return err
	}
	if pluginListOpts.json {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	}
	if len(result.Plugins) == 0 {
		return nil
	}
	lines := make([]string, 0, len(result.Plugins))
	for _, pl := range result.Plugins {
		line := fmt.Sprintf("%s: %s", pl.ID, pl.State)
		if pl.Warning != "" {
			line += " (" + pl.Warning + ")"
		}
		if pl.Drift {
			line += " [drift]"
		}
		lines = append(lines, line)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
	return err
}}

func init() {
	pluginListCmd.Flags().StringVar(&pluginListOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
	pluginListCmd.Flags().BoolVar(&pluginListOpts.json, "json", false, "emit JSON")
}
