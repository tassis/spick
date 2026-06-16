package workspace

import (
	"fmt"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/model"
	"gopkg.in/yaml.v3"
)

type Workspace struct {
	Root string
}

type SourceRepo struct {
	Root string
}

type ManifestDiscovery struct {
	Repo *SourceRepo
}

type Catalog struct {
	Root string
}

type Loader struct {
	Root string
}

type Manifest struct {
	Version int              `yaml:"version"`
	Catalog ManifestCatalog  `yaml:"catalog"`
}

type ManifestCatalog struct {
	Skills []ManifestSkill `yaml:"skills"`
}

type ManifestSkill struct {
	ID          string `yaml:"id"`
	Path        string `yaml:"path"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type ParsedSource struct {
	Kind   string
	Source model.Source
}

func New(root string) *Workspace { return &Workspace{Root: root} }

func (w *Workspace) Resolve(scope config.Scope) (string, error) { return w.Root, nil }
func (w *Workspace) SourceRepo(scope config.Scope) *SourceRepo { return &SourceRepo{Root: w.Root} }
func (w *Workspace) Discover(scope config.Scope, source model.Source) (*Catalog, error) { return &Catalog{Root: w.Root}, nil }
func (w *Workspace) BuildCatalog(scope config.Scope, source model.Source) ([]model.CatalogSkill, error) {
	if source.Path == "" {
		return nil, fmt.Errorf("local source path required")
	}
	loader := &Loader{Root: source.Path}
	return loader.LoadCatalog()
}

func (w *Workspace) OpenSource(source model.Source) (model.Source, error) {
	if source.Locator == "" { return source, nil }
	if source.Path != "" { return source, nil }
	parsed, err := w.ParseSource(source.Locator)
	if err != nil { return model.Source{}, err }
	if parsed.Kind == "local" { return parsed.Source, nil }
	checkout, err := w.fetchHosted(parsed, source.RequestedVersion)
	if err != nil { return model.Source{}, err }
	return model.Source{Path: checkout}, nil
}

func (w *Workspace) fetchHosted(parsed *ParsedSource, ref string) (string, error) {
	root, err := os.MkdirTemp("", "spick-checkout-*")
	if err != nil { return "", err }
	url := hostedURL(parsed)
	args := []string{"clone", "--depth", "1"}
	if ref != "" { args = append(args, "--branch", ref) }
	args = append(args, url, root)
	gitBin, err := gitBinary()
	if err != nil { return "", err }
	cmd := exec.Command(gitBin, args...)
	if out, err := cmd.CombinedOutput(); err != nil { return "", hostedCloneError(url, ref, out, err) }
	return root, nil
}

func gitBinary() (string, error) {
	if bin := os.Getenv("SPICK_GIT_BIN"); bin != "" { return bin, nil }
	bin, err := exec.LookPath("git")
	if err != nil { return "", fmt.Errorf("git is required for hosted sources: %w", err) }
	return bin, nil
}

func hostedCloneError(url, ref string, out []byte, err error) error {
	msg := strings.TrimSpace(string(out))
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "remote branch") || strings.Contains(lower, "not found in upstream origin") {
		return fmt.Errorf("hosted ref %q not found for %s", ref, url)
	}
	if msg == "" { msg = err.Error() }
	if ref != "" {
		return fmt.Errorf("failed to clone hosted source %s at ref %q: %s", url, ref, msg)
	}
	return fmt.Errorf("failed to clone hosted source %s: %s", url, msg)
}

func hostedURL(parsed *ParsedSource) string {
	if base := os.Getenv("SPICK_GIT_BASE_URL"); base != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimPrefix(parsed.Source.Locator, parsed.Kind+":"))
	}
	path := strings.TrimPrefix(parsed.Source.Locator, parsed.Kind+":")
	if parsed.Kind == "github" { return fmt.Sprintf("https://github.com/%s.git", path) }
	return fmt.Sprintf("https://gitlab.com/%s.git", path)
}

func (l *Loader) LoadCatalog() ([]model.CatalogSkill, error) {
	manifest, err := l.LoadManifest()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return l.DiscoverCatalog()
		}
		return nil, err
	}
	return l.NormalizeCatalog(manifest)
}

func (l *Loader) LoadManifest() (*Manifest, error) {
	manifestPath := filepath.Join(l.Root, "spick.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	return parseManifest(data)
}

func (l *Loader) DiscoverCatalog() ([]model.CatalogSkill, error) {
	if _, err := os.Stat(filepath.Join(l.Root, "SKILL.md")); err == nil {
		return []model.CatalogSkill{l.skillFromDir(filepath.Base(l.Root), ".")}, nil
	}
	var out []model.CatalogSkill
	err := filepath.WalkDir(l.Root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == l.Root {
			return nil
		}
		name := d.Name()
		if d.IsDir() && excludedDir(name) {
			return filepath.SkipDir
		}
		if !d.IsDir() && name == "SKILL.md" {
			relDir, err := filepath.Rel(l.Root, filepath.Dir(path))
			if err != nil {
				return err
			}
			if relDir == "." {
				return nil
			}
			if hasExcludedPrefix(relDir) {
				return nil
			}
			out = append(out, l.skillFromDir(filepath.Base(filepath.Dir(path)), relDir))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no manifest or skills found")
	}
	return out, nil
}

func (l *Loader) NormalizeCatalog(m *Manifest) ([]model.CatalogSkill, error) {
	if m == nil {
		return nil, fmt.Errorf("manifest is required")
	}
	if len(m.Catalog.Skills) == 0 {
		return nil, fmt.Errorf("catalog.skills is required")
	}
	seen := map[string]bool{}
	out := make([]model.CatalogSkill, 0, len(m.Catalog.Skills))
	for _, skill := range m.Catalog.Skills {
		if !skillIDRe.MatchString(skill.ID) {
			return nil, fmt.Errorf("invalid skill id %q", skill.ID)
		}
		if seen[skill.ID] {
			return nil, fmt.Errorf("duplicate skill id %q", skill.ID)
		}
		seen[skill.ID] = true
		resolved, err := l.resolveSkill(skill)
		if err != nil {
			return nil, err
		}
		out = append(out, resolved)
	}
	return out, nil
}

func (l *Loader) resolveSkill(skill ManifestSkill) (model.CatalogSkill, error) {
	if skill.Path == "" {
		return model.CatalogSkill{}, fmt.Errorf("skill %q path is required", skill.ID)
	}
	cleaned := filepath.Clean(skill.Path)
	if filepath.IsAbs(cleaned) {
		return model.CatalogSkill{}, fmt.Errorf("skill %q path must be relative", skill.ID)
	}
	joined := filepath.Join(l.Root, cleaned)
	if rel, err := filepath.Rel(l.Root, joined); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return model.CatalogSkill{}, fmt.Errorf("skill %q path escapes repo root", skill.ID)
	}
	if _, err := os.Stat(filepath.Join(joined, "SKILL.md")); err != nil {
		return model.CatalogSkill{}, fmt.Errorf("skill %q missing SKILL.md", skill.ID)
	}
	cs := model.CatalogSkill{ID: skill.ID, Name: skill.Name, Description: skill.Description, Source: &model.Source{Path: cleaned}}
	return cs, nil
}

func (l *Loader) skillFromDir(id, relDir string) model.CatalogSkill {
	return model.CatalogSkill{ID: id, Name: id, Source: &model.Source{Path: relDir}}
}

func excludedDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".skills", ".opencode":
		return true
	default:
		return false
	}
}

func hasExcludedPrefix(relDir string) bool {
	parts := strings.Split(relDir, string(filepath.Separator))
	for _, part := range parts {
		if excludedDir(part) {
			return true
		}
	}
	return false
}

func (w *Workspace) ParseSource(raw string) (*ParsedSource, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("source is required")
	}
	if strings.HasPrefix(raw, "github:") {
		return parseHostedSource("github", strings.TrimPrefix(raw, "github:"), 2, 2)
	}
	if strings.HasPrefix(raw, "gitlab:") {
		return parseHostedSource("gitlab", strings.TrimPrefix(raw, "gitlab:"), 2, 0)
	}
	if strings.Contains(raw, ":") && !strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "./") && !strings.HasPrefix(raw, "../") {
		return nil, fmt.Errorf("unsupported source: %s", raw)
	}
	return &ParsedSource{Kind: "local", Source: model.Source{Path: filepath.Clean(raw)}}, nil
}

func (w *Workspace) ResolveSource(raw string) (model.Source, error) {
	parsed, err := w.ParseSource(raw)
	if err != nil {
		return model.Source{}, err
	}
	return parsed.Source, nil
}

func parseHostedSource(kind, value string, minSegments int, maxSegments int) (*ParsedSource, error) {
	parts := splitHostedPath(value)
	if len(parts) < minSegments {
		return nil, fmt.Errorf("invalid %s source", kind)
	}
	if maxSegments > 0 && len(parts) > maxSegments {
		return nil, fmt.Errorf("invalid %s source", kind)
	}
	return &ParsedSource{Kind: kind, Source: model.Source{Locator: fmt.Sprintf("%s:%s", kind, strings.Join(parts, "/"))}}, nil
}

func splitHostedPath(value string) []string {
	raw := strings.Trim(value, "/")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

var skillIDRe = regexp.MustCompile(`^[a-z0-9_-]+$`)


func parseManifest(data []byte) (*Manifest, error) {
	var raw struct {
		Version *int `yaml:"version"`
		Catalog struct {
			Skills []ManifestSkill `yaml:"skills"`
		} `yaml:"catalog"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	m := &Manifest{Version: 1, Catalog: ManifestCatalog{Skills: raw.Catalog.Skills}}
	if raw.Version != nil {
		m.Version = *raw.Version
	}
	return m, nil
}
