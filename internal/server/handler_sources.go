package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"skillshare/internal/install"

	"gopkg.in/yaml.v3"
)

type managedSourcesFile struct {
	Version int                  `yaml:"version" json:"version"`
	Sources []managedSourceEntry `yaml:"sources" json:"sources"`
}

type managedSourceEntry struct {
	ID      string `yaml:"id" json:"id"`
	Label   string `yaml:"label" json:"label"`
	Source  string `yaml:"source" json:"source"`
	Branch  string `yaml:"branch,omitempty" json:"branch,omitempty"`
	Kind    string `yaml:"kind,omitempty" json:"kind,omitempty"`
	AddedAt string `yaml:"added_at,omitempty" json:"addedAt,omitempty"`
}

type managedSourcesResponse struct {
	Version int                  `json:"version"`
	Path    string               `json:"path"`
	Sources []managedSourceEntry `json:"sources"`
}

type createManagedSourceRequest struct {
	Label  string `json:"label"`
	Source string `json:"source"`
	Branch string `json:"branch"`
	Kind   string `json:"kind"`
}

type createManagedSourceResponse struct {
	Success bool               `json:"success"`
	Source  managedSourceEntry `json:"source"`
	Path    string             `json:"path"`
}

var sourceSlugCleanup = regexp.MustCompile(`[^a-z0-9]+`)

func sourcesFilePathForSkillsSource(skillsSource string) string {
	return filepath.Join(filepath.Dir(filepath.Clean(skillsSource)), "sources.yaml")
}

func normalizeManagedSourceKind(kind string) (string, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		return "skill", nil
	}
	if kind != "skill" && kind != "agent" {
		return "", fmt.Errorf("invalid source kind: %s", kind)
	}
	return kind, nil
}

func defaultManagedSourceLabel(source string) string {
	source = strings.TrimSpace(source)
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")
	source = strings.TrimPrefix(source, "github.com/")
	return strings.TrimSuffix(source, "/")
}

func managedSourceSlug(label, source string) string {
	base := strings.ToLower(strings.TrimSpace(label))
	if base == "" {
		base = strings.ToLower(defaultManagedSourceLabel(source))
	}
	base = sourceSlugCleanup.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return "source"
	}
	return base
}

func uniqueManagedSourceID(label, source string, existing []managedSourceEntry) string {
	base := managedSourceSlug(label, source)
	used := make(map[string]bool, len(existing))
	for _, entry := range existing {
		used[entry.ID] = true
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !used[candidate] {
			return candidate
		}
	}
}

func loadManagedSources(path string) (*managedSourcesFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &managedSourcesFile{Version: 1, Sources: []managedSourceEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &managedSourcesFile{Version: 1, Sources: []managedSourceEntry{}}, nil
	}

	var file managedSourcesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if file.Version == 0 {
		file.Version = 1
	}
	if file.Sources == nil {
		file.Sources = []managedSourceEntry{}
	}
	return &file, nil
}

func saveManagedSources(path string, file *managedSourcesFile) error {
	if file.Version == 0 {
		file.Version = 1
	}
	if file.Sources == nil {
		file.Sources = []managedSourceEntry{}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(file); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func managedSourcesEqual(a, b managedSourceEntry) bool {
	kindA, _ := normalizeManagedSourceKind(a.Kind)
	kindB, _ := normalizeManagedSourceKind(b.Kind)
	return strings.EqualFold(strings.TrimSpace(a.Source), strings.TrimSpace(b.Source)) &&
		strings.TrimSpace(a.Branch) == strings.TrimSpace(b.Branch) &&
		kindA == kindB
}

func (s *Server) managedSourcesPathLocked() string {
	return sourcesFilePathForSkillsSource(s.skillsSource())
}

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	path := s.managedSourcesPathLocked()
	s.mu.RUnlock()

	file, err := loadManagedSources(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sources: "+err.Error())
		return
	}

	writeJSON(w, managedSourcesResponse{
		Version: file.Version,
		Path:    path,
		Sources: file.Sources,
	})
}

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createManagedSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Source = strings.TrimSpace(req.Source)
	req.Label = strings.TrimSpace(req.Label)
	req.Branch = strings.TrimSpace(req.Branch)
	if req.Source == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}
	kind, err := normalizeManagedSourceKind(req.Kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Label == "" {
		req.Label = defaultManagedSourceLabel(req.Source)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := install.ParseSourceWithOptions(req.Source, s.parseOpts()); err != nil {
		writeError(w, http.StatusBadRequest, "invalid source: "+err.Error())
		return
	}

	path := s.managedSourcesPathLocked()
	file, err := loadManagedSources(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sources: "+err.Error())
		return
	}

	entry := managedSourceEntry{
		ID:      uniqueManagedSourceID(req.Label, req.Source, file.Sources),
		Label:   req.Label,
		Source:  req.Source,
		Branch:  req.Branch,
		Kind:    kind,
		AddedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, existing := range file.Sources {
		if managedSourcesEqual(existing, entry) {
			writeError(w, http.StatusConflict, "source already exists")
			return
		}
	}

	file.Sources = append(file.Sources, entry)
	if err := saveManagedSources(path, file); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save sources: "+err.Error())
		return
	}

	s.writeOpsLog("source-add", "ok", start, map[string]any{
		"id":     entry.ID,
		"source": entry.Source,
		"branch": entry.Branch,
		"kind":   entry.Kind,
		"scope":  "ui",
	}, "")

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, createManagedSourceResponse{Success: true, Source: entry, Path: path})
}

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "source id is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.managedSourcesPathLocked()
	file, err := loadManagedSources(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sources: "+err.Error())
		return
	}

	next := make([]managedSourceEntry, 0, len(file.Sources))
	var removed *managedSourceEntry
	for _, entry := range file.Sources {
		if entry.ID == id {
			copyEntry := entry
			removed = &copyEntry
			continue
		}
		next = append(next, entry)
	}
	if removed == nil {
		writeError(w, http.StatusNotFound, "source not found: "+id)
		return
	}

	file.Sources = next
	if err := saveManagedSources(path, file); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save sources: "+err.Error())
		return
	}

	s.writeOpsLog("source-remove", "ok", start, map[string]any{
		"id":     removed.ID,
		"source": removed.Source,
		"branch": removed.Branch,
		"kind":   removed.Kind,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]bool{"success": true})
}
