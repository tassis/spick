package agents

import (
	"fmt"
	"path/filepath"

	"github.com/tassis/spick/internal/config"
)

type Agent struct {
	Name                string
	ProjectExposureRoot string
	GlobalExposureRoot  string
}

var builtin = map[string]Agent{
	"opencode": {
		Name:                "opencode",
		ProjectExposureRoot: ".opencode",
		GlobalExposureRoot:  filepath.Join(".config", "opencode"),
	},
	"codex": {
		Name:                "codex",
		ProjectExposureRoot: ".agents",
		GlobalExposureRoot:  ".agents",
	},
}

func Lookup(name string) (Agent, bool) {
	a, ok := builtin[name]
	return a, ok
}

func Validate(name string) error {
	if name == "" {
		return nil
	}
	if _, ok := Lookup(name); !ok {
		return fmt.Errorf("unsupported agent %q", name)
	}
	return nil
}

func ExposureRoot(scope config.Scope, name string) (string, error) {
	a, ok := Lookup(name)
	if !ok {
		return "", fmt.Errorf("unsupported agent %q", name)
	}
	if scope == config.ScopeGlobal {
		return a.GlobalExposureRoot, nil
	}
	return a.ProjectExposureRoot, nil
}
