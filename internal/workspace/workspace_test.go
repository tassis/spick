package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tassis/spick/internal/model"
)

import "strings"

func TestParseSourceGitHub(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("github:owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "github" || parsed.Source.Locator != "github:owner/repo" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceGitHubWithInlineRef(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("github:owner/repo@v1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "github" || parsed.Source.Locator != "github:owner/repo" || parsed.Source.RequestedVersion != "v1.2.3" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceGitLab(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("gitlab:group/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "gitlab" || parsed.Source.Locator != "gitlab:group/repo" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceGitLabWithInlineRef(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("gitlab:group/repo@main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "gitlab" || parsed.Source.Locator != "gitlab:group/repo" || parsed.Source.RequestedVersion != "main" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceGitLabSubgroup(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("gitlab:group/subgroup/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "gitlab" || parsed.Source.Locator != "gitlab:group/subgroup/repo" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceGitLabSubgroupWithInlineRef(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("gitlab:group/subgroup/repo@abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "gitlab" || parsed.Source.Locator != "gitlab:group/subgroup/repo" || parsed.Source.RequestedVersion != "abc123" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceRawHostedURLs(t *testing.T) {
	w := New("/tmp")
	for _, raw := range []string{"https://example.com/org/repo.git", "ssh://example.com/org/repo.git", "git@host:org/repo.git"} {
		parsed, err := w.ParseSource(raw)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", raw, err)
		}
		if parsed.Kind != "hosted" || parsed.Source.CloneURL != raw {
			t.Fatalf("unexpected parsed raw source for %s: %+v", raw, parsed)
		}
	}
}

func TestParseSourceLocalPath(t *testing.T) {
	w := New("/tmp")
	parsed, err := w.ParseSource("./skills")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Kind != "local" || parsed.Source.Path != "skills" {
		t.Fatalf("unexpected parsed source: %+v", parsed)
	}
}

func TestParseSourceRejectsMalformedHosted(t *testing.T) {
	w := New("/tmp")
	if _, err := w.ParseSource("github:owner"); err == nil {
		t.Fatal("expected error for malformed github source")
	}
	if _, err := w.ParseSource("gitlab:group"); err == nil {
		t.Fatal("expected error for malformed gitlab source")
	}
	if _, err := w.ParseSource("github:owner/repo@"); err == nil {
		t.Fatal("expected error for empty inline ref")
	}
}

func TestParseSourceRejectsEmpty(t *testing.T) {
	w := New("/tmp")
	if _, err := w.ParseSource("   "); err == nil {
		t.Fatal("expected error for empty source")
	}
}

func TestOpenSourceUsesRawCloneURL(t *testing.T) {
	root := t.TempDir()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "SKILL.md"), "# demo\n")
	mustWrite(t, filepath.Join(repo, "spick.skill.yaml"), "version: 1\nskills:\n    - id: demo\n      path: .\n")
	wrapper := filepath.Join(root, "git-wrapper.sh")
	mustWrite(t, wrapper, "#!/bin/bash\nset -eu\nlog=\"$SPICK_GIT_LOG\"\n: > \"$log\"\nprintf '%s\\n' \"$@\" >> \"$log\"\nif [ \"$1\" = clone ]; then\n  dest=\"${@: -1}\"\n  cp -R \"$SPICK_GIT_TEMPLATE\"/. \"$dest\"\nfi\n")
	if err := os.Chmod(wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(root, "git.log")
	t.Setenv("SPICK_GIT_BIN", wrapper)
	t.Setenv("SPICK_GIT_LOG", log)
	t.Setenv("SPICK_GIT_TEMPLATE", repo)
	w := New(root)
	opened, err := w.OpenSource(model.Source{CloneURL: "ssh://example.com/org/repo.git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opened.Path == "" {
		t.Fatal("expected checkout path")
	}
	if _, err := os.Stat(filepath.Join(opened.Path, "SKILL.md")); err != nil {
		t.Fatalf("expected cloned skill files: %v", err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ssh://example.com/org/repo.git") {
		t.Fatalf("expected raw url in git args, got %s", string(data))
	}
}

func TestLoadCatalogValidManifest(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.skill.yaml"), "version: 1\nskills:\n    - id: demo\n      path: .\n      name: Demo\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "demo" || got[0].Source == nil || got[0].Source.Path != "." {
		t.Fatalf("unexpected catalog: %+v", got)
	}
}

func TestLoadCatalogProjectOnlyFallsBackToTopLevelDiscovery(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  skills: []\n  plugins: []\n  agents: {}\n  exposureMethod: symlink\n")
	mustWrite(t, filepath.Join(root, "alpha", "SKILL.md"), "# alpha\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "alpha" || got[0].Source == nil || got[0].Source.Path != "alpha" {
		t.Fatalf("unexpected discovery result: %+v", got)
	}
}

func TestLoadCatalogProjectAndCatalogManifest(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  skills: []\n  plugins: []\n  agents: {}\n  exposureMethod: copy\n")
	mustWrite(t, filepath.Join(root, "spick.skill.yaml"), "version: 1\nskills:\n    - id: demo\n      path: .\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "demo" {
		t.Fatalf("unexpected manifest result: %+v", got)
	}
}

func TestLoadCatalogMissingManifest(t *testing.T) {
	loader := &Loader{Root: t.TempDir()}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected missing manifest error")
	}
}

func TestLoadCatalogInvalidProjectAgents(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  agents:\n    \"\": {skills: []}\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected invalid project.agents error")
	}
}

func TestLoadCatalogInvalidProjectExposureMethod(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  exposureMethod: invalid\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected invalid project.exposureMethod error")
	}
}

func TestLoadProjectConfigParsesPlugins(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  plugins:\n    - id: plugin1\n      source: github:owner/plugin\n      ref: v1\n  agents: {}\n")
	w := New(root)
	got, err := w.LoadProjectConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Plugins) != 1 || got.Plugins[0].ID != "plugin1" || got.Plugins[0].Source != "github:owner/plugin" || got.Plugins[0].Ref != "v1" {
		t.Fatalf("unexpected plugins: %+v", got)
	}
	if !got.AutoApply {
		t.Fatal("expected autoApply default true")
	}
}

func TestLoadProjectConfigParsesAutoApplyFalse(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  autoApply: false\n")
	w := New(root)
	got, err := w.LoadProjectConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AutoApply {
		t.Fatal("expected autoApply false")
	}
}

func TestLoadProjectConfigDefaultsAutoApplyTrue(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  agents: {}\n")
	w := New(root)
	got, err := w.LoadProjectConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.AutoApply {
		t.Fatal("expected default autoApply true")
	}
}

func TestLoadProjectConfigRejectsPluginWithoutSource(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  plugins:\n    - id: plugin1\n      ref: v1\n  agents: {}\n")
	w := New(root)
	if _, err := w.LoadProjectConfig(); err == nil {
		t.Fatal("expected invalid project.plugins error")
	}
}

func TestLoadProjectConfigRejectsDuplicateSkillIDs(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: skill1\n      source: github:owner/skill\n    - id: skill1\n      source: github:owner/skill\n  agents: {}\n")
	w := New(root)
	if _, err := w.LoadProjectConfig(); err == nil {
		t.Fatal("expected duplicate skill id error")
	}
}

func TestLoadProjectConfigRejectsUndeclaredAgentReference(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: skill1\n      source: github:owner/skill\n  agents:\n    opencode:\n      skills:\n        - missing\n")
	w := New(root)
	if _, err := w.LoadProjectConfig(); err == nil {
		t.Fatal("expected undeclared reference error")
	}
}

func TestRemoveProjectSkillsCleansDeclarationsAndEnablements(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  skills:\n    - id: one\n      source: github:owner/one\n    - id: two\n      source: github:owner/two\n  agents:\n    opencode:\n      skills:\n        - one\n        - two\n")
	w := New(root)
	if err := w.RemoveProjectSkills([]string{"one"}); err != nil {
		t.Fatal(err)
	}
	got, err := w.LoadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 1 || got.Skills[0].ID != "two" {
		t.Fatalf("unexpected skills: %+v", got.Skills)
	}
	if len(got.Agents["opencode"].Skills) != 1 || got.Agents["opencode"].Skills[0] != "two" {
		t.Fatalf("unexpected agent skills: %+v", got.Agents)
	}
}

func TestRemoveProjectPluginsCleansDeclarationsAndEnablements(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.yaml"), "project:\n  plugins:\n    - id: one\n      source: github:owner/one\n    - id: two\n      source: github:owner/two\n  agents:\n    opencode:\n      plugins:\n        - one\n        - two\n")
	w := New(root)
	if err := w.RemoveProjectPlugins([]string{"one"}); err != nil {
		t.Fatal(err)
	}
	got, err := w.LoadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Plugins) != 1 || got.Plugins[0].ID != "two" {
		t.Fatalf("unexpected plugins: %+v", got.Plugins)
	}
	if len(got.Agents["opencode"].Plugins) != 1 || got.Agents["opencode"].Plugins[0] != "two" {
		t.Fatalf("unexpected agent plugins: %+v", got.Agents)
	}
}

func TestLoadCatalogDuplicateID(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.skill.yaml"), "version: 1\nskills:\n    - id: demo\n      path: .\n    - id: demo\n      path: .\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestLoadCatalogInvalidID(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.skill.yaml"), "version: 1\nskills:\n    - id: Bad\n      path: .\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# demo\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestLoadCatalogPathEscapeRejected(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Dir(root)
	mustWrite(t, filepath.Join(root, "spick.skill.yaml"), "version: 1\nskills:\n    - id: demo\n      path: ../other\n")
	mustWrite(t, filepath.Join(parent, "SKILL.md"), "# other\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestLoadCatalogMissingSkillMD(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.skill.yaml"), "version: 1\nskills:\n    - id: demo\n      path: .\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected missing SKILL.md error")
	}
}

func TestManifestVersionDefaultsToOne(t *testing.T) {
	m, err := parseManifest([]byte(strings.TrimSpace("skills:\n    - id: demo\n      path: .\n")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Version != 1 {
		t.Fatalf("expected default version 1, got %d", m.Version)
	}
}

func TestDiscoverCatalogTopLevelSkill(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "root", "SKILL.md"), "# root\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "root" || got[0].Source == nil || got[0].Source.Path != "root" {
		t.Fatalf("unexpected root discovery: %+v", got)
	}
}

func TestDiscoverCatalogSkillsDirectoryPreferred(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "skills", "alpha", "SKILL.md"), "# alpha\n")
	mustWrite(t, filepath.Join(root, "beta", "SKILL.md"), "# beta\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "alpha" || got[0].Source == nil || got[0].Source.Path != filepath.Join("skills", "alpha") {
		t.Fatalf("unexpected skills discovery: %+v", got)
	}
}

func TestDiscoverCatalogMultiSkill(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "alpha", "SKILL.md"), "# alpha\n")
	mustWrite(t, filepath.Join(root, "beta", "SKILL.md"), "# beta\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 skills, got %+v", got)
	}
}

func TestDiscoverCatalogIgnoresNestedSkills(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "top", "nested", "SKILL.md"), "# nested\n")
	loader := &Loader{Root: root}
	if _, err := loader.LoadCatalog(); err == nil {
		t.Fatal("expected no top-level discoveries")
	}
}

func TestLoadCatalogManifestOverridesDiscovery(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "spick.skill.yaml"), "version: 1\nskills:\n    - id: demo\n      path: .\n")
	mustWrite(t, filepath.Join(root, "SKILL.md"), "# root\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "demo" {
		t.Fatalf("expected manifest-backed result, got %+v", got)
	}
}

func TestDiscoverCatalogSkipsNoiseDirs(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".git", "SKILL.md"), "# git\n")
	mustWrite(t, filepath.Join(root, "node_modules", "SKILL.md"), "# node\n")
	mustWrite(t, filepath.Join(root, "vendor", "SKILL.md"), "# vendor\n")
	mustWrite(t, filepath.Join(root, ".spick", "skills", "SKILL.md"), "# skills\n")
	mustWrite(t, filepath.Join(root, ".opencode", "SKILL.md"), "# opencode\n")
	loader := &Loader{Root: root}
	got, err := loader.LoadCatalog()
	if err == nil || len(got) != 0 {
		t.Fatalf("expected no discoveries from noise dirs, got %+v err=%v", got, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
