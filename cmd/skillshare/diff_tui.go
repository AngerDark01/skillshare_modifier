package main

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"skillshare/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// diffExpandMsg is sent when async diff computation completes.
type diffExpandMsg struct {
	skill string
	diff  string
	files []fileDiffEntry
}

// ---------------------------------------------------------------------------
// Diff TUI — interactive diff browser: left panel target list, right panel
// detail showing sync/local differences. Browse-only (no mutating actions).
// ---------------------------------------------------------------------------

// diffMinSplitWidth is the minimum terminal width for horizontal split.
const diffMinSplitWidth = tuiMinSplitWidth

// --- List items ---

type diffTargetItem struct {
	result targetDiffResult
}

// diffExtraItem wraps an extraDiffResult for the bubbletea list.
type diffExtraItem struct {
	result extraDiffResult
}

func (i diffExtraItem) Title() string {
	r := i.result
	var icon, desc string
	if r.errMsg != "" {
		icon = "✗"
		desc = r.errMsg
	} else if r.synced {
		icon = "✓"
		desc = "synced"
	} else {
		icon = "~"
		desc = fmt.Sprintf("%d diff", len(r.items))
	}
	return fmt.Sprintf("%s %s → %s  %s", icon, r.extraName, shortenPath(r.targetPath), theme.Dim().Render(desc))
}

func (i diffExtraItem) Description() string { return "" }

func (i diffExtraItem) FilterValue() string { return i.result.extraName }

// diffSeparatorItem is a non-selectable visual separator / group header.
type diffSeparatorItem struct {
	label string
	count int  // 0 = no count displayed
	space bool // true = empty spacer row
}

func (s diffSeparatorItem) Title() string       { return s.label }
func (s diffSeparatorItem) Description() string { return "" }
func (s diffSeparatorItem) FilterValue() string { return "" }

// diffItemDelegate wraps prefixItemDelegate to render separators as group headers.
type diffItemDelegate struct {
	inner prefixItemDelegate
}

func (d diffItemDelegate) Height() int  { return d.inner.Height() }
func (d diffItemDelegate) Spacing() int { return d.inner.Spacing() }
func (d diffItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return d.inner.Update(msg, m)
}

func (d diffItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if sep, ok := item.(diffSeparatorItem); ok {
		width := m.Width()
		if width <= 0 {
			width = 40
		}
		renderDiffSeparatorRow(w, sep, width)
		return
	}
	d.inner.Render(w, m, index, item)
}

func renderDiffSeparatorRow(w io.Writer, sep diffSeparatorItem, width int) {
	if sep.space {
		fmt.Fprint(w, "")
		return
	}
	label := sep.label
	if sep.count > 0 {
		label += fmt.Sprintf(" (%d)", sep.count)
	}
	label = theme.Dim().Render(label)
	lineWidth := width - lipgloss.Width(label) - 3
	if lineWidth < 2 {
		lineWidth = 2
	}
	line := strings.Repeat("─", lineWidth)
	fmt.Fprint(w, theme.Dim().Render("─ ")+label+" "+theme.Dim().Render(line))
}

// skipDiffSeparator advances the list selection past diffSeparatorItem entries.
func skipDiffSeparator(l *list.Model, direction int) {
	items := l.Items()
	idx := l.Index()
	n := len(items)
	for idx >= 0 && idx < n {
		if _, isSep := items[idx].(diffSeparatorItem); !isSep {
			break
		}
		idx += direction
	}
	if idx >= 0 && idx < n {
		l.Select(idx)
	}
}

func (i diffTargetItem) Title() string {
	r := i.result
	if r.errMsg != "" {
		return fmt.Sprintf("%s %s", theme.Danger().Render("✗"), r.name)
	}
	if r.synced {
		return fmt.Sprintf("%s %s", theme.Success().Render("✓"), r.name)
	}
	var parts []string
	if r.syncCount > 0 {
		parts = append(parts, fmt.Sprintf("%d sync", r.syncCount))
	}
	if r.localCount > 0 {
		parts = append(parts, fmt.Sprintf("%d local", r.localCount))
	}
	desc := "0 diff(s)"
	if len(parts) > 0 {
		desc = strings.Join(parts, ", ")
	}
	return fmt.Sprintf("%s %s  %s", theme.Warning().Render("!"), r.name, theme.Dim().Render(desc))
}

func (i diffTargetItem) Description() string { return "" }

func (i diffTargetItem) FilterValue() string { return i.result.name }

// --- Model ---

type diffTUIModel struct {
	quitting   bool
	termWidth  int
	termHeight int

	// Data — sorted: error → diff → synced
	allItems  []targetDiffResult
	allExtras []extraDiffResult

	// Target list
	targetList list.Model

	// Filter
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Detail scroll (right panel)
	detailScroll int

	// Expand state — file-level diff for a specific skill
	expandedSkill string // skill name currently expanded
	expandedDiff  string // cached unified diff text
	expandedFiles []fileDiffEntry

	// Async loading — spinner shown while computing file diffs
	loading     bool
	loadSpinner spinner.Model

	// Cached detail data — recomputed only on selection change
	cachedIdx   int
	cachedItems []copyDiffEntry
	cachedCats  []actionCategory
}

func newDiffTUIModel(results []targetDiffResult, extrasSlice ...[]extraDiffResult) diffTUIModel {
	// Sort: error first, then diffs, then synced
	sorted := make([]targetDiffResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		ri, rj := sorted[i], sorted[j]
		oi, oj := diffSortOrder(ri), diffSortOrder(rj)
		if oi != oj {
			return oi < oj
		}
		return ri.name < rj.name
	})

	var extras []extraDiffResult
	if len(extrasSlice) > 0 {
		extras = extrasSlice[0]
	}

	// Build list items with group headers
	var listItems []list.Item
	listItems = append(listItems, diffSeparatorItem{label: "Targets", count: len(sorted)})
	for _, r := range sorted {
		listItems = append(listItems, diffTargetItem{result: r})
	}
	// Append extras with spacer + separator
	if len(extras) > 0 {
		listItems = append(listItems, diffSeparatorItem{space: true})
		listItems = append(listItems, diffSeparatorItem{label: "Extras", count: len(extras)})
		for _, r := range extras {
			listItems = append(listItems, diffExtraItem{result: r})
		}
	}

	delegate := diffItemDelegate{inner: newPrefixDelegate(false)}

	tl := list.New(listItems, delegate, 0, 0)
	var errN, diffN, syncN int
	for _, r := range sorted {
		switch {
		case r.errMsg != "":
			errN++
		case !r.synced:
			diffN++
		default:
			syncN++
		}
	}
	var titleParts []string
	if errN > 0 {
		titleParts = append(titleParts, fmt.Sprintf("%d err", errN))
	}
	if diffN > 0 {
		titleParts = append(titleParts, fmt.Sprintf("%d diff", diffN))
	}
	if syncN > 0 {
		titleParts = append(titleParts, fmt.Sprintf("%d ok", syncN))
	}
	tl.Title = fmt.Sprintf("Diff — %s", strings.Join(titleParts, ", "))
	tl.Styles.Title = theme.Title()
	tl.SetShowStatusBar(false)
	tl.SetFilteringEnabled(false)
	tl.SetShowHelp(false)
	tl.SetShowPagination(false)
	skipDiffSeparator(&tl, 1)

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = theme.Accent()
	fi.Cursor.Style = theme.Accent()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	return diffTUIModel{
		allItems:    sorted,
		allExtras:   extras,
		targetList:  tl,
		matchCount:  len(listItems),
		filterInput: fi,
		loadSpinner: sp,
	}
}

// diffSortOrder returns 0 for error, 1 for diffs, 2 for synced.
func diffSortOrder(r targetDiffResult) int {
	if r.errMsg != "" {
		return 0
	}
	if !r.synced {
		return 1
	}
	return 2
}

// refreshDetailCache recomputes sorted items and categories for the selected target.
func (m *diffTUIModel) refreshDetailCache() {
	idx := m.targetList.Index()
	if idx == m.cachedIdx && m.cachedItems != nil {
		return
	}
	m.cachedIdx = idx
	item, ok := m.targetList.SelectedItem().(diffTargetItem)
	if !ok || item.result.synced || item.result.errMsg != "" {
		m.cachedItems = nil
		m.cachedCats = nil
		return
	}
	items := make([]copyDiffEntry, len(item.result.items))
	copy(items, item.result.items)
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	m.cachedItems = items
	m.cachedCats = categorizeItems(items)
}

func (m diffTUIModel) Init() tea.Cmd { return nil }

func (m diffTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		lw := diffListWidth(m.termWidth)
		h := m.diffPanelHeight()
		m.targetList.SetSize(lw, h)
		m.refreshDetailCache()
		return m, nil

	case tea.KeyMsg:
		return m.handleDiffKey(msg)

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.loadSpinner, cmd = m.loadSpinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case diffExpandMsg:
		// Discard stale result if user navigated away (loading reset on nav)
		if !m.loading {
			return m, nil
		}
		m.loading = false
		m.expandedSkill = msg.skill
		m.expandedDiff = msg.diff
		m.expandedFiles = msg.files
		return m, nil
	}

	var cmd tea.Cmd
	m.targetList, cmd = m.targetList.Update(msg)
	return m, cmd
}

func (m diffTUIModel) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Filter mode
	if m.filtering {
		switch key {
		case "esc":
			m.filtering = false
			m.filterText = ""
			m.filterInput.SetValue("")
			m.applyDiffFilter()
			return m, nil
		case "enter":
			m.filtering = false
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		newVal := m.filterInput.Value()
		if newVal != m.filterText {
			m.filterText = newVal
			m.applyDiffFilter()
		}
		return m, cmd
	}

	// Normal keys
	switch key {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, textinput.Blink

	case "enter":
		if m.loading {
			return m, nil // ignore while loading
		}
		item, ok := m.targetList.SelectedItem().(diffTargetItem)
		if !ok {
			return m, nil
		}
		r := item.result
		if r.synced || r.errMsg != "" {
			return m, nil
		}
		// Toggle expand
		if m.expandedSkill != "" {
			m.expandedSkill = ""
			m.expandedDiff = ""
			m.expandedFiles = nil
			m.detailScroll = 0
			return m, nil
		}
		// Find first expandable skill (any with srcDir or dstDir)
		for i := range r.items {
			entry := r.items[i]
			if entry.srcDir != "" || entry.dstDir != "" {
				m.loading = true
				m.expandedSkill = ""
				m.expandedDiff = ""
				m.expandedFiles = nil
				m.detailScroll = 0
				return m, tea.Batch(m.loadSpinner.Tick, expandDiffCmd(&entry))
			}
		}
		return m, nil

	// Detail scroll
	case "ctrl+d":
		m.detailScroll += 5
		return m, nil
	case "ctrl+u":
		m.detailScroll -= 5
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
		return m, nil
	}

	// Reset detail scroll on list navigation
	prevIdx := m.targetList.Index()

	var cmd tea.Cmd
	m.targetList, cmd = m.targetList.Update(msg)

	// Skip separator items
	if m.targetList.Index() != prevIdx {
		direction := 1
		if m.targetList.Index() < prevIdx {
			direction = -1
		}
		skipDiffSeparator(&m.targetList, direction)

		m.detailScroll = 0
		m.expandedSkill = ""
		m.expandedDiff = ""
		m.expandedFiles = nil
		m.loading = false // cancel in-flight diff if navigating away
		m.refreshDetailCache()
	}

	return m, cmd
}

// --- Filter ---

func (m *diffTUIModel) applyDiffFilter() {
	needle := strings.ToLower(m.filterText)
	if needle == "" {
		var items []list.Item
		items = append(items, diffSeparatorItem{label: "Targets", count: len(m.allItems)})
		for _, r := range m.allItems {
			items = append(items, diffTargetItem{result: r})
		}
		if len(m.allExtras) > 0 {
			items = append(items, diffSeparatorItem{space: true})
			items = append(items, diffSeparatorItem{label: "Extras", count: len(m.allExtras)})
			for _, r := range m.allExtras {
				items = append(items, diffExtraItem{result: r})
			}
		}
		m.matchCount = len(items)
		m.targetList.SetItems(items)
		m.targetList.ResetSelected()
		skipDiffSeparator(&m.targetList, 1)
		m.cachedItems = nil // invalidate cache
		return
	}
	var matched []list.Item
	for _, r := range m.allItems {
		if strings.Contains(strings.ToLower(r.name), needle) {
			matched = append(matched, diffTargetItem{result: r})
		}
	}
	for _, r := range m.allExtras {
		if strings.Contains(strings.ToLower(r.extraName), needle) {
			matched = append(matched, diffExtraItem{result: r})
		}
	}
	m.matchCount = len(matched)
	m.targetList.SetItems(matched)
	m.targetList.ResetSelected()
	m.cachedItems = nil // invalidate cache
}

// --- Layout helpers ---

func diffListWidth(_ int) int { return 40 }

func diffDetailWidth(termWidth int) int {
	return max(termWidth-diffListWidth(termWidth)-3, 30)
}

func (m diffTUIModel) diffPanelHeight() int {
	return max(m.termHeight-4, 10)
}

// --- Views ---

func (m diffTUIModel) View() string {
	if m.quitting {
		return ""
	}

	if m.termWidth >= diffMinSplitWidth {
		return m.viewDiffHorizontal()
	}
	return m.viewDiffVertical()
}

func (m diffTUIModel) viewDiffHorizontal() string {
	var b strings.Builder

	panelHeight := m.diffPanelHeight()
	leftWidth := diffListWidth(m.termWidth)
	rightWidth := diffDetailWidth(m.termWidth)

	// Detail
	detailStr, scrollInfo := wrapAndScroll(m.buildDiffDetail(), rightWidth-1, m.detailScroll, panelHeight)

	body := renderHorizontalSplit(m.targetList.View(), detailStr, leftWidth, rightWidth, panelHeight)
	b.WriteString(body)
	b.WriteString("\n")

	// Filter bar
	b.WriteString(m.renderDiffFilterBar())

	// Help
	b.WriteString(theme.Dim().MarginLeft(2).Render(appendScrollInfo("↑↓ navigate  / filter  Enter expand  Ctrl+d/u scroll  q quit", scrollInfo)))
	b.WriteString("\n")

	return b.String()
}

func (m diffTUIModel) viewDiffVertical() string {
	var b strings.Builder

	b.WriteString(m.targetList.View())
	b.WriteString("\n")

	b.WriteString(m.renderDiffFilterBar())

	// Detail below list
	detailHeight := max(m.termHeight/3, 6)
	detailStr, scrollInfo := wrapAndScroll(m.buildDiffDetail(), m.termWidth, m.detailScroll, detailHeight)
	b.WriteString(detailStr)
	b.WriteString("\n")

	b.WriteString(theme.Dim().MarginLeft(2).Render(appendScrollInfo("↑↓ navigate  / filter  Enter expand  Ctrl+d/u scroll  q quit", scrollInfo)))
	b.WriteString("\n")

	return b.String()
}

func (m diffTUIModel) renderDiffFilterBar() string {
	pag := renderPageInfoFromPaginator(m.targetList.Paginator)
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0, "targets", pag,
	)
}

// --- Detail renderer ---

func (m diffTUIModel) buildExtraDetail(selected diffExtraItem) string {
	r := selected.result
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(14).Render(label))
		b.WriteString(lipgloss.NewStyle().Render(value))
		b.WriteString("\n")
	}

	row("Extra:  ", r.extraName)
	row("Target: ", shortenPath(r.targetPath))
	row("Mode:   ", r.mode)
	b.WriteString("\n")

	if r.errMsg != "" {
		b.WriteString(theme.Danger().Render("  " + r.errMsg))
		b.WriteString("\n")
		return b.String()
	}
	if r.synced {
		b.WriteString(theme.Success().Render("  ✓ Fully synced"))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(theme.Warning().Render(fmt.Sprintf("  %d difference(s):", len(r.items))))
	b.WriteString("\n")
	hasLocal := false
	for _, item := range r.items {
		var prefix string
		var style lipgloss.Style
		switch item.action {
		case "add":
			prefix, style = "+ ", theme.Success()
		case "remove":
			prefix, style = "- ", theme.Danger()
		case "modify":
			prefix, style = "~ ", theme.Accent()
			if item.reason == "not a symlink (local file)" {
				hasLocal = true
			}
		default:
			prefix, style = "  ", theme.Dim()
		}
		b.WriteString(style.Render(fmt.Sprintf("  %s%s  %s", prefix, item.file, item.reason)))
		b.WriteString("\n")
	}

	// Next Steps
	b.WriteString("\n")
	b.WriteString(theme.Title().Render("── Next Steps ──"))
	b.WriteString("\n")
	b.WriteString(theme.Accent().Render("  → skillshare sync extras"))
	b.WriteString("\n")
	if hasLocal {
		b.WriteString(theme.Accent().Render("  → skillshare extras collect " + r.extraName))
		b.WriteString("\n")
	}

	return b.String()
}

func (m diffTUIModel) buildDiffDetail() string {
	selectedItem := m.targetList.SelectedItem()
	switch selected := selectedItem.(type) {
	case diffExtraItem:
		return m.buildExtraDetail(selected)
	case diffSeparatorItem:
		return ""
	case diffTargetItem:
		// handled below
	default:
		return ""
	}
	item := selectedItem.(diffTargetItem)
	r := item.result

	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(14).Render(label))
		b.WriteString(lipgloss.NewStyle().Render(value))
		b.WriteString("\n")
	}

	row("Target:  ", r.name)
	row("Mode:    ", r.mode)
	if len(r.include) > 0 {
		row("Include: ", strings.Join(r.include, ", "))
	}
	if len(r.exclude) > 0 {
		row("Exclude: ", strings.Join(r.exclude, ", "))
	}
	if !r.srcMtime.IsZero() {
		row("Source:   ", r.srcMtime.Format("2006-01-02 15:04"))
	}
	if !r.dstMtime.IsZero() {
		row("Target:  ", r.dstMtime.Format("2006-01-02 15:04"))
	}

	b.WriteString("\n")

	// Error
	if r.errMsg != "" {
		b.WriteString(theme.Danger().Render("  " + r.errMsg))
		b.WriteString("\n")
		return b.String()
	}

	// Fully synced
	if r.synced {
		b.WriteString(theme.Success().Render("  ✓ Fully synced"))
		b.WriteString("\n")
		return b.String()
	}

	// Loading spinner
	if m.loading {
		b.WriteString(fmt.Sprintf("  %s Loading diff...\n", m.loadSpinner.View()))
		return b.String()
	}

	// Build agent name set for [A] badge rendering
	agentNames := make(map[string]bool, len(m.cachedItems))
	for _, item := range m.cachedItems {
		if item.kind == "agent" {
			agentNames[item.name] = true
		}
	}

	// Use cached sorted categories (refreshed on selection change)
	cats := m.cachedCats
	for _, cat := range cats {
		n := len(cat.names)
		skillWord := "skills"
		if n == 1 {
			skillWord = "skill"
		}

		var kindStyle lipgloss.Style
		switch cat.kind {
		case "new", "restore":
			kindStyle = theme.Success()
		case "modified":
			kindStyle = theme.Accent()
		case "override":
			kindStyle = theme.Warning()
		case "orphan":
			kindStyle = theme.Danger()
		case "local":
			kindStyle = theme.Dim()
		case "warn":
			kindStyle = theme.Danger()
		default:
			kindStyle = theme.Dim()
		}

		header := fmt.Sprintf("  %s %d %s:", cat.label, n, skillWord)
		b.WriteString(kindStyle.Render(header))
		b.WriteString("\n")

		if cat.expand {
			for _, name := range cat.names {
				if agentNames[name] {
					b.WriteString("    " + theme.Accent().Render("[A]") + " " + theme.Dim().Render(name))
				} else {
					b.WriteString(theme.Dim().Render("    " + name))
				}
				b.WriteString("\n")
			}
		}
	}

	// File list + diff content (only shown after Enter toggle)
	if m.expandedSkill != "" {
		if len(m.expandedFiles) > 0 {
			b.WriteString("\n")
			b.WriteString(theme.Dim().Render(fmt.Sprintf("── %s files ──", m.expandedSkill)))
			b.WriteString("\n")
			for _, f := range m.expandedFiles {
				var icon string
				var style lipgloss.Style
				switch f.Action {
				case "add":
					icon, style = "+", theme.Success()
				case "delete":
					icon, style = "-", theme.Danger()
				case "modify":
					icon, style = "~", theme.Accent()
				default:
					icon, style = "?", theme.Dim()
				}
				b.WriteString(style.Render(fmt.Sprintf("  %s %s", icon, f.RelPath)))
				b.WriteString("\n")
			}
		}

		// Unified diff content
		if m.expandedDiff != "" {
			b.WriteString("\n")
			b.WriteString(theme.Dim().Render(fmt.Sprintf("── %s diff ──", m.expandedSkill)))
			b.WriteString("\n")
			for _, line := range strings.Split(strings.TrimRight(m.expandedDiff, "\n"), "\n") {
				switch {
				case strings.HasPrefix(line, "+ "):
					b.WriteString(theme.Success().Render(line))
				case strings.HasPrefix(line, "- "):
					b.WriteString(theme.Danger().Render(line))
				case strings.HasPrefix(line, "--- "):
					b.WriteString(theme.Accent().Render(line))
				default:
					b.WriteString(theme.Dim().Render(line))
				}
				b.WriteString("\n")
			}
		}
		if len(m.expandedFiles) == 0 && m.expandedDiff == "" {
			b.WriteString("\n")
			b.WriteString(theme.Dim().Render("  (No file-level diff available)"))
			b.WriteString("\n")
		}
	}

	// Next Steps
	var hints []string
	for _, cat := range cats {
		switch cat.kind {
		case "new", "modified", "restore", "orphan":
			hints = append(hints, "sync")
		case "override":
			hints = append(hints, "sync --force")
		case "local":
			hints = append(hints, "collect")
		}
	}
	if len(hints) > 0 {
		b.WriteString("\n")
		b.WriteString(theme.Title().Render("── Next Steps ──"))
		b.WriteString("\n")
		seen := map[string]bool{}
		for _, h := range hints {
			if seen[h] {
				continue
			}
			seen[h] = true
			b.WriteString(theme.Accent().Render(fmt.Sprintf("  → skillshare %s", h)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// --- Async diff ---

// expandDiffCmd returns a tea.Cmd that computes file diffs in a goroutine.
// For items with srcDir, also generates parallel unified diffs for modified files.
// For items with only dstDir (local only), populates file list without diff.
func expandDiffCmd(entry *copyDiffEntry) tea.Cmd {
	return func() tea.Msg {
		entry.ensureFiles()

		// For remove/local-only items without srcDir, just return file list (no diff)
		if entry.srcDir == "" {
			return diffExpandMsg{skill: entry.name, files: entry.files}
		}

		// Collect modified files for diff
		var modFiles []fileDiffEntry
		for _, f := range entry.files {
			if f.Action == "modify" {
				modFiles = append(modFiles, f)
			}
		}

		// Parallel diff computation
		results := make([]string, len(modFiles))
		var wg sync.WaitGroup
		for i, f := range modFiles {
			wg.Add(1)
			go func(idx int, fe fileDiffEntry) {
				defer wg.Done()
				src := filepath.Join(entry.srcDir, fe.RelPath)
				dst := filepath.Join(entry.dstDir, fe.RelPath)
				results[idx] = generateUnifiedDiff(src, dst)
			}(i, f)
		}
		wg.Wait()

		// Combine results preserving order
		var buf strings.Builder
		for i, f := range modFiles {
			if results[i] != "" {
				buf.WriteString(fmt.Sprintf("--- %s\n", f.RelPath))
				buf.WriteString(results[i])
			}
		}

		return diffExpandMsg{
			skill: entry.name,
			diff:  buf.String(),
			files: entry.files,
		}
	}
}

// --- Entry point ---

func runDiffTUI(results []targetDiffResult, extrasResults ...[]extraDiffResult) error {
	model := newDiffTUIModel(results, extrasResults...)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
