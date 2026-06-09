package main

import (
	"skillshare/internal/config"
	"skillshare/internal/resource"
)

// statusJSONAgents is the agent section of status --json output.
type statusJSONAgents struct {
	Source  string                  `json:"source"`
	Exists  bool                    `json:"exists"`
	Count   int                     `json:"count"`
	Targets []statusJSONAgentTarget `json:"targets,omitempty"`
}

type statusJSONAgentTarget struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Expected int    `json:"expected"`
	Linked   int    `json:"linked"`
	Drift    bool   `json:"drift"`
}

// buildAgentStatusJSON builds the agents section for status --json output.
func buildAgentStatusJSON(cfg *config.Config) *statusJSONAgents {
	agentsSource := cfg.EffectiveAgentsSource()
	exists := dirExists(agentsSource)

	result := &statusJSONAgents{
		Source: agentsSource,
		Exists: exists,
	}

	if !exists {
		return result
	}

	agents, _ := resource.AgentKind{}.Discover(agentsSource)
	result.Count = len(agents)

	builtinAgents := config.DefaultAgentTargets()
	for name := range cfg.Targets {
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			continue
		}

		linked := countLinkedAgents(agentPath)
		result.Targets = append(result.Targets, statusJSONAgentTarget{
			Name:     name,
			Path:     agentPath,
			Expected: len(agents),
			Linked:   linked,
			Drift:    linked != len(agents) && len(agents) > 0,
		})
	}

	return result
}

// countLinkedAgents counts healthy .md symlinks in the target agent directory.
func countLinkedAgents(targetDir string) int {
	linked, _ := countAgentLinksAndBroken(targetDir)
	return linked
}
