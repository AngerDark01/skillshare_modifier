package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
)

func TestHandleListResourceFolders_IncludesEmptyFolders(t *testing.T) {
	s, src := newTestServer(t)
	if err := os.MkdirAll(filepath.Join(src, "02-code-quality", "review"), 0755); err != nil {
		t.Fatalf("failed to create folder: %v", err)
	}
	addSkillNested(t, src, "03-writing/article-writer")
	addTrackedRepo(t, src, "_team")
	addSkillNested(t, src, "_team/repo-skill")

	req := httptest.NewRequest(http.MethodGet, "/api/resources/folders", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp resourceFoldersResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	want := map[string]bool{
		"02-code-quality":        true,
		"02-code-quality/review": true,
		"03-writing":             true,
	}
	for _, folder := range resp.Folders {
		if folder == "_team" || folder == "_team/repo-skill" || folder == "03-writing/article-writer" {
			t.Fatalf("unexpected folder %q in %+v", folder, resp.Folders)
		}
		delete(want, folder)
	}
	for folder := range want {
		t.Fatalf("expected folder %q in %+v", folder, resp.Folders)
	}
}

func TestHandleCreateResourceFolder(t *testing.T) {
	s, src := newTestServer(t)

	body := `{"folder":"02-code-quality/review"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/folders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(src, "02-code-quality", "review")); err != nil {
		t.Fatalf("expected folder to exist: %v", err)
	}
}

func TestHandleCreateResourceFolder_RejectsTraversal(t *testing.T) {
	s, _ := newTestServer(t)

	body := `{"folder":"../escape"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/folders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateResourceFolder_RejectsSkillSubfolder(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	body := `{"folder":"my-skill/sub"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/folders", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleMoveResource(t *testing.T) {
	s, src := newTestServer(t)
	addSkillNested(t, src, "frontend/my-skill")
	s.skillsStore.Set("frontend/my-skill", &install.MetadataEntry{
		Source: "github.com/acme/skills",
		Group:  "frontend",
	})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	body := `{"folder":"backend"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/frontend__my-skill/move", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp moveResourceResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.To != "backend/my-skill" || resp.FlatName != "backend__my-skill" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(src, "backend", "my-skill", "SKILL.md")); err != nil {
		t.Fatalf("expected moved skill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(src, "frontend", "my-skill")); !os.IsNotExist(err) {
		t.Fatalf("expected old skill path to be gone, err=%v", err)
	}

	entry := s.skillsStore.GetByPath("backend/my-skill")
	if entry == nil || entry.Group != "backend" || entry.Source != "github.com/acme/skills" {
		t.Fatalf("expected metadata to move, got %+v", entry)
	}
	if s.skillsStore.GetByPath("frontend/my-skill") != nil {
		t.Fatalf("expected old metadata path to be removed")
	}
}

func TestHandleMoveResource_ToRoot(t *testing.T) {
	s, src := newTestServer(t)
	addSkillNested(t, src, "frontend/my-skill")

	body := `{"folder":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/frontend__my-skill/move", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(src, "my-skill", "SKILL.md")); err != nil {
		t.Fatalf("expected moved skill at root: %v", err)
	}
}

func TestHandleMoveResource_DestinationConflict(t *testing.T) {
	s, src := newTestServer(t)
	addSkillNested(t, src, "frontend/my-skill")
	addSkillNested(t, src, "backend/my-skill")

	body := `{"folder":"backend"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/frontend__my-skill/move", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleMoveResource_RejectsTrackedRepoSkill(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, "_team")
	addSkillNested(t, src, "_team/my-skill")

	body := `{"folder":"backend"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/_team__my-skill/move", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
