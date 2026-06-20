package spick

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/config"
)

var syncOpts struct {
	global bool
	locked bool
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Reconcile project intent and snapshot",
	Long: `Reconcile the project declaration with local materialization.

Default sync rebuilds managed state from the current project intent, refreshes the
lockfile, and reports restored or changed items, items already in sync, and any
unmanaged plugin material that was found.

Use -g/--global to reconcile the global scope; project sync is the default.

--locked restores strictly from the lockfile snapshot, never rewrites the lockfile,
and fails if the snapshot is missing or incomplete.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scope := config.ScopeProject
		if syncOpts.global {
			scope = config.ScopeGlobal
		}
		result, err := appService.Sync(scope, syncOpts.locked)
		if err != nil {
			return err
		}
		for _, msg := range result.SkillMessages {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), msg)
		}
		for _, msg := range result.PluginMessages {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), msg)
		}
		for _, warn := range result.Warnings {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), warn)
		}
		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVarP(&syncOpts.global, "global", "g", false, "reconcile global scope")
	syncCmd.Flags().BoolVar(&syncOpts.locked, "locked", false, "restore strictly from lockfile snapshot without rewriting it")
}
