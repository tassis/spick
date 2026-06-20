package model

import "testing"

func TestNormalizeResourceManifestSkillsDefaults(t *testing.T) {
	manifest := &ResourceManifest{Resources: ResourceCollections{Skills: []ResourceSkill{{ID: "demo", Path: "./skills/demo"}}}}
	normalized, err := NormalizeResourceManifestSkills(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(normalized) != 1 || normalized[0].ID != "demo" {
		t.Fatalf("unexpected normalized skills: %+v", normalized)
	}
}

func TestNormalizeResourceManifestSkillsRequiresManifest(t *testing.T) {
	if _, err := NormalizeResourceManifestSkills(nil); err == nil {
		t.Fatal("expected manifest required error")
	}
}

func TestNormalizeResourceManifestSkillsRequiresSkills(t *testing.T) {
	manifest := &ResourceManifest{}
	if _, err := NormalizeResourceManifestSkills(manifest); err == nil {
		t.Fatal("expected skills required error")
	}
}

func TestNormalizeResourceManifestSkillsRejectsInvalidID(t *testing.T) {
	manifest := &ResourceManifest{Resources: ResourceCollections{Skills: []ResourceSkill{{ID: "Bad", Path: "."}}}}
	if _, err := NormalizeResourceManifestSkills(manifest); err == nil {
		t.Fatal("expected invalid skill id error")
	}
}

func TestNormalizeResourceManifestSkillsRejectsDuplicateIDs(t *testing.T) {
	manifest := &ResourceManifest{Resources: ResourceCollections{Skills: []ResourceSkill{{ID: "demo", Path: "./a"}, {ID: "demo", Path: "./b"}}}}
	if _, err := NormalizeResourceManifestSkills(manifest); err == nil {
		t.Fatal("expected duplicate skill id error")
	}
}
