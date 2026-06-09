package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

type targetAgentResponse struct {
	Name               string   `json:"name"`
	AgentPath          string   `json:"agentPath"`
	AgentMode          string   `json:"agentMode"`
	AgentInclude       []string `json:"agentInclude"`
	AgentExclude       []string `json:"agentExclude"`
	AgentLinkedCount   *int     `json:"agentLinkedCount"`
	AgentLocalCount    *int     `json:"agentLocalCount"`
	AgentExpectedCount *int     `json:"agentExpectedCount"`
}

func TestHandleListTargets_IncludesGlobalBuiltinAgents(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)

	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	agentSource := s.cfg.EffectiveAgentsSource()
	agentFile := addAgentFile(t, agentSource, "reviewer.md")
	agentTarget := filepath.Join(home, ".claude", "agents")
	addAgentLink(t, agentTarget, "reviewer.md", agentFile)

	target := fetchTargetByName(t, s, "claude")
	if target.AgentPath != agentTarget {
		t.Fatalf("agent path = %q, want %q", target.AgentPath, agentTarget)
	}
	if target.AgentMode != "merge" {
		t.Fatalf("agent mode = %q, want merge", target.AgentMode)
	}
	if target.AgentLinkedCount == nil || *target.AgentLinkedCount != 1 {
		t.Fatalf("agent linked = %v, want 1", target.AgentLinkedCount)
	}
	if target.AgentExpectedCount == nil || *target.AgentExpectedCount != 1 {
		t.Fatalf("agent expected = %v, want 1", target.AgentExpectedCount)
	}
}

func TestHandleListTargets_IncludesProjectBuiltinAgents(t *testing.T) {
	s, projectRoot := newProjectTargetServer(t, []config.ProjectTargetEntry{{Name: "claude"}})

	agentFile := addAgentFile(t, filepath.Join(projectRoot, ".skillshare", "agents"), "reviewer.md")
	agentTarget := filepath.Join(projectRoot, ".claude", "agents")
	addAgentLink(t, agentTarget, "reviewer.md", agentFile)

	target := fetchTargetByName(t, s, "claude")
	if target.AgentPath != agentTarget {
		t.Fatalf("agent path = %q, want %q", target.AgentPath, agentTarget)
	}
	if target.AgentMode != "merge" {
		t.Fatalf("agent mode = %q, want merge", target.AgentMode)
	}
	if target.AgentLinkedCount == nil || *target.AgentLinkedCount != 1 {
		t.Fatalf("agent linked = %v, want 1", target.AgentLinkedCount)
	}
	if target.AgentExpectedCount == nil || *target.AgentExpectedCount != 1 {
		t.Fatalf("agent expected = %v, want 1", target.AgentExpectedCount)
	}
}

func TestHandleListTargets_CustomAgentPathOverridesBuiltin(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	customAgentPath := filepath.Join(t.TempDir(), "custom-agents-target")
	if err := os.MkdirAll(customAgentPath, 0755); err != nil {
		t.Fatalf("mkdir custom target: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfgTarget := cfg.Targets["claude"]
	cfgTarget.Agents = &config.ResourceTargetConfig{
		Path:    customAgentPath,
		Mode:    "copy",
		Include: []string{"review-*"},
		Exclude: []string{"draft-*"},
	}
	cfg.Targets["claude"] = cfgTarget
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	addAgentFile(t, cfg.EffectiveAgentsSource(), "review-alpha.md")
	targetResp := fetchTargetByName(t, s, "claude")
	if targetResp.AgentPath != customAgentPath {
		t.Fatalf("agent path = %q, want %q", targetResp.AgentPath, customAgentPath)
	}
	if targetResp.AgentMode != "copy" {
		t.Fatalf("agent mode = %q, want copy", targetResp.AgentMode)
	}
	if got := targetResp.AgentInclude; len(got) != 1 || got[0] != "review-*" {
		t.Fatalf("agent include = %v, want [review-*]", got)
	}
	if got := targetResp.AgentExclude; len(got) != 1 || got[0] != "draft-*" {
		t.Fatalf("agent exclude = %v, want [draft-*]", got)
	}
}

func TestHandleListTargets_IncludesLocalAgentCount(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)

	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	agentTarget := filepath.Join(home, ".claude", "agents")
	addAgentFile(t, agentTarget, "local-only.md")

	target := fetchTargetByName(t, s, "claude")
	if target.AgentLocalCount == nil || *target.AgentLocalCount != 1 {
		t.Fatalf("agent local = %v, want 1", target.AgentLocalCount)
	}
	if target.AgentLinkedCount == nil || *target.AgentLinkedCount != 0 {
		t.Fatalf("agent linked = %v, want 0", target.AgentLinkedCount)
	}
	if target.AgentExpectedCount == nil || *target.AgentExpectedCount != 0 {
		t.Fatalf("agent expected = %v, want 0", target.AgentExpectedCount)
	}
}

func TestHandleListTargets_OmitsAgentsForUnsupportedTarget(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "custom-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"custom-tool": tgtPath})

	target := fetchTargetByName(t, s, "custom-tool")
	if target.AgentPath != "" {
		t.Fatalf("expected empty agent path, got %q", target.AgentPath)
	}
	if target.AgentLinkedCount != nil {
		t.Fatalf("expected nil agent linked count, got %v", *target.AgentLinkedCount)
	}
	if target.AgentExpectedCount != nil {
		t.Fatalf("expected nil agent expected count, got %v", *target.AgentExpectedCount)
	}
}

func fetchTargetByName(t *testing.T, s *Server, name string) targetAgentResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []targetAgentResponse `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	for _, target := range resp.Targets {
		if target.Name == name {
			return target
		}
	}
	t.Fatalf("target %q not found in response", name)
	return targetAgentResponse{}
}

func addAgentFile(t *testing.T, dir, name string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir agent source: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("# "+name), 0644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}
	return path
}

func addAgentLink(t *testing.T, dir, name, source string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir agent target: %v", err)
	}
	linkPath := filepath.Join(dir, name)
	if err := os.Symlink(source, linkPath); err != nil {
		t.Fatalf("symlink agent: %v", err)
	}
	return linkPath
}

func newProjectTargetServer(t *testing.T, targets []config.ProjectTargetEntry) (*Server, string) {
	t.Helper()

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".skillshare", "skills"), 0755); err != nil {
		t.Fatalf("mkdir project skills: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".skillshare", "agents"), 0755); err != nil {
		t.Fatalf("mkdir project agents: %v", err)
	}

	projectCfg := &config.ProjectConfig{Targets: targets}
	if err := projectCfg.Save(projectRoot); err != nil {
		t.Fatalf("save project config: %v", err)
	}

	resolvedTargets, err := config.ResolveProjectTargets(projectRoot, projectCfg)
	if err != nil {
		t.Fatalf("resolve project targets: %v", err)
	}

	cfg := &config.Config{
		Source:  filepath.Join(projectRoot, ".skillshare", "skills"),
		Mode:    "merge",
		Targets: resolvedTargets,
	}

	return NewProject(cfg, projectCfg, projectRoot, "127.0.0.1:0", "", ""), projectRoot
}
