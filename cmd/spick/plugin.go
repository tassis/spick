package spick

import "github.com/spf13/cobra"

var pluginCmd = &cobra.Command{Use: "plugin", Short: "Manage plugin declarations and materialization"}

func init() {
	pluginCmd.AddCommand(pluginAddCmd)
	pluginCmd.AddCommand(pluginInspectCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginRmCmd)
}
