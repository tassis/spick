package spick

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
)

var applyOpts struct {
	global  bool
	runtime string
	skill   bool
	plugin  bool
	agent   bool
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Edit desired state",
	Long: `Edit desired state at the root level, then reconcile it through sync.

Use --runtime to choose a runtime explicitly. When omitted, apply uses the only
runtime automatically or prompts when multiple runtimes exist.

Use --skill, --plugin, or --agent to narrow the class being edited.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		selected := 0
		for _, enabled := range []bool{applyOpts.skill, applyOpts.plugin, applyOpts.agent} {
			if enabled {
				selected++
			}
		}
		if selected > 1 {
			return fmt.Errorf("--skill, --plugin, and --agent are mutually exclusive")
		}
		mode := "all"
		switch {
		case applyOpts.skill:
			mode = "skill"
		case applyOpts.plugin:
			mode = "plugin"
		case applyOpts.agent:
			mode = "agent"
		}
		result, err := appService.Apply(app.ApplyOptions{Global: applyOpts.global, Runtime: applyOpts.runtime, Skill: applyOpts.skill, Plugin: applyOpts.plugin, AgentMode: applyOpts.agent})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "applied %s\n", strings.TrimSpace(mode))
		_ = result
		return err
	},
}

func init() {
	applyCmd.Flags().BoolVarP(&applyOpts.global, "global", "g", false, "operate in global scope")
	applyCmd.Flags().StringVar(&applyOpts.runtime, "runtime", "", "runtime to apply")
	applyCmd.Flags().BoolVar(&applyOpts.skill, "skill", false, "narrow to skills")
	applyCmd.Flags().BoolVar(&applyOpts.plugin, "plugin", false, "narrow to plugins")
	applyCmd.Flags().BoolVar(&applyOpts.agent, "agent", false, "narrow to agents")
	applyCmd.Flags().SetInterspersed(false)
	applyCmd.MarkFlagsMutuallyExclusive("skill", "plugin", "agent")
}
