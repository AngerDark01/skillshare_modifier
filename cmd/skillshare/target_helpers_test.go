package main

import (
	"testing"

	ssync "skillshare/internal/sync"
)

func TestParseFilterFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOpts parsedTargetFilterFlags
		wantRest []string
		wantErr  bool
	}{
		{
			name:     "no flags",
			args:     []string{"--mode", "merge"},
			wantOpts: parsedTargetFilterFlags{},
			wantRest: []string{"--mode", "merge"},
		},
		{
			name: "all four flags",
			args: []string{
				"--add-include", "team-*",
				"--add-exclude", "_legacy*",
				"--remove-include", "old-*",
				"--remove-exclude", "test-*",
			},
			wantOpts: parsedTargetFilterFlags{
				Skills: filterUpdateOpts{
					AddInclude:    []string{"team-*"},
					AddExclude:    []string{"_legacy*"},
					RemoveInclude: []string{"old-*"},
					RemoveExclude: []string{"test-*"},
				},
			},
		},
		{
			name: "multiple values for same flag",
			args: []string{
				"--add-include", "a-*",
				"--add-include", "b-*",
			},
			wantOpts: parsedTargetFilterFlags{
				Skills: filterUpdateOpts{
					AddInclude: []string{"a-*", "b-*"},
				},
			},
		},
		{
			name: "agent flags",
			args: []string{
				"--add-agent-include", "team-*",
				"--remove-agent-exclude", "draft-*",
			},
			wantOpts: parsedTargetFilterFlags{
				Agents: filterUpdateOpts{
					AddInclude:    []string{"team-*"},
					RemoveExclude: []string{"draft-*"},
				},
			},
		},
		{
			name: "mixed with other flags",
			args: []string{
				"--mode", "merge",
				"--add-include", "team-*",
				"--add-agent-exclude", "draft-*",
			},
			wantOpts: parsedTargetFilterFlags{
				Skills: filterUpdateOpts{
					AddInclude: []string{"team-*"},
				},
				Agents: filterUpdateOpts{
					AddExclude: []string{"draft-*"},
				},
			},
			wantRest: []string{"--mode", "merge"},
		},
		{
			name:    "missing value for --add-include",
			args:    []string{"--add-include"},
			wantErr: true,
		},
		{
			name:    "missing value for --add-exclude",
			args:    []string{"--add-exclude"},
			wantErr: true,
		},
		{
			name:    "missing value for --remove-include",
			args:    []string{"--remove-include"},
			wantErr: true,
		},
		{
			name:    "missing value for --remove-exclude",
			args:    []string{"--remove-exclude"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, rest, err := parseFilterFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertStringSlice(t, "Skills.AddInclude", opts.Skills.AddInclude, tt.wantOpts.Skills.AddInclude)
			assertStringSlice(t, "Skills.AddExclude", opts.Skills.AddExclude, tt.wantOpts.Skills.AddExclude)
			assertStringSlice(t, "Skills.RemoveInclude", opts.Skills.RemoveInclude, tt.wantOpts.Skills.RemoveInclude)
			assertStringSlice(t, "Skills.RemoveExclude", opts.Skills.RemoveExclude, tt.wantOpts.Skills.RemoveExclude)
			assertStringSlice(t, "Agents.AddInclude", opts.Agents.AddInclude, tt.wantOpts.Agents.AddInclude)
			assertStringSlice(t, "Agents.AddExclude", opts.Agents.AddExclude, tt.wantOpts.Agents.AddExclude)
			assertStringSlice(t, "Agents.RemoveInclude", opts.Agents.RemoveInclude, tt.wantOpts.Agents.RemoveInclude)
			assertStringSlice(t, "Agents.RemoveExclude", opts.Agents.RemoveExclude, tt.wantOpts.Agents.RemoveExclude)
			assertStringSlice(t, "rest", rest, tt.wantRest)
		})
	}
}

func TestParseTargetSettingFlags(t *testing.T) {
	settings, err := parseTargetSettingFlags([]string{
		"--mode", "copy",
		"--agent-mode", "merge",
		"--target-naming", "standard",
	})
	if err != nil {
		t.Fatalf("parseTargetSettingFlags: %v", err)
	}
	if settings.SkillMode != "copy" {
		t.Fatalf("SkillMode = %q, want copy", settings.SkillMode)
	}
	if settings.AgentMode != "merge" {
		t.Fatalf("AgentMode = %q, want merge", settings.AgentMode)
	}
	if settings.Naming != "standard" {
		t.Fatalf("Naming = %q, want standard", settings.Naming)
	}
}

func TestApplyFilterUpdates(t *testing.T) {
	tests := []struct {
		name        string
		include     []string
		exclude     []string
		opts        filterUpdateOpts
		wantInclude []string
		wantExclude []string
		wantChanges int
		wantErr     bool
	}{
		{
			name: "add include",
			opts: filterUpdateOpts{
				AddInclude: []string{"team-*"},
			},
			wantInclude: []string{"team-*"},
			wantChanges: 1,
		},
		{
			name: "add exclude",
			opts: filterUpdateOpts{
				AddExclude: []string{"_legacy*"},
			},
			wantExclude: []string{"_legacy*"},
			wantChanges: 1,
		},
		{
			name:    "remove include",
			include: []string{"team-*", "org-*"},
			opts: filterUpdateOpts{
				RemoveInclude: []string{"team-*"},
			},
			wantInclude: []string{"org-*"},
			wantChanges: 1,
		},
		{
			name:    "remove exclude",
			exclude: []string{"_legacy*", "test-*"},
			opts: filterUpdateOpts{
				RemoveExclude: []string{"_legacy*"},
			},
			wantExclude: []string{"test-*"},
			wantChanges: 1,
		},
		{
			name:    "deduplicate add",
			include: []string{"team-*"},
			opts: filterUpdateOpts{
				AddInclude: []string{"team-*"},
			},
			wantInclude: []string{"team-*"},
			wantChanges: 0,
		},
		{
			name: "remove nonexistent is no-op",
			opts: filterUpdateOpts{
				RemoveInclude: []string{"nope"},
			},
			wantChanges: 0,
		},
		{
			name: "invalid include pattern",
			opts: filterUpdateOpts{
				AddInclude: []string{"[invalid"},
			},
			wantErr: true,
		},
		{
			name: "invalid exclude pattern",
			opts: filterUpdateOpts{
				AddExclude: []string{"[invalid"},
			},
			wantErr: true,
		},
		{
			name:    "multiple operations",
			include: []string{"old-*"},
			exclude: []string{"old-exc-*"},
			opts: filterUpdateOpts{
				AddInclude:    []string{"new-*"},
				RemoveInclude: []string{"old-*"},
				AddExclude:    []string{"new-exc-*"},
				RemoveExclude: []string{"old-exc-*"},
			},
			wantInclude: []string{"new-*"},
			wantExclude: []string{"new-exc-*"},
			wantChanges: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			include := append([]string(nil), tt.include...)
			exclude := append([]string(nil), tt.exclude...)

			changes, err := applyFilterUpdates(&include, &exclude, tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(changes) != tt.wantChanges {
				t.Errorf("changes count = %d, want %d (changes: %v)", len(changes), tt.wantChanges, changes)
			}
			assertStringSlice(t, "include", include, tt.wantInclude)
			assertStringSlice(t, "exclude", exclude, tt.wantExclude)
		})
	}
}

func TestFilterUpdateOpts_HasUpdates(t *testing.T) {
	if (filterUpdateOpts{}).hasUpdates() {
		t.Error("empty opts should not have updates")
	}
	if !(filterUpdateOpts{AddInclude: []string{"x"}}).hasUpdates() {
		t.Error("opts with AddInclude should have updates")
	}
}

func TestScopeFilterChanges_Agents(t *testing.T) {
	changes := scopeFilterChanges("agents", []string{
		"added include: team-*",
		"added exclude: draft-*",
		"removed include: team-*",
		"removed exclude: draft-*",
	})
	want := []string{
		"added agent include: team-*",
		"added agent exclude: draft-*",
		"removed agent include: team-*",
		"removed agent exclude: draft-*",
	}
	assertStringSlice(t, "changes", changes, want)
}

func TestFindUnknownSkillTargets_CustomTargets(t *testing.T) {
	discovered := []ssync.DiscoveredSkill{
		{RelPath: "skill-a", Targets: []string{"claude", "custom-tool"}},
		{RelPath: "skill-b", Targets: []string{"custom-tool"}},
		{RelPath: "skill-c", Targets: nil}, // nil = all targets, should be skipped
	}

	// Without extra names, "custom-tool" is unknown
	warnings := findUnknownSkillTargets(discovered, nil)
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings without extra names, got %d: %v", len(warnings), warnings)
	}

	// With "custom-tool" as an extra name, no warnings
	warnings = findUnknownSkillTargets(discovered, []string{"custom-tool"})
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings with custom-tool as extra, got %d: %v", len(warnings), warnings)
	}
}

func assertStringSlice(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if len(got) != len(want) {
		t.Errorf("%s: got %v, want %v", label, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", label, i, got[i], want[i])
		}
	}
}
