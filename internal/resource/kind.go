package resource

// ResourceKind encapsulates per-kind behavior for skills and agents.
// Each kind defines how resources are discovered, named, linked, and validated.
type ResourceKind interface {
	// Kind returns the resource kind identifier ("skill" or "agent").
	Kind() string

	// Discover scans sourceDir and returns all resources found.
	Discover(sourceDir string) ([]DiscoveredResource, error)

	// ResolveName extracts the canonical name from a resource at the given path.
	// For skills: reads SKILL.md frontmatter name field.
	// For agents: uses filename, with optional frontmatter name override.
	ResolveName(path string) string

	// FlatName computes the flattened name used in target directories.
	// For skills: path/to/skill → path__to__skill (nested separator).
	// For agents: dir/file.md → file.md (directory prefix stripped).
	FlatName(relPath string) string

	// CreateLink creates a symlink from dst pointing to src.
	// For skills: directory symlink. For agents: file symlink.
	// Both use os.Symlink; the distinction is semantic (unit shape).
	CreateLink(src, dst string) error

	// SupportsAudit reports whether this kind supports security audit scanning.
	SupportsAudit() bool

	// SupportsTrack reports whether this kind supports tracked repo updates.
	SupportsTrack() bool

	// SupportsCollect reports whether this kind supports collecting from targets.
	SupportsCollect() bool
}

// DiscoveredResource represents a resource found during source directory scan.
// Used for both skills and agents.
type DiscoveredResource struct {
	Name        string   // Canonical name (from frontmatter or filename)
	Kind        string   // "skill" or "agent"
	RelPath     string   // Relative path from source root
	AbsPath     string   // Full absolute path
	IsNested    bool     // Whether this resource is inside a subdirectory
	FlatName    string   // Flattened name for target directories
	IsInRepo    bool     // Whether this resource is inside a tracked repo
	RepoRelPath string   // Relative path of the tracked repo root (when IsInRepo)
	Disabled    bool     // Whether this resource is ignored by ignore file
	SourcePath  string   // Full path preserving caller's logical path (may differ from AbsPath if symlinked)
	Targets     []string // Per-resource target restrictions from frontmatter (nil = all targets)
}

// ConventionalExcludes are filenames excluded from agent discovery.
// These are well-known convention files that appear in repos but are not agents.
var ConventionalExcludes = map[string]bool{
	"README.md":          true,
	"CHANGELOG.md":       true,
	"LICENSE.md":         true,
	"HISTORY.md":         true,
	"SECURITY.md":        true,
	"SKILL.md":           true,
	"CLAUDE.md":          true,
	"AGENTS.md":          true,
	"GEMINI.md":          true,
	"COPILOT.md":         true,
	"CONTRIBUTING.md":    true,
	"CODE_OF_CONDUCT.md": true,
	"SUPPORT.md":         true,
}
