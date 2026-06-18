package spick

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/app"
	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
)

var listOpts struct {
	scope string
	json  bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := appService.List(app.ListOptions{Scope: config.Scope(listOpts.scope), JSON: listOpts.json})
		if err != nil {
			return err
		}
		if listOpts.json {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return err
		}
		if len(result.Skills) == 0 {
			return nil
		}
		lines := make([]string, 0, len(result.Skills))
		for _, sk := range result.Skills {
			lines = append(lines, fmt.Sprintf("%s%s", sk.ID, appliedAgentsBadge(sk)))
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
		return err
	},
}

func appliedAgentsBadge(sk model.InstalledSkill) string {
	if len(sk.Exposures) == 0 {
		return ""
	}
	agents := make([]string, 0, len(sk.Exposures))
	seen := map[string]struct{}{}
	for _, exposure := range sk.Exposures {
		if exposure.Agent == "" {
			continue
		}
		if _, ok := seen[exposure.Agent]; ok {
			continue
		}
		seen[exposure.Agent] = struct{}{}
		agents = append(agents, exposure.Agent)
	}
	if len(agents) == 0 {
		return ""
	}
	sort.Strings(agents)
	return fmt.Sprintf(" [%s]", strings.Join(agents, ", "))
}

func init() {
	listCmd.Flags().StringVar(&listOpts.scope, "scope", string(config.ScopeProject), "scope to operate in")
	listCmd.Flags().BoolVar(&listOpts.json, "json", false, "emit JSON")
}
