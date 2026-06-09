package main

import (
	"skillshare/internal/config"
	"skillshare/internal/ui"
)

// printSyncOverlapHint emits a single-line warning when enabled targets have
// overlapping skill paths (same primary path or cross-runtime discovery via
// also_scans). Quiet by design — points users at `doctor` for details.
func printSyncOverlapHint(targets map[string]config.TargetConfig, isProject, jsonOutput bool) {
	if jsonOutput {
		return
	}
	involved := config.DetectPathOverlap(targets, isProject)
	if len(involved) == 0 {
		return
	}
	ui.Warning("Skill path overlap across %d target(s) — run `skillshare doctor` for details", len(involved))
}
