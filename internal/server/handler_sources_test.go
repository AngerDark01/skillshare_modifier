package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleListSources_MissingFile(t *testing.T) {
	s, src := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp managedSourcesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Version != 1 || len(resp.Sources) != 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Path != filepath.Join(filepath.Dir(src), "sources.yaml") {
		t.Fatalf("unexpected sources path: %q", resp.Path)
	}
}

func TestHandleCreateSource_PersistsYAML(t *testing.T) {
	s, src := newTestServer(t)

	body := `{"label":"Matt Pocock Skills","source":"mattpocock/skills","kind":"skill"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp createManagedSourceResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Source.ID != "matt-pocock-skills" || resp.Source.Source != "mattpocock/skills" || resp.Source.Kind != "skill" {
		t.Fatalf("unexpected source: %+v", resp.Source)
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(src), "sources.yaml"))
	if err != nil {
		t.Fatalf("expected sources.yaml to be written: %v", err)
	}
	text := string(data)
	for _, want := range []string{"version: 1", "id: matt-pocock-skills", "source: mattpocock/skills"} {
		if !strings.Contains(text, want) {
			t.Fatalf("sources.yaml missing %q:\n%s", want, text)
		}
	}
}

func TestHandleCreateSource_RejectsDuplicateSourceAndBranch(t *testing.T) {
	s, _ := newTestServer(t)

	body := `{"label":"A","source":"mattpocock/skills","branch":"main"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/sources", bytes.NewBufferString(body))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if i == 0 && rr.Code != http.StatusCreated {
			t.Fatalf("expected first request to create, got %d: %s", rr.Code, rr.Body.String())
		}
		if i == 1 && rr.Code != http.StatusConflict {
			t.Fatalf("expected second request to conflict, got %d: %s", rr.Code, rr.Body.String())
		}
	}
}

func TestHandleDeleteSource(t *testing.T) {
	s, _ := newTestServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/api/sources", bytes.NewBufferString(`{"source":"mattpocock/skills"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRR.Code, createRR.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/sources/mattpocock-skills", nil)
	deleteRR := httptest.NewRecorder()
	s.handler.ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d: %s", deleteRR.Code, deleteRR.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	listRR := httptest.NewRecorder()
	s.handler.ServeHTTP(listRR, listReq)
	var resp managedSourcesResponse
	if err := json.Unmarshal(listRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Sources) != 0 {
		t.Fatalf("expected no sources after delete, got %+v", resp.Sources)
	}
}
