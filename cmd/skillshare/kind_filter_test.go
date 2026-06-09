package main

import "testing"

func TestParseKindArg(t *testing.T) {
	tests := []struct {
		args     []string
		wantKind resourceKindFilter
		wantRest []string
	}{
		{nil, kindSkills, nil},
		{[]string{}, kindSkills, []string{}},
		{[]string{"skills"}, kindSkills, []string{}},
		{[]string{"skill"}, kindSkills, []string{}},
		{[]string{"agents"}, kindAgents, []string{}},
		{[]string{"agent"}, kindAgents, []string{}},
		{[]string{"all"}, kindSkills, []string{"all"}},                     // "all" no longer a keyword
		{[]string{"all", "--json"}, kindSkills, []string{"all", "--json"}}, // falls through to default
		{[]string{"agents", "tutor"}, kindAgents, []string{"tutor"}},
		{[]string{"--json"}, kindSkills, []string{"--json"}},
		{[]string{"my-skill"}, kindSkills, []string{"my-skill"}},
	}

	for _, tt := range tests {
		kind, rest := parseKindArg(tt.args)
		if kind != tt.wantKind {
			t.Errorf("parseKindArg(%v) kind = %v, want %v", tt.args, kind, tt.wantKind)
		}
		if len(rest) != len(tt.wantRest) {
			t.Errorf("parseKindArg(%v) rest = %v, want %v", tt.args, rest, tt.wantRest)
		}
	}
}

func TestParseKindFlag(t *testing.T) {
	tests := []struct {
		args     []string
		wantKind resourceKindFilter
		wantRest []string
		wantErr  bool
	}{
		{[]string{}, kindAll, []string{}, false},
		{[]string{"--kind", "agent"}, kindAgents, []string{}, false},
		{[]string{"--kind", "skill"}, kindSkills, []string{}, false},
		{[]string{"--json", "--kind", "agent", "foo"}, kindAgents, []string{"--json", "foo"}, false},
		{[]string{"--kind"}, kindAll, nil, true},
		{[]string{"--kind", "invalid"}, kindAll, nil, true},
	}

	for _, tt := range tests {
		kind, rest, err := parseKindFlag(tt.args)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseKindFlag(%v) err = %v, wantErr %v", tt.args, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if kind != tt.wantKind {
			t.Errorf("parseKindFlag(%v) kind = %v, want %v", tt.args, kind, tt.wantKind)
		}
		if len(rest) != len(tt.wantRest) {
			t.Errorf("parseKindFlag(%v) rest = %v, want %v", tt.args, rest, tt.wantRest)
		}
	}
}

func TestExtractAllFlag(t *testing.T) {
	tests := []struct {
		args     []string
		wantAll  bool
		wantRest []string
	}{
		{nil, false, []string{}},
		{[]string{"--all"}, true, []string{}},
		{[]string{"--json", "--all", "foo"}, true, []string{"--json", "foo"}},
		{[]string{"--json"}, false, []string{"--json"}},
		{[]string{"agents", "--all"}, true, []string{"agents"}},
		{[]string{"--all", "--all"}, true, []string{}}, // duplicate --all
	}

	for _, tt := range tests {
		gotAll, gotRest := extractAllFlag(tt.args)
		if gotAll != tt.wantAll {
			t.Errorf("extractAllFlag(%v) all = %v, want %v", tt.args, gotAll, tt.wantAll)
		}
		if len(gotRest) != len(tt.wantRest) {
			t.Errorf("extractAllFlag(%v) rest = %v, want %v", tt.args, gotRest, tt.wantRest)
		}
	}
}

func TestParseKindArgWithAll(t *testing.T) {
	tests := []struct {
		args     []string
		wantKind resourceKindFilter
		wantRest []string
	}{
		{nil, kindSkills, nil},
		{[]string{"--all"}, kindAll, []string{}},
		{[]string{"agents"}, kindAgents, []string{}},
		{[]string{"agents", "--all"}, kindAll, []string{}},
		{[]string{"--json", "--all"}, kindAll, []string{"--json"}},
		{[]string{"--json"}, kindSkills, []string{"--json"}},
	}

	for _, tt := range tests {
		kind, rest := parseKindArgWithAll(tt.args)
		if kind != tt.wantKind {
			t.Errorf("parseKindArgWithAll(%v) kind = %v, want %v", tt.args, kind, tt.wantKind)
		}
		if len(rest) != len(tt.wantRest) {
			t.Errorf("parseKindArgWithAll(%v) rest = %v, want %v", tt.args, rest, tt.wantRest)
		}
	}
}

func TestResourceKindFilter_Includes(t *testing.T) {
	if !kindAll.IncludesSkills() || !kindAll.IncludesAgents() {
		t.Error("kindAll should include both")
	}
	if !kindSkills.IncludesSkills() || kindSkills.IncludesAgents() {
		t.Error("kindSkills should include only skills")
	}
	if kindAgents.IncludesSkills() || !kindAgents.IncludesAgents() {
		t.Error("kindAgents should include only agents")
	}
}
