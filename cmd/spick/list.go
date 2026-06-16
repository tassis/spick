package spick

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
)

var listOpts struct {
	scope string
	all   bool
	json  bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		if listOpts.all {
			return fmt.Errorf("--all is not supported for list")
		}
		result, err := appService.List(app.ListOptions{Scope: config.Scope(listOpts.scope), All: listOpts.all, JSON: listOpts.json})
		if err != nil { return err }
		if listOpts.json {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil { return err }
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return err
		}
		if len(result.Skills) == 0 {
			return nil
		}
		lines := make([]string, 0, len(result.Skills))
		for _, sk := range result.Skills {
			lines = append(lines, fmt.Sprintf("%s: %s", sk.ID, skillPath(sk)))
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
		return err
	},
}

func skillPath(sk model.InstalledSkill) string {
	if sk.Install != nil && sk.Install.CanonicalPath != "" { return sk.Install.CanonicalPath }
	if sk.Source != nil && sk.Source.Path != "" { return sk.Source.Path }
	return ""
}

func init() {
	listCmd.Flags().StringVar(&listOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
	listCmd.Flags().BoolVar(&listOpts.all, "all", false, "include all skills")
	listCmd.Flags().BoolVar(&listOpts.json, "json", false, "emit JSON")
}
