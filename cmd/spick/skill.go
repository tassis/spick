package spick

import "github.com/spf13/cobra"

var skillCmd = &cobra.Command{Use: "skill", Short: "Manage skill declarations and agent enablement"}

func init() {
	skillCmd.AddCommand(addCmd)
	skillCmd.AddCommand(inspectCmd)
	skillCmd.AddCommand(listCmd)
	skillCmd.AddCommand(rmCmd)
	skillCmd.AddCommand(applyCmd)
}
