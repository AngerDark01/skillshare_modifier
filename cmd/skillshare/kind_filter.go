package main

import "fmt"

// resourceKindFilter represents the kind filtering for CLI commands.
type resourceKindFilter int

const (
	kindAll    resourceKindFilter = iota // no filter — all kinds
	kindSkills                           // skills only
	kindAgents                           // agents only
)

// parseKindArg extracts a kind filter from the first positional argument.
// Returns the filter and remaining args.
// Recognized values: "skills", "skill", "agents", "agent".
// If the first arg is not a kind keyword, returns kindSkills with args unchanged
// (default is skills-only; use --all flag for both).
func parseKindArg(args []string) (resourceKindFilter, []string) {
	if len(args) == 0 {
		return kindSkills, args
	}

	switch args[0] {
	case "skills", "skill":
		return kindSkills, args[1:]
	case "agents", "agent":
		return kindAgents, args[1:]
	default:
		return kindSkills, args
	}
}

// extractAllFlag scans args for --all, removes it, and returns true if found.
// Commands use this to let --all mean kindAll (skills + agents).
func extractAllFlag(args []string) (bool, []string) {
	found := false
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--all" {
			found = true
		} else {
			rest = append(rest, a)
		}
	}
	return found, rest
}

// parseKindArgWithAll combines parseKindArg and extractAllFlag.
// It parses the positional kind keyword (agents/skills) and the --all flag.
// Used by commands where --all means kindAll (skills + agents).
func parseKindArgWithAll(args []string) (resourceKindFilter, []string) {
	kind, rest := parseKindArg(args)
	if allKinds, remaining := extractAllFlag(rest); allKinds {
		kind = kindAll
		rest = remaining
	}
	return kind, rest
}

// parseKindFlag extracts --kind flag from args.
// Returns the filter and remaining args with --kind removed.
func parseKindFlag(args []string) (resourceKindFilter, []string, error) {
	kind := kindAll
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		if args[i] == "--kind" {
			if i+1 >= len(args) {
				return kindAll, nil, fmt.Errorf("--kind requires a value (skill or agent)")
			}
			i++
			switch args[i] {
			case "skill", "skills":
				kind = kindSkills
			case "agent", "agents":
				kind = kindAgents
			default:
				return kindAll, nil, fmt.Errorf("--kind must be 'skill' or 'agent', got %q", args[i])
			}
		} else {
			rest = append(rest, args[i])
		}
	}

	return kind, rest, nil
}

func (k resourceKindFilter) String() string {
	switch k {
	case kindSkills:
		return "skills"
	case kindAgents:
		return "agents"
	default:
		return "all"
	}
}

// Noun returns the pluralized resource noun for display.
// Noun(1) → "skill"/"agent", Noun(2+) → "skills"/"agents".
func (k resourceKindFilter) Noun(count int) string {
	switch k {
	case kindAgents:
		if count == 1 {
			return "agent"
		}
		return "agents"
	default:
		if count == 1 {
			return "skill"
		}
		return "skills"
	}
}

// SingularNoun returns the singular resource noun (no count needed).
func (k resourceKindFilter) SingularNoun() string {
	return k.Noun(1)
}

func (k resourceKindFilter) IncludesSkills() bool {
	return k == kindAll || k == kindSkills
}

func (k resourceKindFilter) IncludesAgents() bool {
	return k == kindAll || k == kindAgents
}
