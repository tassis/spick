package spick

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/ui"
)

var demoPromptCmd = &cobra.Command{
	Use:    "demo-prompt",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := ui.NewPromptTea()
		choice, err := p.Select("Pick one", []ui.Option{{Label: "one"}, {Label: "two"}}, 0)
		if err != nil {
			return err
		}
		multi, err := p.MultiSelect("Pick many", []ui.Option{{Label: "one"}, {Label: "two"}, {Label: "three"}}, []int{0, 2})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "choice=%d multi=%v\n", choice, multi)
		return nil
	},
}
