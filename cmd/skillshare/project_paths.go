package main

import (
	"path/filepath"

	"skillshare/internal/config"
)

// resolveProjectPath expands ~ and resolves relative project paths against
// the project root so all project-mode comparisons use a single path form.
func resolveProjectPath(projectRoot, path string) string {
	if path == "" {
		return ""
	}

	resolved := config.ExpandPath(path)
	if !filepath.IsAbs(resolved) {
		return filepath.Join(projectRoot, filepath.FromSlash(resolved))
	}

	return resolved
}
