package model

import "testing"

func TestParseResourceManifestDefaultsVersionAndInferenceFromKind(t *testing.T) {
	raw := ResourceManifestRaw{
		Kind: ResourceKindResources,
		Resources: ResourceCollections{
			Skills: []ResourceSkill{{ID: "demo", Path: "."}},
		},
	}
	got, err := ParseResourceManifest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 1 {
		t.Fatalf("expected default version 1, got %d", got.Version)
	}
	if got.Kind != ResourceKindResources {
		t.Fatalf("expected kind %q, got %q", ResourceKindResources, got.Kind)
	}
	if len(got.Resources.Skills) != 1 || got.Resources.Skills[0].ID != "demo" {
		t.Fatalf("unexpected parsed resources: %+v", got.Resources)
	}
}

func TestParseResourceManifestInfersPluginKind(t *testing.T) {
	raw := ResourceManifestRaw{
		Plugin: &ResourcePlugin{ID: "demo", Runtime: "opencode", Entry: "index.js"},
	}
	got, err := ParseResourceManifest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != ResourceKindPlugin {
		t.Fatalf("expected kind %q, got %q", ResourceKindPlugin, got.Kind)
	}
	if got.Plugin == nil || got.Plugin.ID != "demo" {
		t.Fatalf("expected plugin details to be preserved: %+v", got.Plugin)
	}
}

func TestParseResourceManifestInfersResourcesKind(t *testing.T) {
	raw := ResourceManifestRaw{
		Resources: ResourceCollections{
			Agents: []AgentResource{{ID: "agent", Path: "agents/review.md", Format: "markdown"}},
		},
	}
	got, err := ParseResourceManifest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != ResourceKindResources {
		t.Fatalf("expected kind %q, got %q", ResourceKindResources, got.Kind)
	}
	if len(got.Resources.Agents) != 1 || got.Resources.Agents[0].ID != "agent" {
		t.Fatalf("unexpected parsed agents: %+v", got.Resources)
	}
}

func TestParseResourceManifestRejectsInvalidKind(t *testing.T) {
	raw := ResourceManifestRaw{Kind: ResourceKind("invalid")}
	if _, err := ParseResourceManifest(raw); err == nil {
		t.Fatal("expected invalid kind error")
	}
}

func TestParseResourceManifestKeepsBlankKindWhenNotInferrable(t *testing.T) {
	raw := ResourceManifestRaw{}
	got, err := ParseResourceManifest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != "" {
		t.Fatalf("expected blank kind when no inference possible, got %q", got.Kind)
	}
}

func TestNormalizeResourceManifestForPersistenceDefaultsVersion(t *testing.T) {
	raw := ResourceManifest{Version: 0, Kind: ResourceKindPlugin}
	normalized := NormalizeResourceManifestForPersistence(raw)
	if normalized.Version != 1 {
		t.Fatalf("expected version default 1, got %d", normalized.Version)
	}
	if normalized.Kind != ResourceKindPlugin {
		t.Fatalf("expected explicit kind %q preserved, got %q", ResourceKindPlugin, normalized.Kind)
	}
}

func TestNormalizeResourceManifestForPersistenceInfersKindFromPlugin(t *testing.T) {
	raw := ResourceManifest{Plugin: &ResourcePlugin{ID: "demo", Runtime: "opencode", Entry: "index.js"}}
	normalized := NormalizeResourceManifestForPersistence(raw)
	if normalized.Kind != ResourceKindPlugin {
		t.Fatalf("expected kind %q, got %q", ResourceKindPlugin, normalized.Kind)
	}
}

func TestNormalizeResourceManifestForPersistenceInfersKindFromResources(t *testing.T) {
	raw := ResourceManifest{Resources: ResourceCollections{Skills: []ResourceSkill{{ID: "demo", Path: "."}}}}
	normalized := NormalizeResourceManifestForPersistence(raw)
	if normalized.Kind != ResourceKindResources {
		t.Fatalf("expected kind %q, got %q", ResourceKindResources, normalized.Kind)
	}
}
