package spick

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

var rootCmd = &cobra.Command{
	Use:   "spick",
	Short: "spick skill picker",
}

var appService = app.New(ui.NewPromptTea(), workspace.New(""), skills.New(""))

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(demoPromptCmd)
}
