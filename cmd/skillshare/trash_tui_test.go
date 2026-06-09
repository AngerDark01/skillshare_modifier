package main

import (
	"strings"
	"testing"

	"skillshare/internal/trash"
)

func TestTrashTUIRenderRestoreConfirmHeader_UsesAgentDestination(t *testing.T) {
	model := newTrashTUIModel([]trash.TrashEntry{
		{Name: "tutor", Kind: "agent"},
	}, "", "", "/tmp/skills", "/tmp/agents", "", "global")
	model.selected[0] = true
	model.selCount = 1
	model.confirmAction = "restore"
	model.confirmNames = []string{"tutor"}

	got := model.renderRestoreConfirmHeader()
	if !strings.Contains(got, "/tmp/agents") {
		t.Fatalf("expected agent restore header to use agent destination, got %q", got)
	}
	if strings.Contains(got, "/tmp/skills") {
		t.Fatalf("expected agent restore header to avoid skill destination, got %q", got)
	}
}

func TestTrashTUIRenderRestoreConfirmHeader_ShowsMixedDestinations(t *testing.T) {
	model := newTrashTUIModel([]trash.TrashEntry{
		{Name: "demo-skill", Kind: "skill"},
		{Name: "tutor", Kind: "agent"},
	}, "", "", "/tmp/skills", "/tmp/agents", "", "global")
	model.selected[0] = true
	model.selected[1] = true
	model.selCount = 2
	model.confirmAction = "restore"
	model.confirmNames = []string{"demo-skill", "tutor"}

	got := model.renderRestoreConfirmHeader()
	if !strings.Contains(got, "skills -> /tmp/skills") {
		t.Fatalf("expected mixed restore header to mention skills destination, got %q", got)
	}
	if !strings.Contains(got, "agents -> /tmp/agents") {
		t.Fatalf("expected mixed restore header to mention agent destination, got %q", got)
	}
}
