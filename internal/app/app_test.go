package app

import (
	"os"
	"path/filepath"
	"os/exec"
	"strings"
	"testing"

	"github.com/tassis/spick/internal/config"
	"github.com/tassis/spick/internal/skills"
	"github.com/tassis/spick/internal/ui"
	"github.com/tassis/spick/internal/workspace"
)

func TestInspectLocalSource(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: demo\n      path: .\n")
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

func TestInspectHostedRefAccepted(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	a := New(nil, workspace.New(t.TempDir()), nil)
	got, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo"), Ref: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Skills) != 1 || got.Skills[0].ID != "demo" {
		t.Fatalf("unexpected hosted inspect result: %+v", got)
	}
}

func TestInspectRejectsLocalRef(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(nil, workspace.New(root), nil)
	_, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Ref: "main"})
	if err == nil || !strings.Contains(err.Error(), "ref is not yet supported for local sources") {
		t.Fatalf("expected local ref error, got %v", err)
	}
}

func TestInspectHostedMissingRefFailsClearly(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	a := New(nil, workspace.New(t.TempDir()), nil)
	_, err := a.Inspect(InspectOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo"), Ref: "missing"})
	if err == nil || !strings.Contains(err.Error(), "hosted ref") {
		t.Fatalf("expected hosted ref error, got %v", err)
	}
}

type fakePrompter struct{ multi []int }

func (f fakePrompter) Select(title string, options []ui.Option, defaultIndex int) (int, error) { return defaultIndex, nil }
func (f fakePrompter) MultiSelect(title string, options []ui.Option, defaults []int) ([]int, error) { return f.multi, nil }

func TestAddSingleSkillAutoSelect(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(got.Selected) != 1 || got.Selected[0].ID != "demo" { t.Fatalf("unexpected add result: %+v", got) }
}

func TestAddAllSelectsEverySkill(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), All: true})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(got.Selected) != 2 { t.Fatalf("expected all selected, got %+v", got) }
}

func TestAddResultMessageIsBrief(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if strings.Contains(got.Message, "install") { t.Fatalf("message too verbose: %+v", got) }
}

func TestAddExplicitSkillSelection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Skills: []string{"two"}})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(got.Selected) != 1 || got.Selected[0].ID != "two" { t.Fatalf("unexpected add result: %+v", got) }
}

func TestAddUnknownSkillErrors(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: one\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Skills: []string{"missing"}}); err == nil { t.Fatal("expected unknown skill error") }
}

func TestAddRejectsUnsupportedAgent(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: one\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Agent: "foo"}); err == nil || !strings.Contains(err.Error(), "unsupported agent") { t.Fatalf("expected agent error, got %v", err) }
}

func TestAddRejectsUnsupportedMode(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: one\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Mode: "link"}); err == nil || !strings.Contains(err.Error(), "unsupported mode") { t.Fatalf("expected mode error, got %v", err) }
}

func TestAddRejectsVersionAndLocalRef(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: one\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Version: "1.2.3"}); err == nil || !strings.Contains(err.Error(), "version is not yet supported") { t.Fatalf("expected version error, got %v", err) }
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), Ref: "main"}); err == nil || !strings.Contains(err.Error(), "ref is not yet supported for local sources") { t.Fatalf("expected ref error, got %v", err) }
}

func TestAddPromptsForMultiSkillSelection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{multi: []int{1}}, workspace.New(root), nil)
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(got.Selected) != 1 || got.Selected[0].ID != "two" { t.Fatalf("unexpected add result: %+v", got) }
}

func TestAddHostedRefAccepted(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	root := t.TempDir()
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo"), Ref: "main"})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(got.Selected) != 1 || got.Selected[0].ID != "demo" { t.Fatalf("unexpected hosted add result: %+v", got) }
	data, err := os.ReadFile(filepath.Join(root, "spick.lock"))
	if err != nil { t.Fatalf("expected lockfile: %v", err) }
	lockText := string(data)
	if !strings.Contains(lockText, `"locator": "github:owner/repo"`) || !strings.Contains(lockText, `"requestedVersion": "main"`) || !strings.Contains(lockText, `"path": "."`) {
		t.Fatalf("unexpected lockfile contents: %s", lockText)
	}
}

func TestAddHostedMissingRefFailsClearly(t *testing.T) {
	base := t.TempDir()
	createHostedRepo(t, base, "owner/repo.git")
	t.Setenv("SPICK_GIT_BASE_URL", "file://"+base)
	a := New(fakePrompter{}, workspace.New(t.TempDir()), nil)
	_, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator("github:owner/repo"), Ref: "missing"})
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
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	got, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if got.Message == "" { t.Fatal("expected success message") }
	if _, err := os.Stat(filepath.Join(root, "spick.lock")); err != nil { t.Fatal(err) }
}

func TestListMissingLockfileReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	a := New(fakePrompter{}, workspace.New(root), nil)
	got, err := a.List(ListOptions{Scope: config.ScopeProject})
	if err != nil { t.Fatal(err) }
	if len(got.Skills) != 0 { t.Fatalf("expected empty list, got %+v", got) }
}

func TestRemoveMissingStateReturnsFriendlyError(t *testing.T) {
	root := t.TempDir()
	a := New(fakePrompter{}, workspace.New(root), nil)
	if _, err := a.Remove(RemoveOptions{Scope: config.ScopeProject, Skills: []string{"demo"}}); err == nil || !strings.Contains(err.Error(), "no installed skills") {
		t.Fatalf("expected friendly error, got %v", err)
	}
}

func TestRemoveDefaultPreservesCanonical(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)}); err != nil { t.Fatal(err) }
	if _, err := a.Remove(RemoveOptions{Scope: config.ScopeProject, Skills: []string{"demo"}}); err != nil { t.Fatal(err) }
	if info, err := os.Lstat(filepath.Join(root, ".skills", "demo")); err != nil || info.IsDir() == false { t.Fatalf("canonical should remain as copied dir: %v %v", info, err) }
}

func TestRemovePurgeRemovesCanonical(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: demo\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root)}); err != nil { t.Fatal(err) }
	if _, err := a.Remove(RemoveOptions{Scope: config.ScopeProject, Skills: []string{"demo"}, Purge: true}); err != nil { t.Fatal(err) }
	if _, err := os.Stat(filepath.Join(root, ".skills", "demo")); !os.IsNotExist(err) { t.Fatalf("expected canonical removed, got %v", err) }
}

func TestRemovePruneUnusedRemovesOrphans(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root+"/spick.yaml", "catalog:\n  skills:\n    - id: one\n      path: .\n    - id: two\n      path: .\n")
	writeTestFile(t, root+"/SKILL.md", "# demo\n")
	a := New(fakePrompter{}, workspace.New(root), skills.New(root))
	if _, err := a.Add(AddOptions{Scope: config.ScopeProject, Source: SourceFromLocator(root), All: true}); err != nil { t.Fatal(err) }
	_ = os.RemoveAll(filepath.Join(root, ".opencode", "skills", "two"))
	if _, err := a.Remove(RemoveOptions{Scope: config.ScopeProject, Skills: []string{"one"}, PruneUnused: true}); err != nil { t.Fatal(err) }
	if _, err := os.Stat(filepath.Join(root, ".skills", "two")); !os.IsNotExist(err) { t.Fatalf("expected orphan pruned, got %v", err) }
	if _, err := os.Stat(filepath.Join(root, ".skills", "one")); !os.IsNotExist(err) { t.Fatalf("expected removed canonical pruned, got %v", err) }
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
	if err := os.MkdirAll(work, 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(repo, 0o755); err != nil { t.Fatal(err) }
	writeTestFile(t, filepath.Join(work, "spick.yaml"), "catalog:\n  skills:\n    - id: demo\n      path: .\n")
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
	if out, err := cmd.CombinedOutput(); err != nil { t.Fatalf("%s %v: %v: %s", name, args, err, string(out)) }
}
