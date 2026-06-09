package main

import (
	"fmt"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/targetsummary"
)

func applyTargetListAgentSummary(item *targetListJSONItem, summary *targetsummary.AgentSummary) {
	if summary == nil {
		return
	}

	item.AgentPath = summary.Path
	item.AgentMode = summary.Mode
	item.AgentSync = formatTargetAgentSyncSummary(summary)
	item.AgentInclude = append([]string(nil), summary.Include...)
	item.AgentExclude = append([]string(nil), summary.Exclude...)
	item.AgentLinkedCount = intPtr(summary.ManagedCount)
	item.AgentLocalCount = intPtr(summary.LocalCount)
	item.AgentExpectedCount = intPtr(summary.ExpectedCount)
}

func newTargetListJSONItem(item targetTUIItem) targetListJSONItem {
	sc := item.target.SkillsConfig()

	jsonItem := targetListJSONItem{
		Name:         item.name,
		Path:         sc.Path,
		Mode:         sync.EffectiveMode(sc.Mode),
		TargetNaming: config.EffectiveTargetNaming(sc.TargetNaming),
		Sync:         item.skillSync,
		Include:      append([]string(nil), sc.Include...),
		Exclude:      append([]string(nil), sc.Exclude...),
	}
	applyTargetListAgentSummary(&jsonItem, item.agentSummary)
	return jsonItem
}

func printTargetAgentSection(summary *targetsummary.AgentSummary) {
	if summary == nil {
		return
	}

	displayPath := summary.DisplayPath
	if displayPath == "" {
		displayPath = summary.Path
	}

	fmt.Println("  Agents:")
	fmt.Printf("    Path:    %s\n", displayPath)
	fmt.Printf("    Mode:    %s\n", summary.Mode)
	fmt.Printf("    Status:  %s\n", formatTargetAgentSyncSummary(summary))
	if summary.Mode == "symlink" {
		fmt.Println("    Filters: ignored in symlink mode")
		return
	}
	fmt.Printf("    Include: %s\n", formatFilterList(summary.Include))
	fmt.Printf("    Exclude: %s\n", formatFilterList(summary.Exclude))
}

func printTargetListPlain(items []targetTUIItem) {
	for idx, item := range items {
		if idx > 0 {
			fmt.Println()
		}

		sc := item.target.SkillsConfig()
		displayPath := item.displayPath
		if displayPath == "" {
			displayPath = sc.Path
		}

		fmt.Printf("  %s\n", item.name)
		fmt.Println("    Skills:")
		fmt.Printf("      Path:    %s\n", displayPath)
		fmt.Printf("      Mode:    %s\n", sync.EffectiveMode(sc.Mode))
		fmt.Printf("      Naming:  %s\n", config.EffectiveTargetNaming(sc.TargetNaming))
		fmt.Printf("      Sync:    %s\n", item.skillSync)
		if len(sc.Include) == 0 && len(sc.Exclude) == 0 {
			fmt.Println("      No include/exclude filters")
		} else {
			fmt.Printf("      Include: %s\n", formatFilterList(sc.Include))
			fmt.Printf("      Exclude: %s\n", formatFilterList(sc.Exclude))
		}

		if item.agentSummary == nil {
			continue
		}

		agentPath := item.agentSummary.DisplayPath
		if agentPath == "" {
			agentPath = item.agentSummary.Path
		}
		fmt.Println("    Agents:")
		fmt.Printf("      Path:    %s\n", agentPath)
		fmt.Printf("      Mode:    %s\n", item.agentSummary.Mode)
		fmt.Printf("      Sync:    %s\n", formatTargetAgentSyncSummary(item.agentSummary))
		if item.agentSummary.Mode == "symlink" {
			fmt.Println("      Filters: ignored in symlink mode")
		} else if len(item.agentSummary.Include) == 0 && len(item.agentSummary.Exclude) == 0 {
			fmt.Println("      No agent include/exclude filters")
		} else {
			fmt.Printf("      Include: %s\n", formatFilterList(item.agentSummary.Include))
			fmt.Printf("      Exclude: %s\n", formatFilterList(item.agentSummary.Exclude))
		}
	}
}

func formatTargetAgentSyncSummary(summary *targetsummary.AgentSummary) string {
	if summary == nil {
		return ""
	}

	if summary.ExpectedCount == 0 {
		counts := joinAgentCounts(summary.ManagedCount, targetAgentCountLabel(summary.Mode), summary.LocalCount)
		if counts != "" {
			return fmt.Sprintf("no source agents yet (%s)", counts)
		}
		return "no source agents yet"
	}

	summaryText := fmt.Sprintf("%d/%d %s", summary.ManagedCount, summary.ExpectedCount, targetAgentCountLabel(summary.Mode))
	if summary.LocalCount > 0 {
		summaryText += fmt.Sprintf(", %d local", summary.LocalCount)
	}
	if summary.Mode == "symlink" {
		return summaryText + " (directory symlink)"
	}
	return summaryText
}

func joinAgentCounts(managed int, managedLabel string, local int) string {
	var parts []string
	if managed > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", managed, managedLabel))
	}
	if local > 0 {
		parts = append(parts, fmt.Sprintf("%d local", local))
	}
	return strings.Join(parts, ", ")
}

func targetAgentCountLabel(mode string) string {
	if mode == "copy" {
		return "managed"
	}
	return "linked"
}

func intPtr(v int) *int {
	return &v
}
