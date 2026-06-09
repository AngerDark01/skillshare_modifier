package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadRegistry_Empty(t *testing.T) {
	dir := t.TempDir()
	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry should succeed for missing file: %v", err)
	}
	if len(reg.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(reg.Skills))
	}
}

func TestRegistry_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	reg := &Registry{
		Skills: []SkillEntry{
			{Name: "my-skill", Source: "github.com/user/repo"},
			{Name: "nested", Source: "github.com/org/team", Tracked: true, Group: "frontend"},
		},
	}
	if err := reg.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "registry.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("registry.yaml not created: %v", err)
	}

	loaded, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}
	if len(loaded.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(loaded.Skills))
	}
	if loaded.Skills[0].Name != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", loaded.Skills[0].Name)
	}
	if loaded.Skills[1].Group != "frontend" {
		t.Errorf("expected group 'frontend', got %q", loaded.Skills[1].Group)
	}
}

func TestMigrateGlobalSkillsToRegistry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sourceDir := filepath.Join(dir, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write old-format config with skills[]
	oldConfig := "source: " + sourceDir + "\nskills:\n  - name: my-skill\n    source: github.com/user/repo\n"
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	_, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// registry.yaml should have been migrated to source dir
	reg, err := LoadRegistry(sourceDir)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}
	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill in registry, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Name != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", reg.Skills[0].Name)
	}

	// Re-read config.yaml — should no longer contain skills key
	data, _ := os.ReadFile(configPath)
	var check map[string]any
	yaml.Unmarshal(data, &check)
	if _, hasSkills := check["skills"]; hasSkills {
		t.Error("config.yaml should not contain skills: after migration")
	}
}

func TestMigrateGlobalSkills_NoMigrationWhenRegistryExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sourceDir := filepath.Join(dir, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write config with skills (old format)
	oldConfig := "source: " + sourceDir + "\nskills:\n  - name: stale\n    source: old\n"
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-existing registry.yaml in source dir — should NOT be overwritten
	reg := &Registry{Skills: []SkillEntry{{Name: "real", Source: "github.com/real"}}}
	if err := reg.Save(sourceDir); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SKILLSHARE_CONFIG", configPath)

	_, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// registry should still have "real", not "stale"
	loaded, err := LoadRegistry(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Name != "real" {
		t.Errorf("registry should be untouched, got: %+v", loaded.Skills)
	}
}

func TestLoadUnifiedRegistry_MergesBoth(t *testing.T) {
	skillsDir := t.TempDir()
	agentsDir := t.TempDir()

	// Write skills registry
	skillsReg := &Registry{
		Skills: []SkillEntry{
			{Name: "my-skill", Source: "github.com/user/skills-repo"},
		},
	}
	if err := skillsReg.Save(skillsDir); err != nil {
		t.Fatalf("Save skills: %v", err)
	}

	// Write agents registry
	agentsReg := &Registry{
		Skills: []SkillEntry{
			{Name: "my-agent", Source: "github.com/user/agents-repo"},
		},
	}
	if err := agentsReg.Save(agentsDir); err != nil {
		t.Fatalf("Save agents: %v", err)
	}

	unified, err := LoadUnifiedRegistry(skillsDir, agentsDir)
	if err != nil {
		t.Fatalf("LoadUnifiedRegistry: %v", err)
	}

	if len(unified.Skills) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(unified.Skills))
	}

	// Agent entry should have Kind="agent"
	for _, e := range unified.Skills {
		if e.Name == "my-agent" && e.EffectiveKind() != "agent" {
			t.Errorf("agent entry should have kind=agent, got %q", e.Kind)
		}
		if e.Name == "my-skill" && e.EffectiveKind() != "skill" {
			t.Errorf("skill entry should have kind=skill, got %q", e.Kind)
		}
	}
}

func TestLoadUnifiedRegistry_EmptyAgents(t *testing.T) {
	skillsDir := t.TempDir()
	agentsDir := t.TempDir() // no registry file

	skillsReg := &Registry{
		Skills: []SkillEntry{
			{Name: "s1", Source: "test"},
		},
	}
	skillsReg.Save(skillsDir)

	unified, err := LoadUnifiedRegistry(skillsDir, agentsDir)
	if err != nil {
		t.Fatalf("LoadUnifiedRegistry: %v", err)
	}
	if len(unified.Skills) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(unified.Skills))
	}
}

func TestSaveSplitByKind_RoundTrip(t *testing.T) {
	skillsDir := t.TempDir()
	agentsDir := t.TempDir()

	unified := &Registry{
		Skills: []SkillEntry{
			{Name: "skill-a", Source: "s1"},
			{Name: "agent-b", Source: "s2", Kind: "agent"},
			{Name: "skill-c", Source: "s3", Kind: "skill"},
		},
	}

	if err := unified.SaveSplitByKind(skillsDir, agentsDir); err != nil {
		t.Fatalf("SaveSplitByKind: %v", err)
	}

	// Load back separately
	skillsReg, err := LoadRegistry(skillsDir)
	if err != nil {
		t.Fatalf("LoadRegistry skills: %v", err)
	}
	if len(skillsReg.Skills) != 2 {
		t.Fatalf("expected 2 skill entries, got %d", len(skillsReg.Skills))
	}

	agentsReg, err := LoadRegistry(agentsDir)
	if err != nil {
		t.Fatalf("LoadRegistry agents: %v", err)
	}
	if len(agentsReg.Skills) != 1 {
		t.Fatalf("expected 1 agent entry, got %d", len(agentsReg.Skills))
	}
	if agentsReg.Skills[0].Name != "agent-b" {
		t.Errorf("agent name = %q, want %q", agentsReg.Skills[0].Name, "agent-b")
	}
}

func TestSaveSplitByKind_NoAgents_SkipsAgentFile(t *testing.T) {
	skillsDir := t.TempDir()
	agentsDir := t.TempDir()

	unified := &Registry{
		Skills: []SkillEntry{
			{Name: "only-skill", Source: "s1"},
		},
	}

	if err := unified.SaveSplitByKind(skillsDir, agentsDir); err != nil {
		t.Fatalf("SaveSplitByKind: %v", err)
	}

	// Agents dir should not have registry file
	if _, err := os.Stat(filepath.Join(agentsDir, "registry.yaml")); err == nil {
		t.Error("expected no agents registry.yaml when no agent entries")
	}
}

func TestSourceRoot_NoGit(t *testing.T) {
	dir := t.TempDir()
	got := SourceRoot(dir)
	if got != dir {
		t.Errorf("SourceRoot(%q) = %q, want %q", dir, got, dir)
	}
}

func TestSourceRoot_GitAtSource(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0755)
	got := SourceRoot(dir)
	if got != dir {
		t.Errorf("SourceRoot(%q) = %q, want %q", dir, got, dir)
	}
}

func TestSourceRoot_GitAtParent(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".git"), 0755)
	subdir := filepath.Join(root, "claude")
	os.Mkdir(subdir, 0755)
	got := SourceRoot(subdir)
	if got != root {
		t.Errorf("SourceRoot(%q) = %q, want %q", subdir, got, root)
	}
}

// Issue #157: LoadProject must preserve skills in config.yaml (no migration to registry).
func TestProjectSkillsNotMigratedToRegistry(t *testing.T) {
	root := t.TempDir()
	skillshareDir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(skillshareDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(skillshareDir, "config.yaml")

	oldConfig := "targets:\n  - claude\nskills:\n  - name: my-skill\n    source: github.com/user/repo\n"
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	// Skills should be in ProjectConfig, not migrated to registry.yaml
	if len(cfg.Skills) != 1 {
		t.Fatalf("expected 1 skill in ProjectConfig, got %d", len(cfg.Skills))
	}
	if cfg.Skills[0].Name != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", cfg.Skills[0].Name)
	}

	// registry.yaml should NOT be created
	registryPath := RegistryPath(skillshareDir)
	if _, err := os.Stat(registryPath); err == nil {
		t.Error("registry.yaml should not exist — skills stay in config.yaml")
	}
}
