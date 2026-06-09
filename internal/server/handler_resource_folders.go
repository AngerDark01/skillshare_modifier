package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/install"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

type resourceFoldersResponse struct {
	Folders []string `json:"folders"`
}

type createResourceFolderRequest struct {
	Folder string `json:"folder"`
}

type createResourceFolderResponse struct {
	Success bool   `json:"success"`
	Folder  string `json:"folder"`
}

type moveResourceRequest struct {
	Folder string `json:"folder"`
}

type moveResourceResponse struct {
	Success  bool   `json:"success"`
	Name     string `json:"name"`
	From     string `json:"from"`
	To       string `json:"to"`
	FlatName string `json:"flatName"`
}

func normalizeResourceFolder(input string) (string, error) {
	folder := strings.TrimSpace(strings.ReplaceAll(input, "\\", "/"))
	if folder == "" {
		return "", nil
	}
	if strings.ContainsRune(folder, 0) {
		return "", fmt.Errorf("invalid folder: NUL byte not allowed")
	}
	if strings.HasPrefix(folder, "/") || filepath.IsAbs(filepath.FromSlash(folder)) {
		return "", fmt.Errorf("invalid folder: absolute paths are not allowed")
	}
	clean := path.Clean(folder)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid folder: path traversal not allowed")
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid folder: empty or traversal segment")
		}
		if utils.IsHidden(part) {
			return "", fmt.Errorf("invalid folder: hidden folders are reserved")
		}
		if utils.IsTrackedRepoDir(part) {
			return "", fmt.Errorf("invalid folder: folders starting with _ are reserved for tracked repositories")
		}
	}
	return clean, nil
}

func safeJoinResourceFolder(root, folder string) (string, error) {
	root = filepath.Clean(root)
	if folder == "" {
		return root, nil
	}
	target := filepath.Clean(filepath.Join(root, filepath.FromSlash(folder)))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid folder: outside source root")
	}
	return target, nil
}

func ensureCategoryFolder(root, folder string) error {
	if folder == "" {
		return nil
	}
	current := filepath.Clean(root)
	for _, part := range strings.Split(folder, "/") {
		current = filepath.Join(current, filepath.FromSlash(part))
		if _, err := os.Stat(filepath.Join(current, "SKILL.md")); err == nil {
			return fmt.Errorf("folder is inside a skill: %s", folder)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to inspect folder: %w", err)
		}
	}
	return nil
}

func collectResourceFolders(root string) ([]string, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return []string{}, nil
	}

	var folders []string
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if current == root {
			return nil
		}
		if utils.IsHidden(entry.Name()) {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(root, current)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if len(parts) > 0 && utils.IsTrackedRepoDir(parts[0]) {
			return filepath.SkipDir
		}
		if _, statErr := os.Stat(filepath.Join(current, "SKILL.md")); statErr == nil {
			return filepath.SkipDir
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return nil
		}

		folders = append(folders, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(folders)
	return folders, nil
}

func (s *Server) handleListResourceFolders(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	s.mu.RUnlock()

	folders, err := collectResourceFolders(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list folders: "+err.Error())
		return
	}
	writeJSON(w, resourceFoldersResponse{Folders: folders})
}

func (s *Server) handleCreateResourceFolder(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createResourceFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	folder, err := normalizeResourceFolder(req.Folder)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if folder == "" {
		writeError(w, http.StatusBadRequest, "folder is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	source := s.cfg.EffectiveSkillsSource()
	if err := ensureCategoryFolder(source, folder); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	folderPath, err := safeJoinResourceFolder(source, folder)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := os.Stat(folderPath); err == nil {
		writeError(w, http.StatusConflict, "folder already exists: "+folder)
		return
	} else if err != nil && !os.IsNotExist(err) {
		writeError(w, http.StatusInternalServerError, "failed to inspect folder: "+err.Error())
		return
	}
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create folder: "+err.Error())
		return
	}

	s.writeOpsLog("create-folder", "ok", start, map[string]any{
		"folder": folder,
		"scope":  "ui",
	}, "")

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, createResourceFolderResponse{Success: true, Folder: folder})
}

func findSkillByName(discovered []ssync.DiscoveredSkill, name string) *ssync.DiscoveredSkill {
	for i := range discovered {
		if discovered[i].FlatName == name || discovered[i].RelPath == name {
			return &discovered[i]
		}
	}
	for i := range discovered {
		if filepath.Base(discovered[i].SourcePath) == name {
			return &discovered[i]
		}
	}
	return nil
}

func metadataKeyByRelPath(store *install.MetadataStore, relPath string) string {
	if store == nil {
		return ""
	}
	for _, key := range store.List() {
		entry := store.Get(key)
		if key == relPath || install.KeyToRelPath(key, entry) == relPath {
			return key
		}
	}
	return ""
}

func folderFromSkillRelPath(relPath string) string {
	dir := path.Dir(filepath.ToSlash(relPath))
	if dir == "." {
		return ""
	}
	return dir
}

func (s *Server) moveSkillMetadata(source, oldRelPath, newRelPath, newSourcePath string) {
	if s.skillsStore == nil {
		return
	}
	entry := s.skillsStore.GetByPath(oldRelPath)
	if entry == nil {
		return
	}
	next := *entry
	next.Group = folderFromSkillRelPath(newRelPath)

	if key := metadataKeyByRelPath(s.skillsStore, oldRelPath); key != "" {
		s.skillsStore.Remove(key)
	}
	s.skillsStore.Set(newRelPath, &next)
	s.skillsStore.RefreshHashes(newRelPath, newSourcePath)
	s.skillsStore.Save(source) //nolint:errcheck
}

func (s *Server) handleMoveResource(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	var req moveResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	folder, err := normalizeResourceFolder(req.Folder)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	source := s.cfg.EffectiveSkillsSource()
	discovered, err := ssync.DiscoverSourceSkillsAll(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}
	match := findSkillByName(discovered, name)
	if match == nil {
		writeError(w, http.StatusNotFound, "skill not found: "+name)
		return
	}
	if match.IsInRepo {
		writeError(w, http.StatusBadRequest, "cannot move tracked-repo skill; manage the repository layout instead")
		return
	}
	if err := ensureCategoryFolder(source, folder); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	baseName := filepath.Base(match.SourcePath)
	newRelPath := baseName
	if folder != "" {
		newRelPath = folder + "/" + baseName
	}
	if newRelPath == match.RelPath {
		writeJSON(w, moveResourceResponse{
			Success:  true,
			Name:     baseName,
			From:     match.RelPath,
			To:       newRelPath,
			FlatName: match.FlatName,
		})
		return
	}

	destPath, err := safeJoinResourceFolder(source, newRelPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := os.Stat(destPath); err == nil {
		writeError(w, http.StatusConflict, "destination already exists: "+newRelPath)
		return
	} else if err != nil && !os.IsNotExist(err) {
		writeError(w, http.StatusInternalServerError, "failed to inspect destination: "+err.Error())
		return
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create destination folder: "+err.Error())
		return
	}
	if err := os.Rename(match.SourcePath, destPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to move skill: "+err.Error())
		return
	}

	s.moveSkillMetadata(source, match.RelPath, newRelPath, destPath)
	newFlatName := utils.PathToFlatName(newRelPath)

	s.writeOpsLog("move-skill", "ok", start, map[string]any{
		"name":  baseName,
		"from":  match.RelPath,
		"to":    newRelPath,
		"scope": "ui",
	}, "")

	writeJSON(w, moveResourceResponse{
		Success:  true,
		Name:     baseName,
		From:     match.RelPath,
		To:       newRelPath,
		FlatName: newFlatName,
	})
}
