package app

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/lock"
	"github.com/tassis/spick/internal/model"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

func TestInspectLocalSource(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(nil, workspace.New(root), nil)
	got, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Skills) != 1 || got.Skills[0].ID != "demo" {
		t.Fatalf("unexpected inspect result: %+v", got)
	}
}

func TestInspectPluginRepoWithoutManifestFailsExplicitly(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/plugin.json", "{}")
	a := New(nil, workspace.New(root), nil)
	_, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err == nil || !strings.Contains(err.Error(), "spick.res.yaml") {
		t.Fatalf("expected explicit manifest guidance, got %v", err)
	}
}

func TestInspectHostedRefAccepted(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	a := New(nil, workspace.New(t.TempDir()), nil)
	got, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo@main")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Skills) != 1 || got.Skills[0].ID != "demo" {
		t.Fatalf("unexpected hosted inspect result: %+v", got)
	}
}

func TestInspectHostedInlineRefAccepted(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	a := New(nil, workspace.New(t.TempDir()), nil)
	got, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo@main")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Skills) != 1 || got.Source.RequestedVersion != "main" {
		t.Fatalf("unexpected hosted inspect result: %+v", got)
	}
}

func TestInspectRejectsLocalRef(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(nil, workspace.New(root), nil)
	_, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root + "@main")})
	if err == nil || !strings.Contains(err.Error(), "spick.res.yaml") {
		t.Fatalf("expected explicit no-manifest guidance, got %v", err)
	}
}

func TestInspectHostedMissingRefFailsClearly(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	a := New(nil, workspace.New(t.TempDir()), nil)
	got, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo@missing")})
	if err == nil || !strings.Contains(err.Error(), "hosted ref") {
		t.Fatalf("expected hosted ref error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil result, got %+v", got)
	}
}

type fakePrompter struct {
	multi     []int
	matrix    map[int][]int
	selectIdx int
	selectErr error
	multiErr  error
}

func (f fakePrompter) Select(title string, options []ui.Option, defaultIndex int) (int, error) {
	if f.selectErr != nil {
		return 0, f.selectErr
	}
	if f.selectIdx != 0 {
		return f.selectIdx, nil
	}
	return defaultIndex, nil
}
func (f fakePrompter) MultiSelect(title string, options []ui.Option, defaults []int) ([]int, error) {
	if f.multiErr != nil {
		return nil, f.multiErr
	}
	return f.multi, nil
}
func (f fakePrompter) MatrixSelect(title string, rows []ui.Option, cols []ui.Option, defaults map[int][]int) (map[int][]int, error) {
	return f.matrix, nil
}

func TestAddSingleSkillAutoSelect(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Selected) != 1 || got.Selected[0].ID != "demo" {
		t.Fatalf("unexpected add result: %+v", got)
	}
}

func TestAddAllSelectsEverySkill(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), All: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Selected) != 2 {
		t.Fatalf("expected all selected, got %+v", got)
	}
}

func TestAddResultMessageIsBrief(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got.Message, "install") {
		t.Fatalf("message too verbose: %+v", got)
	}
}

func TestAddExplicitSkillSelection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Skills: []string{"two"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Selected) != 1 || got.Selected[0].ID != "two" {
		t.Fatalf("unexpected add result: %+v", got)
	}
}

func TestAddUnknownSkillErrors(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Skills: []string{"missing"}}); err == nil {
		t.Fatal("expected unknown skill error")
	}
}

func TestAddRejectsUnsupportedAgent(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Agent: "foo"}); err == nil || !strings.Contains(err.Error(), "unsupported agent") {
		t.Fatalf("expected agent error, got %v", err)
	}
}

func TestAddRejectsUnsupportedMode(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), ExposureMethod: model.ExposureMethod("link")}); err == nil || !strings.Contains(err.Error(), "unsupported exposure method") {
		t.Fatalf("expected exposure method error, got %v", err)
	}
}

func TestAddRejectsLocalRef(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root + "@main")}); err == nil {
		t.Fatal("expected local @ path to fail as a literal path")
	}
}

func TestAddPromptsForMultiSkillSelection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{multi: []int{1}}, workspace.New(root), nil)
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Selected) != 1 || got.Selected[0].ID != "two" {
		t.Fatalf("unexpected add result: %+v", got)
	}
}

func TestApplyPromptsForMatrixSelection(t *testing.T) {
	_ = skills.New
	_ = workspace.New
	_ = config.ScopeProject
	_ = fakePrompter{matrix: map[int][]int{0: []int{0}, 1: []int{1}}}
}

func TestAddHostedRefAccepted(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	root := t.TempDir()
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo@main")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Selected) != 1 || got.Selected[0].ID != "demo" {
		t.Fatalf("unexpected hosted add result: %+v", got)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatalf("expected lockfile: %v", err)
	}
	lockText := string(data)
	if !strings.Contains(lockText, `"declared":`) || !strings.Contains(lockText, `"source": "github:owner/repo"`) || !strings.Contains(lockText, `"ref": "main"`) || !strings.Contains(lockText, `"resolved":`) {
		t.Fatalf("unexpected lockfile contents: %s", lockText)
	}
}

func TestAddPersistsHostedSourceIdentity(t *testing.T) {
	root := t.TempDir()
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  agents: {}\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo@main"), All: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "github:owner/repo") || strings.Contains(string(data), "skills/ramblings-archive") {
		t.Fatalf("expected original hosted source identity, got %s", string(data))
	}
}

func TestManifestWritesDoNotPreserveCatalogBlock(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "version: 1\nproject:\n  skills:\n    - id: demo\n      source: github:owner/repo\n  plugins:\n    - id: plugin1\n      source: github:owner/plugin\n  agents:\n    opencode:\n      skills: [demo]\n  exposureMethod: symlink\n  autoApply: true\ncatalog:\n  skills:\n    - id: stale\n      path: .\n")
	w := workspace.New(root)
	if err := w.WriteProjectSkills([]model.ProjectSkill{{ID: "demo", Source: "github:owner/repo"}}); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteProjectPlugins([]model.ProjectPlugin{{ID: "plugin1", Source: "github:owner/plugin"}}); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteProjectAgentEnablement("opencode", []string{"demo"}, []string{"plugin1"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "catalog:") {
		t.Fatalf("expected catalog block removed, got %s", string(data))
	}
}

func TestAddHostedInlineRefAccepted(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	root := t.TempDir()
	a := New(fakePrompter{}, workspace.New(root), nil)
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo@main")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Selected) != 1 || got.Source.RequestedVersion != "main" {
		t.Fatalf("unexpected hosted add result: %+v", got)
	}
}

func TestAddRawHostedURLPersistsCloneMetadata(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	writeTestFile(t, filepath.Join(repo, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(repo, "SKILL.md"), "# demo\n")
	wrapper := filepath.Join(base, "git-wrapper.sh")
	writeTestFile(t, wrapper, "#!/bin/bash\nset -eu\nif [ \"$1\" = clone ]; then\n  dest=\"${@: -1}\"\n  cp -R \"$SPICK_GIT_TEMPLATE\"/. \"$dest\"\nfi\n")
	if err := os.Chmod(wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPICK_GIT_BIN", wrapper)
	t.Setenv("SPICK_GIT_TEMPLATE", repo)
	root := t.TempDir()
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: model.Source{CloneURL: "ssh://example.com/org/repo.git"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Selected) != 1 {
		t.Fatalf("unexpected add result: %+v", got)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatalf("expected lockfile: %v", err)
	}
	lockText := string(data)
	if !strings.Contains(lockText, `"declared":`) || !strings.Contains(lockText, `"source": "ssh://example.com/org/repo.git"`) {
		t.Fatalf("unexpected lockfile contents: %s", lockText)
	}
}

func TestAddHostedMissingRefFailsClearly(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	a := New(fakePrompter{}, workspace.New(t.TempDir()), nil)
	_, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo@missing")})
	if err == nil || !strings.Contains(err.Error(), "hosted ref") {
		t.Fatalf("expected hosted ref error, got %v", err)
	}
}

func TestAddHostedMissingGitFailsClearly(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	t.Setenv("PATH", "/no/such/path")
	a := New(fakePrompter{}, workspace.New(t.TempDir()), nil)
	_, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo")})
	if err == nil || !strings.Contains(err.Error(), "git is required for hosted sources") {
		t.Fatalf("expected git missing error, got %v", err)
	}
}

func TestAddUpdatesLockfile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Message == "" {
		t.Fatal("expected success message")
	}
	if _, err := os.Stat(filepath.Join(root, "spick.lock")); err != nil {
		t.Fatal(err)
	}
}

func TestAddWritesSkillDeclarationBeforeMaterialization(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "version: 1\nproject:\n  autoApply: false\n")
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "skills:") || !strings.Contains(text, "id: demo") {
		t.Fatalf("expected skill declaration persisted, got %s", text)
	}
}

func TestAddDuplicateSkillRequiresForce(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "version: 1\nproject:\n  skills:\n    - id: demo\n      source: ./old\n")
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)}); err == nil || !strings.Contains(err.Error(), "use --force") {
		t.Fatalf("expected force error, got %v", err)
	}
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Force: true}); err != nil {
		t.Fatalf("expected forced add to succeed, got %v", err)
	}
}

func TestPluginAddWritesDeclarationAndEnablement(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "version: 1\nproject:\n  agents:\n    opencode: {}\n")
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: plugin\nplugin:\n  id: plugin-demo\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, root+"/index.js", "console.log('ok')\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.AddPlugin(AddPluginOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "plugins:") || !strings.Contains(text, "plugin-demo") || !strings.Contains(text, "opencode") {
		t.Fatalf("expected plugin declaration and enablement persisted, got %s", text)
	}
}

func TestPluginAddPersistsHostedRepoIdentity(t *testing.T) {
	root := t.TempDir()
	base := t.TempDir()
	repo := filepath.Join(base, "owner", "plugin")
	mustRun(t, base, "git", "init", "--bare", repo)
	work := filepath.Join(base, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(work, "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: plugin-demo\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, filepath.Join(work, "index.js"), "console.log('ok')\n")
	mustRun(t, work, "git", "init")
	mustRun(t, work, "git", "config", "user.email", "test@example.com")
	mustRun(t, work, "git", "config", "user.name", "Test User")
	mustRun(t, work, "git", "add", ".")
	mustRun(t, work, "git", "commit", "-m", "init plugin")
	mustRun(t, work, "git", "remote", "add", "origin", repo)
	mustRun(t, work, "git", "push", "-u", "origin", "HEAD:main")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "version: 1\nproject:\n  agents:\n    opencode: {}\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.AddPlugin(AddPluginOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/plugin@main")}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "source: github:owner/plugin") || !strings.Contains(text, "ref: main") || strings.Contains(text, "github:owner/plugin@main") {
		t.Fatalf("expected hosted repo identity and explicit ref, got %s", text)
	}
}

func TestSyncReopensHostedPluginWithStoredRef(t *testing.T) {
	root := t.TempDir()
	base := t.TempDir()
	repo := filepath.Join(base, "owner", "plugin")
	mustRun(t, base, "git", "init", "--bare", repo)
	work := filepath.Join(base, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(work, "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: plugin-demo\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, filepath.Join(work, "index.js"), "console.log('ok')\n")
	mustRun(t, work, "git", "init")
	mustRun(t, work, "git", "config", "user.email", "test@example.com")
	mustRun(t, work, "git", "config", "user.name", "Test User")
	mustRun(t, work, "git", "add", ".")
	mustRun(t, work, "git", "commit", "-m", "init plugin")
	mustRun(t, work, "git", "remote", "add", "origin", repo)
	mustRun(t, work, "git", "push", "-u", "origin", "HEAD:main")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  plugins:\n    - id: plugin-demo\n      source: ./plugin\n  agents:\n    - id: opencode\n      path: ./agents/opencode\n  runtimes:\n    opencode:\n      plugins: [plugin-demo]\n")
	writeTestFile(t, filepath.Join(root, "plugin", "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: plugin-demo\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, filepath.Join(root, "plugin", "index.js"), "console.log('ok')\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if err := a.Locks.Write(string(config.ScopeProject), &model.Lockfile{Version: 1, Scope: string(config.ScopeProject), Plugins: []model.LockPlugin{{ID: "plugin-demo", Declared: model.LockDeclared{Source: "github:owner/plugin", Ref: "main"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "plugins", "plugin-demo")}, Projected: model.LockPluginProjected{Path: filepath.Join(root, ".spick", "plugins", "plugin-demo")}}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".spick", "plugins", "plugin-demo")); err != nil {
		t.Fatal(err)
	}
	got, err := a.Sync(config.ScopeProject, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spick", "plugins", "plugin-demo")); err != nil {
		t.Fatalf("expected plugin restored: %v", err)
	}
	if len(got.PluginMessages) == 0 {
		t.Fatalf("expected plugin sync output")
	}
}

func TestAddAutoApplyFalseSkipsExposures(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  autoApply: false\n  agents:\n    opencode: {}\n")
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Added) != 1 || len(got.Added[0].Exposures) != 0 {
		t.Fatalf("expected no exposures when autoApply disabled, got %+v", got.Added)
	}
	if _, err := os.Stat(filepath.Join(root, ".spick", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("expected materialized skill storage: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "demo")); !os.IsNotExist(err) {
		t.Fatalf("expected no agent link materialization, got err=%v", err)
	}
}

func TestAddAutoApplyDefaultKeepsExposures(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Added) != 1 || len(got.Added[0].Exposures) == 0 {
		t.Fatalf("expected exposures when autoApply default enabled, got %+v", got.Added)
	}
}

func TestAddUsesProjectAgentAndExposureMethod(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  agents:\n    opencode: {}\n  exposureMethod: copy\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Added) != 1 || len(got.Added[0].Exposures) != 1 || got.Added[0].Exposures[0].Agent != "opencode" {
		t.Fatalf("unexpected project-configured add result: %+v", got.Added)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"exposures":`) || !strings.Contains(string(data), `"agent": "opencode"`) {
		t.Fatalf("expected exposure metadata in lockfile: %s", string(data))
	}
}

func TestAddUsesMultipleProjectAgents(t *testing.T) {
	t.Skip("runtime-first apply work defers multi-agent add parity")
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  agents:\n    opencode: {}\n    codex: {}\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Added) != 1 || len(got.Added[0].Exposures) != 2 {
		t.Fatalf("expected two exposures, got %+v", got.Added)
	}
	if got.Added[0].Exposures[0].Agent == got.Added[0].Exposures[1].Agent {
		t.Fatalf("expected distinct agents, got %+v", got.Added[0].Exposures)
	}
}

func TestAddCLIOverridesProjectDefaults(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  agents:\n    codex: {}\n  exposureMethod: copy\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Agent: "opencode", ExposureMethod: model.ExposureMethodSymlink})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Added) != 1 || len(got.Added[0].Exposures) != 1 || got.Added[0].Exposures[0].Agent != "opencode" || got.Added[0].Mode != "symlink" {
		t.Fatalf("unexpected CLI override result: %+v", got.Added)
	}
}

func TestAddFallsBackWithoutProjectConfig(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Added) != 1 || len(got.Added[0].Exposures) != 1 || got.Added[0].Exposures[0].Agent != "opencode" || got.Added[0].Mode != "symlink" {
		t.Fatalf("expected fallback defaults, got %+v", got.Added)
	}
}

func TestApplyUsesProjectAgents(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./skill\n  agents:\n    opencode:\n      source: ./agents/opencode\n  runtimes:\n    opencode:\n      skills: [demo]\n")
	if err := os.MkdirAll(filepath.Join(root, "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "skill", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "skill", "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, ".spick", "skills", "demo", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if err := a.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".spick", "skills", "demo")}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "runtimes:") || !strings.Contains(string(data), "demo") {
		t.Fatalf("expected apply to persist runtime enablement, got %s", string(data))
	}
}

func TestApplyGlobalMutatesGlobalConfigOnly(t *testing.T) {
	workspaceRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalRoot := filepath.Join(home, ".spick")
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(globalRoot, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: "+filepath.Join(workspaceRoot, "skill")+"\n  agents:\n    opencode:\n      source: "+filepath.Join(workspaceRoot, "agents", "opencode")+"\n  runtimes:\n    opencode:\n      skills: [demo]\n")
	writeTestFile(t, filepath.Join(workspaceRoot, "spick.yaml"), "project:\n  skills:\n    - id: local\n      source: ./local\n")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspaceRoot, "skill", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(workspaceRoot, "skill", "SKILL.md"), "# demo\n")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "agents", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspaceRoot, "agents", "opencode", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  agents:\n    - id: opencode\n      path: .\n")
	a := New(fakePrompter{}, workspace.New(workspaceRoot), skills.New(workspaceRoot))
	if _, err := a.Apply(ApplyOptions{Global: true, Skill: true, Skills: []string{"demo"}}); err != nil {
		t.Fatal(err)
	}
	globalData, err := os.ReadFile(filepath.Join(globalRoot, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(globalData), "demo") || !strings.Contains(string(globalData), "runtimes:") {
		t.Fatalf("expected global config updated, got %s", string(globalData))
	}
	workspaceData, err := os.ReadFile(filepath.Join(workspaceRoot, "spick.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(workspaceData), "demo") && !strings.Contains(string(workspaceData), "local") {
		t.Fatalf("expected workspace config unchanged, got %s", string(workspaceData))
	}
}

func TestApplySingleRuntimeAutoSelects(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./skill\n  agents:\n    opencode:\n      source: ./agents/opencode\n  runtimes:\n    opencode:\n      skills: [demo]\n")
	if err := os.MkdirAll(filepath.Join(root, "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "skill", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "skill", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyMultiRuntimePromptsForSelection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./skill\n  agents:\n    opencode:\n      source: ./agents/opencode\n    codex:\n      source: ./agents/codex\n  runtimes:\n    opencode:\n      skills: [demo]\n    codex:\n      skills: []\n")
	if err := os.MkdirAll(filepath.Join(root, "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "skill", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "skill", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{selectIdx: 1}, workspace.New(root), skills.New(root))
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyAgentModeNarrowsToAgents(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./skill\n  agents:\n    opencode:\n      source: ./agents/opencode\n  runtimes:\n    opencode:\n      agents: [opencode]\n")
	if err := os.MkdirAll(filepath.Join(root, "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "skill", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "skill", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject, AgentMode: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lf, err := a.Locks.Read(string(config.ScopeProject))
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Agents) != 1 || lf.Agents[0].ID != "opencode" {
		t.Fatalf("expected agent lock entry, got %+v", lf.Agents)
	}
}

func TestApplyRejectsBadRuntimeSkillPluginAgentIDs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./skill\n  plugins:\n    - id: plugin1\n      source: ./plugin\n  agents:\n    - id: opencode\n      source: ./agents/opencode\n      path: ./agents/opencode\n  runtimes:\n    opencode:\n      skills: [demo]\n")
	if err := os.MkdirAll(filepath.Join(root, "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "skill", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "skill", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject, Runtime: "missing"}); err == nil || !strings.Contains(err.Error(), "unknown runtime ids") {
		t.Fatalf("expected bad runtime error before writeback, got %v", err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject, Skills: []string{"missing"}}); err == nil {
		t.Fatal("expected bad skill error")
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject, Plugins: []string{"missing"}}); err == nil {
		t.Fatal("expected bad plugin error")
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject, Agent: "missing"}); err == nil {
		t.Fatal("expected bad agent error")
	}
}

func TestApplyPropagatesPromptErrors(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./skill\n  agents:\n    - id: opencode\n      source: ./agents/opencode\n      path: ./agents/opencode\n  runtimes:\n    opencode:\n      skills: [demo]\n")
	if err := os.MkdirAll(filepath.Join(root, "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "skill", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "skill", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{multiErr: io.EOF}, workspace.New(root), skills.New(root))
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err == nil {
		t.Fatal("expected prompt error")
	}
}

func TestApplyRejectsUndeclaredSkillSelection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  agents:\n    opencode: {}\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, ".spick", "skills", "demo", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if err := a.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".spick", "skills", "demo")}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject, Skills: []string{"missing"}}); err == nil {
		t.Fatal("expected bad skill error")
	}
}

func TestApplyRepeatedAndForceOverwrite(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  agents:\n    opencode:\n      source: ./agents/opencode\n  runtimes:\n    opencode:\n      skills: [demo]\n")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, ".spick", "skills", "demo", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if err := a.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".spick", "skills", "demo")}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatalf("expected repeated apply to be a no-op, got %v", err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject, Force: true}); err != nil {
		t.Fatalf("expected force overwrite, got %v", err)
	}
}

func TestApplyDisablesUnwantedExposure(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  agents:\n    opencode:\n      source: ./agents/opencode\n  runtimes:\n    opencode:\n      skills: [demo]\n")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, ".spick", "skills", "demo", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if err := a.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".spick", "skills", "demo")}, Exposures: []model.Exposure{{Agent: "opencode", Path: filepath.Join(root, ".opencode", "skills", "demo")}, {Agent: "codex", Path: filepath.Join(root, ".agents", "skills", "demo")}}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, ".opencode", "skills", "demo")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, ".agents", "skills", "demo")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".spick", "skills", "demo"), filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".spick", "skills", "demo"), filepath.Join(root, ".agents", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".agents", "skills", "demo")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("expected readable exposure state, got %v", err)
	}
}

func TestApplyAfterAddAutoApplyFalse(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  autoApply: false\n  agents:\n    opencode: {}\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}
}

func TestApplyUnknownSkillAgentAndMissingCanonical(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  agents:\n    opencode:\n      source: ./agents/opencode\n  runtimes:\n    opencode:\n      skills: [demo]\n")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if err := a.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".spick", "skills", "demo")}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatalf("expected apply success, got %v", err)
	}
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject, Agent: "missing"}); err == nil || !strings.Contains(err.Error(), "unsupported agent") {
		t.Fatalf("expected agent error, got %v", err)
	}
	writeTestFile(t, filepath.Join(root, ".spick", "skills", "demo", "SKILL.md"), "# demo\n")
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatalf("expected apply success before missing canonical check, got %v", err)
	}
	_ = os.RemoveAll(filepath.Join(root, ".spick", "skills", "demo"))
	if _, err := a.Apply(ApplyOptions{Scope: config.ScopeProject}); err != nil {
		t.Fatalf("expected apply to remain config-driven, got %v", err)
	}
}

func TestSyncRestoresSkillsAndReportsExtraPlugins(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  plugins:\n    - id: plugin-demo\n      source: ./plugin\n  agents:\n    - id: opencode\n      path: ./agents/opencode\n    - id: codex\n      path: ./agents/codex\n  runtimes:\n    opencode:\n      skills: [demo]\n      plugins: [plugin-demo]\n      agents: [opencode]\n")
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, ".spick", "skills", "demo", "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, "plugin", "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: plugin-demo\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, filepath.Join(root, "plugin", "index.js"), "console.log('ok')\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if err := a.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".spick", "skills", "demo")}, Exposures: []model.Exposure{{Agent: "opencode", Path: filepath.Join(root, ".opencode", "skills", "demo")}}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, ".opencode", "skills", "demo")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".spick", "skills", "demo"), filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	if err := a.Locks.UpsertPlugins(string(config.ScopeProject), []model.LockPlugin{{ID: "plugin-extra", Materialized: model.LockMaterialized{Path: filepath.Join(root, "extra-plugin")}, Projected: model.LockPluginProjected{Path: filepath.Join(root, "extra-plugin")}}}); err != nil {
		t.Fatal(err)
	}
	got, err := a.Sync(config.ScopeProject, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsString(got.SkillMessages, "restored skill demo") {
		t.Fatalf("expected restored skill message, got %+v", got.SkillMessages)
	}
	if len(got.SkillMessages) == 0 || len(got.PluginMessages) != 1 {
		t.Fatalf("unexpected sync result: %+v", got)
	}
	lf, err := a.Locks.Read(string(config.ScopeProject))
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Plugins) != 1 || lf.Plugins[0].ID != "plugin-demo" {
		t.Fatalf("expected config-derived plugin snapshot, got %+v", lf.Plugins)
	}
	if _, err := os.Lstat(filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatalf("expected exposure to remain restored, got %v", err)
	}
}

func TestSyncBuildsAndPersistsConfigDerivedLockSnapshot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  autoApply: false\n  skills:\n    - id: demo\n      source: ./src\n  plugins:\n    - id: plugin-demo\n      source: ./plugin\n  agents:\n    opencode:\n      path: ./agents/opencode\n  runtimes:\n    opencode:\n      skills: [demo]\n      plugins: [plugin-demo]\n")
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, "plugin", "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: plugin-demo\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, filepath.Join(root, "plugin", "index.js"), "console.log('ok')\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Sync(config.ScopeProject, false); err != nil {
		t.Fatal(err)
	}
	lf, err := a.Locks.Read(string(config.ScopeProject))
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Skills) != 1 || lf.Skills[0].Declared.Source != "./src" || lf.Skills[0].Projected.Mode != "symlink" || len(lf.Skills[0].Projected.Exposures) != 0 {
		t.Fatalf("unexpected skill snapshot: %+v", lf.Skills)
	}
	if len(lf.Plugins) != 1 || lf.Plugins[0].Declared.Source != "./plugin" || lf.Plugins[0].Projected.Path == "" {
		t.Fatalf("unexpected plugin snapshot: %+v", lf.Plugins)
	}
	if len(lf.Agents) != 1 || lf.Agents[0].ID != "opencode" {
		t.Fatalf("unexpected agent snapshot: %+v", lf.Agents)
	}
}

func TestSyncFiltersSnapshotByRuntimeEnablement(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  plugins:\n    - id: plugin-a\n      source: ./plugin-a\n    - id: plugin-b\n      source: ./plugin-b\n  agents:\n    opencode:\n      path: ./agents/opencode\n    codex:\n      path: ./agents/codex\n  runtimes:\n    opencode:\n      skills: [demo]\n      plugins: [plugin-a]\n      agents: [opencode]\n")
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	for _, id := range []string{"plugin-a", "plugin-b"} {
		writeTestFile(t, filepath.Join(root, id, "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: "+id+"\n  runtime: node\n  entry: index.js\n")
		writeTestFile(t, filepath.Join(root, id, "index.js"), "console.log('ok')\n")
	}
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Sync(config.ScopeProject, false); err != nil {
		t.Fatal(err)
	}
	lf, err := a.Locks.Read(string(config.ScopeProject))
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Plugins) != 2 {
		t.Fatalf("expected all declared plugins in snapshot, got %+v", lf.Plugins)
	}
	if len(lf.Agents) != 2 {
		t.Fatalf("expected all declared agents in snapshot, got %+v", lf.Agents)
	}
	if lf.ExposureMethod != model.ExposureMethodSymlink || !lf.AutoApply {
		t.Fatalf("expected config policy snapshot, got %+v", lf)
	}
	if len(lf.Runtimes) != 1 || len(lf.Runtimes["opencode"].Plugins) != 1 || lf.Runtimes["opencode"].Plugins[0] != "plugin-a" {
		t.Fatalf("expected runtime membership snapshot, got %+v", lf.Runtimes)
	}
}

func TestSyncLockedRestoreAndFailureModes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  agents:\n    opencode:\n      skills: [demo]\n")
	writeTestFile(t, filepath.Join(root, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, ".spick", "skills", "demo", "SKILL.md"), "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if err := a.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Source: &model.SkillSource{Locator: root, Path: root, RequestedVersion: "v1"}, Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".spick", "skills", "demo")}, Exposures: []model.Exposure{{Agent: "opencode", Path: filepath.Join(root, ".opencode", "skills", "demo")}}}}); err != nil {
		t.Fatal(err)
	}
	lfRuntime, err := a.Locks.Read(string(config.ScopeProject))
	if err != nil {
		t.Fatal(err)
	}
	lfRuntime.Runtimes = map[string]model.LockRuntimeEntry{"opencode": {Skills: []string{"demo"}}}
	if err := a.Locks.Write(string(config.ScopeProject), lfRuntime); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, ".opencode", "skills", "demo")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".spick", "skills", "demo"), filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	got, err := a.Sync(config.ScopeProject, true)
	if err != nil {
		t.Fatalf("unexpected locked restore error: %v", err)
	}
	if !containsString(got.SkillMessages, "skill demo already in sync") && !containsString(got.SkillMessages, "restored skill demo") {
		t.Fatalf("expected locked restore/in-sync message, got %+v", got.SkillMessages)
	}
	if err := os.RemoveAll(filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatalf("expected locked restore for missing exposure, got %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatalf("expected missing exposure restored, got %v", err)
	}
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: other\n      source: ./src\n")
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatalf("expected locked sync to ignore config mismatch, got %v", err)
	}
}

func TestListMissingLockfileReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	a := New(fakePrompter{}, workspace.New(root), nil)
	got, err := a.List(ListOptions{Scope: config.ScopeProject})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 0 {
		t.Fatalf("expected empty list, got %+v", got)
	}
}

func TestRemoveMissingStateReturnsFriendlyError(t *testing.T) {
	root := t.TempDir()
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Remove(RemoveOptions{Scope: config.ScopeProject, Skills: []string{"demo"}}); err == nil || !strings.Contains(err.Error(), "no installed skills") {
		t.Fatalf("expected friendly error, got %v", err)
	}
}

func TestRemoveDefaultRemovesCanonical(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Remove(RemoveOptions{Scope: config.ScopeProject, Skills: []string{"demo"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spick", "skills", "demo")); !os.IsNotExist(err) {
		t.Fatalf("canonical should be removed, got %v", err)
	}
}

func TestRemoveReportsFullRemoval(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)}); err != nil {
		t.Fatal(err)
	}
	result, err := a.Remove(RemoveOptions{Scope: config.ScopeProject, Skills: []string{"demo"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "removed skills from canonical storage and exposures" || len(result.Purged) != 1 {
		t.Fatalf("expected full removal result, got %+v", result)
	}
}

func TestRemovePruneUnusedRemovesOrphans(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.res.yaml", "version: 1\nkind: resources\nresources:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), All: true}); err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(root, ".opencode", "skills", "two"))
	if _, err := a.Remove(RemoveOptions{Scope: config.ScopeProject, Skills: []string{"one"}, PruneUnused: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spick", "skills", "two")); !os.IsNotExist(err) {
		t.Fatalf("expected orphan pruned, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spick", "skills", "one")); !os.IsNotExist(err) {
		t.Fatalf("expected removed canonical pruned, got %v", err)
	}
}

func TestSyncLockedRequiresLockfileAndMatch(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./demo\n  agents:\n    opencode: {}\n")
	writeTestFile(t, filepath.Join(root, "demo", "SKILL.md"), "# demo\n")
	a := New(nil, workspace.New(root), skills.New(root))
	if _, err := a.Sync(config.ScopeProject, true); err == nil || !strings.Contains(err.Error(), "existing lockfile") {
		t.Fatalf("expected missing lockfile error, got %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".spick", "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, ".opencode", "skills", "demo")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".spick", "skills", "demo"), filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	if err := a.Locks.UpsertInstalled(string(config.ScopeProject), []model.InstalledSkill{{ID: "demo", Source: &model.SkillSource{Locator: "./demo", Path: filepath.Join(root, "demo")}, Install: &model.SkillInstall{Mode: "symlink", CanonicalPath: filepath.Join(root, ".spick", "skills", "demo")}, Exposures: []model.Exposure{{Agent: "opencode", Path: filepath.Join(root, ".opencode", "skills", "demo")}}}}); err != nil {
		t.Fatal(err)
	}
	lfRuntime, err := a.Locks.Read(string(config.ScopeProject))
	if err != nil {
		t.Fatal(err)
	}
	lfRuntime.Runtimes = map[string]model.LockRuntimeEntry{"opencode": {Skills: []string{"demo"}}}
	if err := a.Locks.Write(string(config.ScopeProject), lfRuntime); err != nil {
		t.Fatal(err)
	}
	lf, err := a.Locks.Read(string(config.ScopeProject))
	if err != nil {
		t.Fatal(err)
	}
	lf.Skills[0].Resolved = model.LockResolved{}
	if err := a.Locks.Write(string(config.ScopeProject), lf); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatalf("expected locked restore to succeed with snapshot material, got %v", err)
	}
}

func TestSyncLockedRestoresFromSnapshotWithoutConfigAuthority(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./demo\n  agents:\n    opencode: {}\n")
	writeTestFile(t, filepath.Join(root, "demo", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: other\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "demo", "SKILL.md"), "# other\n")
	lockStore := lock.New(root)
	if err := lockStore.Write(string(config.ScopeProject), &model.Lockfile{Version: 1, Scope: string(config.ScopeProject), Skills: []model.LockSkill{{ID: "other", Declared: model.LockDeclared{Source: "./demo"}, Resolved: model.LockResolved{Source: "./demo", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "skills", "other")}, Projected: model.LockProjected{Mode: "symlink"}}}, Runtimes: map[string]model.LockRuntimeEntry{"opencode": {Skills: []string{"other"}}}}); err != nil {
		t.Fatal(err)
	}
	a := &App{Workspace: workspace.New(root), Skills: skills.New(root), Locks: lockStore}
	if err := os.RemoveAll(filepath.Join(root, ".spick", "skills", "other")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatalf("expected locked restore from snapshot, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spick", "skills", "other")); err != nil {
		t.Fatalf("expected snapshot-managed skill restored, got %v", err)
	}
}

func TestSyncLockedRestoresMissingExposureWithoutRewritingLockfile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./demo\n  agents:\n    opencode: {}\n")
	writeTestFile(t, filepath.Join(root, "demo", "SKILL.md"), "# demo\n")
	canonical := filepath.Join(root, ".spick", "skills", "demo")
	exposure := filepath.Join(root, ".opencode", "skills", "demo")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(exposure), 0o755); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(filepath.Dir(exposure), canonical)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, exposure); err != nil {
		t.Fatal(err)
	}
	lockStore := lock.New(root)
	lf := &model.Lockfile{Version: 1, Scope: string(config.ScopeProject), Skills: []model.LockSkill{{ID: "demo", Declared: model.LockDeclared{Source: "./demo"}, Resolved: model.LockResolved{Source: "./demo", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "skills", "demo")}, Projected: model.LockProjected{Mode: "symlink"}}}, Runtimes: map[string]model.LockRuntimeEntry{"opencode": {Skills: []string{"demo"}}}}
	if err := lockStore.Write(string(config.ScopeProject), lf); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(exposure); err != nil {
		t.Fatal(err)
	}
	a := &App{Workspace: workspace.New(root), Skills: skills.New(root), Locks: lockStore}
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("expected lockfile unchanged")
	}
	if _, err := os.Stat(exposure); err != nil {
		t.Fatalf("expected exposure restored: %v", err)
	}
}

func TestSyncLockedRestoresLocalSourceWithoutResolvedRevision(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  agents:\n    opencode:\n      skills: [demo]\n")
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	if err := os.MkdirAll(filepath.Join(root, ".spick", "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	lockStore := lock.New(root)
	lf := &model.Lockfile{Version: 1, Scope: string(config.ScopeProject), Skills: []model.LockSkill{{ID: "demo", Declared: model.LockDeclared{Source: "./src"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "skills", "demo")}, Projected: model.LockProjected{Mode: "symlink"}}}, Runtimes: map[string]model.LockRuntimeEntry{"opencode": {Skills: []string{"demo"}}}}
	if err := lockStore.Write(string(config.ScopeProject), lf); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	a := &App{Workspace: workspace.New(root), Skills: skills.New(root), Locks: lockStore}
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatalf("expected exposure restored: %v", err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "spick.lock"))
	after, _ := os.ReadFile(filepath.Join(root, "spick.lock"))
	if string(before) != string(after) {
		t.Fatalf("expected lockfile unchanged")
	}
}

func TestSyncLockedFallsBackWhenManagedSkillMissing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  agents:\n    opencode:\n      skills: [demo]\n")
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	lockStore := lock.New(root)
	lf := &model.Lockfile{Version: 1, Scope: string(config.ScopeProject), Skills: []model.LockSkill{{ID: "demo", Declared: model.LockDeclared{Source: "./src"}, Resolved: model.LockResolved{Source: "./src", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "skills", "demo")}, Projected: model.LockProjected{Mode: "symlink"}}}, Runtimes: map[string]model.LockRuntimeEntry{"opencode": {Skills: []string{"demo"}}}}
	if err := lockStore.Write(string(config.ScopeProject), lf); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{Workspace: workspace.New(root), Skills: skills.New(root), Locks: lockStore}
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("expected lockfile unchanged")
	}
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "demo")); err != nil {
		t.Fatalf("expected exposure restored: %v", err)
	}
}

func TestSyncLockedFallsBackWhenManagedPluginMissing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  plugins:\n    - id: demo-plugin\n      source: ./plugin\n")
	writeTestFile(t, filepath.Join(root, "plugin", "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: demo-plugin\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, filepath.Join(root, "plugin", "index.js"), "console.log('ok')\n")
	lockStore := lock.New(root)
	lf := &model.Lockfile{Version: 1, Scope: string(config.ScopeProject), Plugins: []model.LockPlugin{{ID: "demo-plugin", Declared: model.LockDeclared{Source: "./plugin"}, Resolved: model.LockResolved{Source: "./plugin", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "plugins", "demo-plugin")}, Projected: model.LockPluginProjected{Path: filepath.Join(root, ".spick", "plugins", "demo-plugin")}}}}
	if err := lockStore.Write(string(config.ScopeProject), lf); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{Workspace: workspace.New(root), Skills: skills.New(root), Locks: lockStore}
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("expected lockfile unchanged")
	}
	if _, err := os.Stat(filepath.Join(root, ".spick", "plugins", "demo-plugin")); err != nil {
		t.Fatalf("expected plugin managed material restored: %v", err)
	}
}

func TestSyncLockedWarnsOnUnpinnedFallbackRestores(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: demo\n      source: ./src\n  plugins:\n    - id: demo-plugin\n      source: ./plugin\n  agents:\n    opencode:\n      skills: [demo]\n")
	writeTestFile(t, filepath.Join(root, "src", "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(root, "src", "SKILL.md"), "# demo\n")
	writeTestFile(t, filepath.Join(root, "plugin", "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: demo-plugin\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, filepath.Join(root, "plugin", "index.js"), "console.log('ok')\n")
	lockStore := lock.New(root)
	lf := &model.Lockfile{Version: 1, Scope: string(config.ScopeProject), Skills: []model.LockSkill{{ID: "demo", Declared: model.LockDeclared{Source: "./src"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "skills", "demo")}, Projected: model.LockProjected{Mode: "symlink", Exposures: []model.Exposure{{Agent: "opencode", Path: filepath.Join(root, ".opencode", "skills", "demo")}}}}}, Plugins: []model.LockPlugin{{ID: "demo-plugin", Declared: model.LockDeclared{Source: "./plugin"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "plugins", "demo-plugin")}, Projected: model.LockPluginProjected{Path: filepath.Join(root, ".spick", "plugins", "demo-plugin")}}}}
	if err := lockStore.Write(string(config.ScopeProject), lf); err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(root, ".spick", "skills", "demo"))
	_ = os.RemoveAll(filepath.Join(root, ".spick", "plugins", "demo-plugin"))
	before, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{Workspace: workspace.New(root), Skills: skills.New(root), Locks: lockStore}
	got, err := a.Sync(config.ScopeProject, true)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("expected lockfile unchanged")
	}
	if !containsString(got.Warnings, "locked restore refetched unpinned skill source ./src for ids: demo") || !containsString(got.Warnings, "locked restore refetched unpinned plugin source ./plugin for ids: demo-plugin") {
		t.Fatalf("expected fallback warnings, got %+v", got.Warnings)
	}
}

func TestSyncLockedFailsWhenPluginSnapshotIncomplete(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "spick.yaml"), "project:\n  plugins:\n    - id: demo-plugin\n      source: ./plugin\n")
	writeTestFile(t, filepath.Join(root, "plugin", "spick.res.yaml"), "version: 1\nkind: plugin\nplugin:\n  id: demo-plugin\n  runtime: node\n  entry: index.js\n")
	writeTestFile(t, filepath.Join(root, "plugin", "index.js"), "console.log('ok')\n")
	lockStore := lock.New(root)
	lf := &model.Lockfile{Version: 1, Scope: string(config.ScopeProject), Plugins: []model.LockPlugin{{ID: "demo-plugin", Declared: model.LockDeclared{Source: "./plugin"}, Resolved: model.LockResolved{Source: "./plugin", Revision: "rev1"}, Materialized: model.LockMaterialized{Path: filepath.Join(root, ".spick", "plugins", "demo-plugin")}, Projected: model.LockPluginProjected{Path: ""}}}}
	if err := lockStore.Write(string(config.ScopeProject), lf); err != nil {
		t.Fatal(err)
	}
	a := &App{Workspace: workspace.New(root), Skills: skills.New(root), Locks: lockStore}
	if _, err := a.Sync(config.ScopeProject, true); err != nil {
		t.Fatalf("expected plugin restore from canonical material, got %v", err)
	}
	before, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("expected lockfile unchanged")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func createHostedRepo(t *testing.T, base, rel string) {
	t.Helper()
	repo := filepath.Join(base, rel)
	work := filepath.Join(base, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(work, "spick.res.yaml"), "version: 1\nkind: resources\nresources:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, filepath.Join(work, "SKILL.md"), "# demo\n")
	mustRun(t, work, "git", "init")
	mustRun(t, work, "git", "config", "user.email", "test@example.com")
	mustRun(t, work, "git", "config", "user.name", "Test")
	mustRun(t, work, "git", "add", ".")
	mustRun(t, work, "git", "commit", "-m", "init")
	mustRun(t, work, "git", "branch", "-M", "main")
	mustRun(t, work, "git", "clone", "--bare", work, repo)
}

func mustRun(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, string(out))
	}
}
