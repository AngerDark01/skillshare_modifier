package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentFile_Valid(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "tutor.md")
	os.WriteFile(f, []byte("# Tutor"), 0644)

	r := AgentFile(f)
	if !r.Valid {
		t.Errorf("expected valid, got errors: %v", r.Errors)
	}
	if len(r.Warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", r.Warnings)
	}
}

func TestAgentFile_WrongExtension(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "tutor.txt")
	os.WriteFile(f, []byte("content"), 0644)

	r := AgentFile(f)
	if r.Valid {
		t.Error("expected invalid for non-.md file")
	}
}

func TestAgentFile_InvalidFilename(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "my agent.md")
	os.WriteFile(f, []byte("# Agent"), 0644)

	r := AgentFile(f)
	if r.Valid {
		t.Error("expected invalid for filename with spaces")
	}
}

func TestAgentFile_Oversized(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "big.md")
	os.WriteFile(f, []byte(strings.Repeat("x", 200*1024)), 0644)

	r := AgentFile(f)
	if !r.Valid {
		t.Error("oversized file should be valid (warning only)")
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestAgentFile_NotExist(t *testing.T) {
	r := AgentFile("/nonexistent/agent.md")
	if r.Valid {
		t.Error("expected invalid for nonexistent file")
	}
}

func TestAgentFile_Directory(t *testing.T) {
	dir := t.TempDir()
	r := AgentFile(dir)
	if r.Valid {
		t.Error("expected invalid for directory path")
	}
}

func TestAgentName_Valid(t *testing.T) {
	for _, name := range []string{"tutor", "math-tutor", "code_review", "a1"} {
		if err := AgentName(name); err != nil {
			t.Errorf("AgentName(%q) should be valid, got: %v", name, err)
		}
	}
}

func TestAgentName_Invalid(t *testing.T) {
	for _, name := range []string{"", "-start", "has space", strings.Repeat("a", 129)} {
		if err := AgentName(name); err == nil {
			t.Errorf("AgentName(%q) should be invalid", name)
		}
	}
}
