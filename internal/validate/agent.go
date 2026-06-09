package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// agentFileNameRegex allows letters, numbers, underscores, hyphens, and dots.
// Must end with .md (case-insensitive checked separately).
var agentFileNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// AgentFileSizeWarningThreshold is the size above which a warning is issued.
const AgentFileSizeWarningThreshold = 100 * 1024 // 100KB

// AgentValidationResult holds the result of validating an agent file.
type AgentValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// AgentFile validates a single agent .md file.
func AgentFile(filePath string) AgentValidationResult {
	result := AgentValidationResult{Valid: true}

	fileName := filepath.Base(filePath)

	// Must be .md extension
	if !strings.HasSuffix(strings.ToLower(fileName), ".md") {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("agent file must have .md extension, got %q", fileName))
		return result
	}

	// Filename restrictions: no spaces or special chars
	if !agentFileNameRegex.MatchString(fileName) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("agent filename %q contains invalid characters (spaces, special chars not allowed)", fileName))
	}

	// Check file exists and size
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Valid = false
			result.Errors = append(result.Errors, "agent file does not exist")
		}
		return result
	}

	if info.IsDir() {
		result.Valid = false
		result.Errors = append(result.Errors, "path is a directory, not a file")
		return result
	}

	if info.Size() > int64(AgentFileSizeWarningThreshold) {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("agent file is %dKB (>100KB) — large agents may slow down AI tools", info.Size()/1024))
	}

	return result
}

// AgentName validates an agent name (derived from filename).
func AgentName(name string) error {
	if name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}

	if len(name) > 128 {
		return fmt.Errorf("agent name too long (max 128 characters)")
	}

	nameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("agent name must start with a letter or number and contain only letters, numbers, underscores, and hyphens")
	}

	return nil
}
