package spick

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tassis/spick/internal/model"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

type localSkill struct {
	ID   string
	Path string
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a project or resource repo",
	Long: `Initialize managed-project scaffolds.

Use plain init for an interactive choice, or init --project / init --resource
to skip the picker.

Project scaffolds write spick.yaml; resource scaffolds write spick.res.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(cmd)
	},
}

var initForce bool

func init() {
	initCmd.Flags().Bool("project", false, "scaffold spick.yaml")
	initCmd.Flags().Bool("resource", false, "scaffold spick.res.yaml")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing init outputs")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command) error {
	return runInitWithProjectOnly(cmd, false)
}

func runProjectInitOnly(cmd *cobra.Command) error {
	return runInitWithProjectOnly(cmd, true)
}

func runInitWithProjectOnly(cmd *cobra.Command, projectOnly bool) error {
	projectMode, resourceMode, err := readInitModeFlags(cmd)
	if err != nil {
		return err
	}
	if projectMode && resourceMode {
		return fmt.Errorf("--project and --resource cannot be used together")
	}
	ws := resolveInitWorkspace()
	if projectMode {
		return runProjectInit(cmd, ws)
	}
	if resourceMode {
		return runResourceInit(cmd, ws)
	}
	projectExists, resourceExists, err := inspectExistingInitTargets(ws)
	if err != nil {
		return err
	}
	if projectOnly {
		return runProjectInit(cmd, ws)
	}
	if idx, err := selectExistingInitRoute(projectExists, resourceExists); err == nil && idx >= 0 {
		return routeInitMode(cmd, ws, idx)
	} else if err != nil {
		return err
	}
	idx, err := selectInitMode()
	if err != nil {
		return err
	}
	return routeInitMode(cmd, ws, idx)
}

func readInitModeFlags(cmd *cobra.Command) (bool, bool, error) {
	if cmd == nil {
		return false, false, nil
	}
	if cmd.Flags().Lookup("project") == nil {
		return false, false, nil
	}
	projectMode, err := cmd.Flags().GetBool("project")
	if err != nil {
		return false, false, err
	}
	if cmd.Flags().Lookup("resource") == nil {
		return projectMode, false, nil
	}
	resourceMode, err := cmd.Flags().GetBool("resource")
	if err != nil {
		return false, false, err
	}
	return projectMode, resourceMode, nil
}

func resolveInitWorkspace() *workspace.Workspace {
	if appService != nil && appService.Workspace != nil {
		return appService.Workspace
	}
	ws := workspace.New("")
	if cwd, err := os.Getwd(); err == nil {
		ws.Root = cwd
	}
	return ws
}

func inspectExistingInitTargets(ws *workspace.Workspace) (bool, bool, error) {
	projectExists, err := fileExists(filepath.Join(ws.Root, "spick.yaml"))
	if err != nil {
		return false, false, err
	}
	resourceExists, err := fileExists(filepath.Join(ws.Root, "spick.res.yaml"))
	if err != nil {
		return false, false, err
	}
	if !initForce && projectExists && resourceExists {
		return false, false, fmt.Errorf("spick.yaml already exists; use --force to overwrite, and spick.res.yaml already exists; use --force to regenerate")
	}
	return projectExists, resourceExists, nil
}

func selectExistingInitRoute(projectExists, resourceExists bool) (int, error) {
	if !initForce {
		switch {
		case projectExists:
			return 1, nil
		case resourceExists:
			return 0, nil
		}
	}
	return -1, nil
}

func selectInitMode() (int, error) {
	if appService.Prompter == nil {
		return 0, fmt.Errorf("prompting is required for init")
	}
	return appService.Prompter.Select("Choose init mode", []ui.Option{{Label: "project"}, {Label: "resource"}}, 0)
}

func routeInitMode(cmd *cobra.Command, ws *workspace.Workspace, idx int) error {
	if idx == 1 {
		return runResourceInit(cmd, ws)
	}
	return runProjectInit(cmd, ws)
}

func runProjectInit(cmd *cobra.Command, ws *workspace.Workspace) error {
	if err := ensureMissing(filepath.Join(ws.Root, "spick.yaml"), initForce); err != nil {
		return err
	}
	exposure := model.ExposureMethodSymlink
	autoApply := true
	if appService.Prompter != nil {
		prompter := appService.Prompter
		idx, err := prompter.Select("Choose exposure mode", []ui.Option{{Label: "symlink"}, {Label: "copy"}}, 0)
		if err != nil {
			return err
		}
		if idx == 1 {
			exposure = model.ExposureMethodCopy
		}
		choice, err := prompter.Select("Auto-apply declared assets?", []ui.Option{{Label: "yes"}, {Label: "no"}}, 0)
		if err != nil {
			return err
		}
		autoApply = choice == 0
	}
	project := model.ProjectConfig{
		Skills:         []model.ProjectSkill{},
		Plugins:        []model.ProjectPlugin{},
		Agents:         []model.ProjectAgent{},
		Runtimes:       map[string]model.ProjectRuntimeEnablement{},
		ExposureMethod: exposure,
		AutoApply:      autoApply,
	}
	if err := ws.WriteProjectConfig(project); err != nil {
		return err
	}
	return nil
}

func runResourceInit(cmd *cobra.Command, ws *workspace.Workspace) error {
	if err := ensureMissing(filepath.Join(ws.Root, "spick.res.yaml"), initForce); err != nil {
		return err
	}
	catalog, err := discoverLocalSkillCatalog(ws.Root)
	if err != nil {
		return err
	}
	if appService.Prompter == nil {
		return fmt.Errorf("prompting is required for resource init")
	}
	out := model.ResourceManifest{Version: 1, Kind: model.ResourceKindResources, Resources: model.ResourceCollections{Skills: []model.ResourceSkill{}}}
	if len(catalog) == 0 {
		return ws.WriteResourceManifest(out)
	}
	options := make([]ui.Option, 0, len(catalog))
	for _, skill := range catalog {
		options = append(options, ui.Option{Label: skill.ID + " - " + skill.Path})
	}
	selected, err := appService.Prompter.MultiSelect("Select skills to export", options, nil)
	if err != nil {
		return err
	}
	for _, idx := range selected {
		if idx < 0 || idx >= len(catalog) {
			continue
		}
		skill := catalog[idx]
		out.Resources.Skills = append(out.Resources.Skills, model.ResourceSkill{ID: skill.ID, Path: skill.Path})
	}
	return ws.WriteResourceManifest(out)
}

func discoverLocalSkillCatalog(root string) ([]localSkill, error) {
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	options := make([]localSkill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "skills", entry.Name(), "SKILL.md")); err == nil {
			options = append(options, localSkill{ID: entry.Name(), Path: filepath.Join("skills", entry.Name())})
		}
	}
	return options, nil
}

func ensureMissing(path string, force bool) error {
	if _, err := os.Stat(path); err == nil {
		if force {
			return nil
		}
		if filepath.Base(path) == "spick.res.yaml" {
			return fmt.Errorf("%s already exists; use --force to regenerate", filepath.Base(path))
		}
		return fmt.Errorf("%s already exists", filepath.Base(path))
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
