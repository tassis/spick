package spick

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/ui"
)

type preflightResult struct {
	stop bool
}

func ensureProjectInitPreflight(cmd *cobra.Command, scope config.Scope) (preflightResult, error) {
	if scope != config.ScopeProject || appService == nil || appService.Workspace == nil {
		return preflightResult{}, nil
	}
	if cmd != nil {
		if v, err := cmd.Flags().GetBool("global"); err == nil && v {
			return preflightResult{}, nil
		}
	}
	if _, err := os.Stat(filepath.Join(appService.Workspace.Root, "spick.yaml")); err == nil {
		return preflightResult{}, nil
	} else if !os.IsNotExist(err) {
		return preflightResult{}, err
	}
	if appService.Prompter == nil {
		return preflightResult{}, fmt.Errorf("project config is missing; run spick init first")
	}
	choice, err := appService.Prompter.Select("Project config is missing. Initialize now?", []ui.Option{{Label: "yes"}, {Label: "no"}}, 0)
	if err != nil {
		return preflightResult{}, err
	}
	if choice != 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "project config is missing; run spick init first")
		return preflightResult{stop: true}, nil
	}
	if err := runProjectInitOnly(cmd); err != nil {
		return preflightResult{}, err
	}
	return preflightResult{}, nil
}
