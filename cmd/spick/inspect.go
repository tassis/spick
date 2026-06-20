package spick

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
)

var inspectOpts struct {
	json bool
}

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect a source",
	Long:  "Inspect a source using manifest-first repo detection. Resource repos use spick.res.yaml and do not fall back to plugin classification without a manifest.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := appService.Inspect(app.InspectOptions{Source: app.SourceFromLocator(args[0]), JSON: inspectOpts.json})
		if err != nil {
			return err
		}
		if inspectOpts.json {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return err
		}
		if result.Message != "" {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return err
		}
		if len(result.Skills) == 0 {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "no skills found")
			return err
		}
		var lines []string
		for _, skill := range result.Skills {
			lines = append(lines, fmt.Sprintf("%s %s", skill.ID, skill.Name))
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
		return err
	},
}

func init() {
	inspectCmd.Flags().BoolVar(&inspectOpts.json, "json", false, "emit JSON")
	inspectCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		_, err := fmt.Fprintf(cmd.OutOrStderr(), "Usage:\n  %s <source> [flags]\n", cmd.CommandPath())
		return err
	})
}
