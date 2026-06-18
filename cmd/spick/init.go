package spick

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold project, skill, or plugin helpers",
	Long: `Initialize managed-project scaffolds.

Use plain init for a project scaffold, init --skill for a skill export helper,
and init --plugin for a plugin helper.

Project init writes explicit declaration scaffolding, while the skill and plugin
modes help generate minimal id-first source manifests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(cmd)
	},
}

func init() {
	initCmd.Flags().Bool("skill", false, "scaffold a skill export helper")
	initCmd.Flags().Bool("plugin", false, "scaffold a plugin helper")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command) error {
	skillMode, _ := cmd.Flags().GetBool("skill")
	pluginMode, _ := cmd.Flags().GetBool("plugin")
	if skillMode && pluginMode {
		return fmt.Errorf("--skill and --plugin cannot be used together")
	}
	ws := workspace.New("")
	if appService != nil && appService.Workspace != nil {
		ws = appService.Workspace
	} else if cwd, err := os.Getwd(); err == nil {
		ws.Root = cwd
	}
	switch {
	case skillMode:
		return runSkillInit(cmd, ws)
	case pluginMode:
		return runPluginInit(cmd, ws)
	default:
		return runProjectInit(cmd, ws)
	}
}

func runProjectInit(cmd *cobra.Command, ws *workspace.Workspace) error {
	if err := ensureMissing(filepath.Join(ws.Root, "spick.yaml")); err != nil {
		return err
	}
	exposure := "symlink"
	autoApply := true
	if appService.Prompter != nil {
		prompter := appService.Prompter
		idx, err := prompter.Select("Choose exposure mode", []ui.Option{{Label: "symlink"}, {Label: "copy"}}, 0)
		if err != nil {
			return err
		}
		if idx == 1 {
			exposure = "copy"
		}
		choice, err := prompter.Select("Auto-apply declared assets?", []ui.Option{{Label: "yes"}, {Label: "no"}}, 0)
		if err != nil {
			return err
		}
		autoApply = choice == 0
	}
	content := fmt.Sprintf("version: 1\nproject:\n  skills: []\n  plugins: []\n  agents: {}\n  exposureMethod: %s\n  autoApply: %t\n", exposure, autoApply)
	if err := os.WriteFile(filepath.Join(ws.Root, "spick.yaml"), []byte(content), 0o644); err != nil {
		return err
	}
	readmePath := filepath.Join(ws.Root, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		return os.WriteFile(readmePath, []byte("# New spick project\n"), 0o644)
	} else if err != nil {
		return err
	}
	return nil
}

func runSkillInit(cmd *cobra.Command, ws *workspace.Workspace) error {
	if err := ensureMissing(filepath.Join(ws.Root, "spick.skill.yaml")); err != nil {
		return err
	}
	catalog, err := (&workspace.Loader{Root: ws.Root}).LoadCatalog()
	if err != nil {
		return err
	}
	if len(catalog) == 0 {
		return fmt.Errorf("no exportable skills found")
	}
	if appService.Prompter == nil {
		return fmt.Errorf("prompting is required for skill init")
	}
	options := make([]ui.Option, 0, len(catalog))
	for _, skill := range catalog {
		options = append(options, ui.Option{Label: skill.ID + " - " + skill.Name})
	}
	selected, err := appService.Prompter.MultiSelect("Select skills to export", options, nil)
	if err != nil {
		return err
	}
	out := struct {
		Version int `yaml:"version"`
		Skills  []struct {
			ID          string `yaml:"id"`
			Path        string `yaml:"path"`
			Name        string `yaml:"name,omitempty"`
			Description string `yaml:"description"`
		} `yaml:"skills"`
	}{Version: 1}
	for _, idx := range selected {
		if idx < 0 || idx >= len(catalog) {
			continue
		}
		skill := catalog[idx]
		out.Skills = append(out.Skills, struct {
			ID          string `yaml:"id"`
			Path        string `yaml:"path"`
			Name        string `yaml:"name,omitempty"`
			Description string `yaml:"description"`
		}{ID: skill.ID, Path: skill.Source.Path, Name: skill.Name, Description: ""})
	}
	data, err := yaml.Marshal(&out)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ws.Root, "spick.skill.yaml"), data, 0o644)
}

func runPluginInit(cmd *cobra.Command, ws *workspace.Workspace) error {
	if err := ensureMissing(filepath.Join(ws.Root, "spick.plugin.yaml")); err != nil {
		return err
	}
	if appService.Prompter == nil {
		return fmt.Errorf("prompting is required for plugin init")
	}
	plugin := struct {
		ID      string `yaml:"id"`
		Entry   string `yaml:"entry"`
		Runtime string `yaml:"runtime"`
	}{
		ID:      strings.TrimSpace(filepath.Base(ws.Root)),
		Entry:   "index.ts",
		Runtime: "node",
	}
	choices := []ui.Option{{Label: "node"}, {Label: "deno"}, {Label: "bun"}}
	idx, err := appService.Prompter.Select("Choose plugin runtime", choices, 0)
	if err != nil {
		return err
	}
	if idx >= 0 && idx < len(choices) {
		plugin.Runtime = choices[idx].Label
	}
	data, err := yaml.Marshal(&plugin)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ws.Root, "spick.plugin.yaml"), data, 0o644)
}

func ensureMissing(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", filepath.Base(path))
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}
