import { useState, useEffect, useCallback, useRef, useMemo, memo } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Virtuoso } from 'react-virtuoso';
import {
  Check,
  CheckSquare,
  ChevronDown,
  ChevronRight,
  Filter,
  Folder,
  FolderOpen,
  Info,
  ListChecks,
  Minus,
  PackageOpen,
  RotateCcw,
  Search,
  Square,
  X,
} from 'lucide-react';
import { api } from '../api/client';
import type { SyncMatrixEntry, Target } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useToast } from '../components/Toast';
import Card from '../components/Card';
import Button from '../components/Button';
import Spinner from '../components/Spinner';
import PageHeader from '../components/PageHeader';
import EmptyState from '../components/EmptyState';
import KindBadge from '../components/KindBadge';
import { radius } from '../design';
import { formatPreviewResourceName } from '../lib/resourceNames';
import { syncMatrixReasonText } from '../lib/syncMatrixText';
import { useT } from '../i18n';

type FilterKind = 'skill' | 'agent';
type CheckState = 'checked' | 'partial' | 'empty';

const EMPTY_SELECTION_PATTERN = '__skillshare_empty_selection__';

interface SkillLeaf {
  entry: SyncMatrixEntry;
  key: string;
  label: string;
  displayName: string;
  selected: boolean;
  selectable: boolean;
}

interface FolderNode {
  key: string;
  label: string;
  pathParts: string[];
  folders: Map<string, FolderNode>;
  skills: SkillLeaf[];
  skillNames: string[];
  totalCount: number;
  selectedCount: number;
}

type SelectionRow =
  | {
      type: 'folder';
      key: string;
      depth: number;
      label: string;
      totalCount: number;
      selectedCount: number;
      skillNames: string[];
    }
  | {
      type: 'skill';
      key: string;
      depth: number;
      leaf: SkillLeaf;
    };

function isSelectableEntry(entry: SyncMatrixEntry): boolean {
  return entry.status !== 'skill_target_mismatch' && entry.status !== 'na';
}

function toPatternName(flatName: string, kind: FilterKind): string {
  return kind === 'agent' ? flatName.replace(/\.md$/i, '') : flatName;
}

function patternParts(flatName: string, kind: FilterKind): string[] {
  return toPatternName(flatName, kind).split('__').filter(Boolean);
}

function displayParts(entry: SyncMatrixEntry, kind: FilterKind): string[] {
  return formatPreviewResourceName(entry.skill, kind).split('/').filter(Boolean);
}

function createFolderNode(label: string, pathParts: string[]): FolderNode {
  return {
    key: pathParts.join('__'),
    label,
    pathParts,
    folders: new Map(),
    skills: [],
    skillNames: [],
    totalCount: 0,
    selectedCount: 0,
  };
}

function sortedFolders(node: FolderNode): FolderNode[] {
  return Array.from(node.folders.values()).sort((a, b) => a.label.localeCompare(b.label));
}

function finalizeFolderNode(node: FolderNode): void {
  node.skills.sort((a, b) => a.label.localeCompare(b.label));

  let totalCount = 0;
  let selectedCount = 0;
  const skillNames: string[] = [];

  for (const child of node.folders.values()) {
    finalizeFolderNode(child);
    totalCount += child.totalCount;
    selectedCount += child.selectedCount;
    skillNames.push(...child.skillNames);
  }

  for (const skill of node.skills) {
    if (!skill.selectable) continue;
    totalCount += 1;
    skillNames.push(skill.entry.skill);
    if (skill.selected) selectedCount += 1;
  }

  node.totalCount = totalCount;
  node.selectedCount = selectedCount;
  node.skillNames = skillNames;
}

function buildSelectionTree(
  entries: SyncMatrixEntry[],
  selectedSkills: Set<string>,
  kind: FilterKind,
): FolderNode {
  const root = createFolderNode('', []);

  for (const entry of entries) {
    const parts = patternParts(entry.skill, kind);
    const labels = displayParts(entry, kind);
    if (parts.length === 0) continue;

    let node = root;
    for (let i = 0; i < parts.length - 1; i += 1) {
      const pathParts = parts.slice(0, i + 1);
      const key = pathParts.join('__');
      let child = node.folders.get(key);
      if (!child) {
        child = createFolderNode(labels[i] ?? parts[i], pathParts);
        node.folders.set(key, child);
      }
      node = child;
    }

    const label = labels[labels.length - 1] ?? parts[parts.length - 1];
    const selectable = isSelectableEntry(entry);
    node.skills.push({
      entry,
      key: entry.skill,
      label,
      displayName: formatPreviewResourceName(entry.skill, kind),
      selected: selectable && selectedSkills.has(entry.skill),
      selectable,
    });
  }

  finalizeFolderNode(root);
  return root;
}

function flattenSelectionTree(
  node: FolderNode,
  expandedFolders: Set<string>,
  rows: SelectionRow[] = [],
  depth = 0,
  forceOpen = false,
): SelectionRow[] {
  for (const folder of sortedFolders(node)) {
    rows.push({
      type: 'folder',
      key: folder.key,
      depth,
      label: folder.label,
      totalCount: folder.totalCount,
      selectedCount: folder.selectedCount,
      skillNames: folder.skillNames,
    });

    if (forceOpen || expandedFolders.has(folder.key)) {
      flattenSelectionTree(folder, expandedFolders, rows, depth + 1, forceOpen);
    }
  }

  for (const skill of node.skills) {
    rows.push({
      type: 'skill',
      key: skill.key,
      depth,
      leaf: skill,
    });
  }

  return rows;
}

function selectedPatternFilters(
  selectedSkills: Set<string>,
  entries: SyncMatrixEntry[],
  kind: FilterKind,
): { include: string[]; exclude: string[] } {
  const selectableEntries = entries.filter(isSelectableEntry);
  const allNames = selectableEntries.map((entry) => entry.skill);
  const selectedNames = new Set(allNames.filter((name) => selectedSkills.has(name)));

  if (allNames.length === 0 || selectedNames.size === allNames.length) {
    return { include: [], exclude: [] };
  }

  if (selectedNames.size === 0) {
    return { include: [EMPTY_SELECTION_PATTERN], exclude: [] };
  }

  const tree = buildSelectionTree(selectableEntries, selectedNames, kind);
  const include = compressSelectedNode(tree, kind, true);
  return { include, exclude: [] };
}

function compressSelectedNode(node: FolderNode, kind: FilterKind, isRoot = false): string[] {
  if (!isRoot && node.totalCount > 0 && node.selectedCount === node.totalCount) {
    return [`${node.key}__*`];
  }

  const patterns: string[] = [];
  for (const folder of sortedFolders(node)) {
    patterns.push(...compressSelectedNode(folder, kind));
  }
  for (const skill of node.skills) {
    if (skill.selectable && skill.selected) {
      patterns.push(toPatternName(skill.entry.skill, kind));
    }
  }
  return patterns;
}

function topLevelFolderKeys(entries: SyncMatrixEntry[], kind: FilterKind): string[] {
  const keys = new Set<string>();
  for (const entry of entries) {
    const parts = patternParts(entry.skill, kind);
    if (parts.length > 1) keys.add(parts[0]);
  }
  return Array.from(keys).sort((a, b) => a.localeCompare(b));
}

function checkState(selectedCount: number, totalCount: number): CheckState {
  if (totalCount === 0 || selectedCount === 0) return 'empty';
  if (selectedCount === totalCount) return 'checked';
  return 'partial';
}

export default function FilterStudioPage() {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const t = useT();

  const kind: FilterKind = searchParams.get('kind') === 'agent' ? 'agent' : 'skill';
  const kindLabel = kind === 'agent' ? 'agents' : 'skills';

  const targetsQuery = useQuery({
    queryKey: queryKeys.targets.all,
    queryFn: () => api.listTargets(),
    staleTime: staleTimes.targets,
  });

  const target = useMemo(
    () => targetsQuery.data?.targets.find((t) => t.name === name),
    [targetsQuery.data, name],
  );

  if (targetsQuery.isPending) {
    return (
      <div className="flex items-center justify-center py-20">
        <Spinner size="lg" />
      </div>
    );
  }

  if (!target) {
    return (
      <div className="animate-fade-in">
        <EmptyState
          icon={Filter}
          title={t('filterStudio.targetNotFound.title', { name: name ?? '' })}
          description={t('filterStudio.targetNotFound.description')}
          action={
            <Button variant="secondary" size="sm" onClick={() => navigate('/targets')}>
              {t('targets.backToPicker')}
            </Button>
          }
        />
      </div>
    );
  }

  return (
    <FilterStudioEditor
      key={`${target.name}:${kind}`}
      target={target}
      targetName={name ?? target.name}
      kind={kind}
      kindLabel={kindLabel}
    />
  );
}

function savedTargetFilters(target: Target, kind: FilterKind): { include: string[]; exclude: string[] } {
  return kind === 'agent'
    ? { include: target.agentInclude ?? [], exclude: target.agentExclude ?? [] }
    : { include: target.include ?? [], exclude: target.exclude ?? [] };
}

function FilterStudioEditor({
  target,
  targetName,
  kind,
  kindLabel,
}: {
  target: Target;
  targetName: string;
  kind: FilterKind;
  kindLabel: string;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const t = useT();
  const savedFilters = useMemo(() => savedTargetFilters(target, kind), [target, kind]);

  const [include, setInclude] = useState<string[]>(() => savedFilters.include);
  const [exclude, setExclude] = useState<string[]>(() => savedFilters.exclude);
  const [selectedSkills, setSelectedSkills] = useState<Set<string> | null>(null);
  const [expandedFolders, setExpandedFolders] = useState<Set<string> | null>(null);

  const [preview, setPreview] = useState<SyncMatrixEntry[]>([]);
  const [previewLoading, setPreviewLoading] = useState(true);
  const [previewReady, setPreviewReady] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const fetchPreview = useCallback(
    async (inc: string[], exc: string[]) => {
      setPreviewLoading(true);
      try {
        const skillInc = kind === 'skill' ? inc : [];
        const skillExc = kind === 'skill' ? exc : [];
        const agentInc = kind === 'agent' ? inc : [];
        const agentExc = kind === 'agent' ? exc : [];
        const res = await api.previewSyncMatrix(targetName, skillInc, skillExc, agentInc, agentExc);
        setPreview(res.entries);
        setPreviewReady(true);
      } catch {
        setPreviewReady(true);
      } finally {
        setPreviewLoading(false);
      }
    },
    [targetName, kind],
  );

  useEffect(() => {
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => fetchPreview(include, exclude), 300);
    return () => clearTimeout(debounceRef.current);
  }, [include, exclude, fetchPreview]);

  const kindPreview = useMemo(() => {
    if (kind === 'agent') return preview.filter((e) => e.kind === 'agent');
    return preview.filter((e) => e.kind !== 'agent');
  }, [preview, kind]);

  const currentSelected = useMemo(() => {
    if (selectedSkills) return selectedSkills;
    return new Set(
      kindPreview
        .filter((entry) => isSelectableEntry(entry) && entry.status === 'synced')
        .map((entry) => entry.skill),
    );
  }, [kindPreview, selectedSkills]);

  const hasChanges = useMemo(() => {
    return (
      JSON.stringify(include) !== JSON.stringify(savedFilters.include) ||
      JSON.stringify(exclude) !== JSON.stringify(savedFilters.exclude)
    );
  }, [include, exclude, savedFilters]);

  const [saving, setSaving] = useState(false);

  const handleSave = async (goBack: boolean) => {
    setSaving(true);
    try {
      const payload = kind === 'agent'
        ? { agent_include: include, agent_exclude: exclude }
        : { include, exclude };
      await api.updateTarget(targetName, payload);
      toast(t('filterStudio.toast.saved', { kind: kind === 'agent' ? 'Agent' : 'Skill', name: targetName }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.syncMatrix() });
      if (goBack) navigate('/targets');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const [previewSearch, setPreviewSearch] = useState('');
  const filteredPreview = useMemo(() => {
    if (!previewSearch) return kindPreview;
    const q = previewSearch.toLowerCase();
    return kindPreview.filter((e) => {
      const displayName = formatPreviewResourceName(e.skill, kind);
      return e.skill.toLowerCase().includes(q) || displayName.toLowerCase().includes(q);
    });
  }, [kindPreview, previewSearch, kind]);

  const selectionTree = useMemo(
    () => buildSelectionTree(filteredPreview, currentSelected, kind),
    [filteredPreview, currentSelected, kind],
  );

  const defaultExpandedFolders = useMemo(
    () => new Set(topLevelFolderKeys(kindPreview, kind)),
    [kindPreview, kind],
  );

  const effectiveExpandedFolders = expandedFolders ?? defaultExpandedFolders;

  const selectionRows = useMemo(
    () => flattenSelectionTree(selectionTree, effectiveExpandedFolders, [], 0, Boolean(previewSearch)),
    [selectionTree, effectiveExpandedFolders, previewSearch],
  );

  const selectableEntries = useMemo(
    () => kindPreview.filter(isSelectableEntry),
    [kindPreview],
  );

  const selectedCount = useMemo(
    () => selectableEntries.filter((entry) => currentSelected.has(entry.skill)).length,
    [selectableEntries, currentSelected],
  );

  const applySelection = useCallback((nextSelected: Set<string>) => {
    setSelectedSkills(nextSelected);
    const nextFilters = selectedPatternFilters(nextSelected, kindPreview, kind);
    setInclude(nextFilters.include);
    setExclude(nextFilters.exclude);
  }, [kindPreview, kind]);

  const toggleSkill = useCallback((entry: SyncMatrixEntry) => {
    if (!isSelectableEntry(entry)) return;
    const next = new Set(currentSelected);
    if (next.has(entry.skill)) {
      next.delete(entry.skill);
    } else {
      next.add(entry.skill);
    }
    applySelection(next);
  }, [applySelection, currentSelected]);

  const toggleFolder = useCallback((skillNames: string[]) => {
    if (skillNames.length === 0) return;
    const next = new Set(currentSelected);
    const allSelected = skillNames.every((skill) => next.has(skill));
    for (const skill of skillNames) {
      if (allSelected) {
        next.delete(skill);
      } else {
        next.add(skill);
      }
    }
    applySelection(next);
  }, [applySelection, currentSelected]);

  const selectAll = useCallback(() => {
    applySelection(new Set(selectableEntries.map((entry) => entry.skill)));
  }, [applySelection, selectableEntries]);

  const clearSelection = useCallback(() => {
    applySelection(new Set());
  }, [applySelection]);

  const resetDraft = useCallback(() => {
    setSelectedSkills(null);
    setExpandedFolders(null);
    setPreviewReady(false);
    setInclude(savedFilters.include);
    setExclude(savedFilters.exclude);
  }, [savedFilters]);

  const toggleExpand = useCallback((key: string) => {
    setExpandedFolders((prev) => {
      const next = new Set(prev ?? defaultExpandedFolders);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }, [defaultExpandedFolders]);

  const expandAll = useCallback(() => {
    const allKeys = new Set<string>();
    const visit = (node: FolderNode) => {
      for (const child of node.folders.values()) {
        allKeys.add(child.key);
        visit(child);
      }
    };
    visit(selectionTree);
    setExpandedFolders(allKeys);
  }, [selectionTree]);

  const collapseAll = useCallback(() => {
    setExpandedFolders(new Set());
  }, []);

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<ListChecks size={24} strokeWidth={2.5} />}
        title={t('filterStudio.title')}
        subtitle={
          <span className="inline-flex items-center gap-2">
            <KindBadge kind={kind} />
            <span>{t('filterStudio.routeSubtitle', { kindLabel, name: targetName })}</span>
          </span>
        }
        backTo="/targets"
        actions={
          <>
            <Button
              variant="primary"
              size="sm"
              onClick={() => handleSave(false)}
              loading={saving}
              disabled={!hasChanges}
            >
              {t('filterStudio.save')}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => handleSave(true)}
              loading={saving}
              disabled={!hasChanges}
            >
              {t('filterStudio.saveAndBack')}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={resetDraft}
              disabled={!hasChanges}
            >
              <RotateCcw size={14} strokeWidth={2.5} />
              {t('filterStudio.reset')}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => navigate('/targets')}>
              {t('filterStudio.cancel')}
            </Button>
            {hasChanges && (
              <span className="text-xs text-warning">{t('filterStudio.hasChanges')}</span>
            )}
          </>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1.35fr)_minmax(20rem,0.65fr)] gap-6">
        <Card>
          <div className="flex flex-col gap-3 mb-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 className="font-bold text-pencil">
                {kind === 'agent' ? t('filterStudio.agentSelectorTitle') : t('filterStudio.skillSelectorTitle')}
              </h3>
              <p className="text-sm text-pencil-light mt-1">
                {t('filterStudio.selectedSummary', { selected: selectedCount, total: selectableEntries.length })}
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="secondary" size="xs" onClick={selectAll} disabled={selectableEntries.length === 0}>
                {t('filterStudio.selectAll')}
              </Button>
              <Button variant="ghost" size="xs" onClick={clearSelection} disabled={selectableEntries.length === 0}>
                {t('filterStudio.clearSelection')}
              </Button>
              <Button variant="ghost" size="xs" onClick={expandAll} disabled={selectionRows.length === 0}>
                {t('filterStudio.expandAll')}
              </Button>
              <Button variant="ghost" size="xs" onClick={collapseAll} disabled={selectionRows.length === 0}>
                {t('filterStudio.collapseAll')}
              </Button>
            </div>
          </div>

          <div className="relative mb-3">
            <Search size={14} strokeWidth={2.5} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-pencil-light" />
            <input
              type="text"
              value={previewSearch}
              onChange={(e) => setPreviewSearch(e.target.value)}
              placeholder={t('filterStudio.searchPlaceholder', { kindLabel })}
              className="w-full pl-8 pr-3 py-1.5 text-sm text-pencil bg-surface border-2 border-muted font-mono placeholder:text-muted-dark focus:border-pencil focus:outline-none"
              style={{ borderRadius: radius.sm }}
            />
          </div>

          {kindPreview.length === 0 && !previewLoading ? (
            <EmptyState
              icon={PackageOpen}
              title={t('filterStudio.noPreview.title', { kindLabel })}
              description={t('filterStudio.noPreview.description', { kindLabel })}
            />
          ) : (
            <div
              className="border-2 border-dashed border-pencil-light/30"
              style={{ borderRadius: radius.md }}
            >
              {previewLoading && !previewReady ? (
                <div className="flex items-center justify-center h-[30rem]">
                  <Spinner size="lg" />
                </div>
              ) : filteredPreview.length === 0 && previewSearch ? (
                <p className="text-sm text-pencil-light text-center py-6">
                  {t('filterStudio.noSearchMatch', { kindLabel, query: previewSearch })}
                </p>
              ) : (
                <Virtuoso
                  style={{ height: '30rem' }}
                  totalCount={selectionRows.length}
                  overscan={300}
                  itemContent={(index) => (
                    <SelectionRowView
                      row={selectionRows[index]}
                      kind={kind}
                      expanded={selectionRows[index]?.type === 'folder' && effectiveExpandedFolders.has(selectionRows[index].key)}
                      onToggleExpand={toggleExpand}
                      onToggleFolder={toggleFolder}
                      onToggleSkill={toggleSkill}
                    />
                  )}
                />
              )}
            </div>
          )}
        </Card>

        <Card>
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-bold text-pencil">{t('filterStudio.assignmentPreview')}</h3>
            {previewLoading && <Spinner size="sm" />}
          </div>

          <div className="grid grid-cols-2 gap-3 mb-4">
            <StatBox label={t('filterStudio.selectedLabel')} value={String(selectedCount)} />
            <StatBox label={t('filterStudio.availableLabel')} value={String(selectableEntries.length)} />
          </div>

          <div className="space-y-3">
            <div>
              <div className="flex items-center justify-between mb-2">
                <h4 className="text-sm font-bold text-pencil-light">{t('filterStudio.generatedInclude')}</h4>
                <span className="text-xs text-pencil-light">{include.length}</span>
              </div>
              <PatternList
                patterns={include}
                emptyLabel={selectedCount === selectableEntries.length
                  ? t('filterStudio.allSelectedRule')
                  : t('filterStudio.noPatternRule')}
              />
            </div>

            {exclude.length > 0 && (
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h4 className="text-sm font-bold text-pencil-light">{t('filterStudio.generatedExclude')}</h4>
                  <span className="text-xs text-pencil-light">{exclude.length}</span>
                </div>
                <PatternList patterns={exclude} emptyLabel="" danger />
              </div>
            )}
          </div>

          <div
            className="mt-4 p-3 bg-muted/10 border border-muted"
            style={{ borderRadius: radius.md }}
          >
            <div className="flex items-center gap-2 text-sm font-bold text-pencil">
              <StatusIcon status={selectedCount > 0 ? 'synced' : 'not_included'} />
              {t('filterStudio.syncCount', { synced: selectedCount, total: selectableEntries.length, kindLabel })}
            </div>
            {previewSearch && (
              <p className="text-xs text-pencil-light mt-1">
                {t('filterStudio.syncCountSearch', { count: filteredPreview.length })}
              </p>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}

const SelectionRowView = memo(function SelectionRowView({
  row,
  kind,
  expanded,
  onToggleExpand,
  onToggleFolder,
  onToggleSkill,
}: {
  row: SelectionRow;
  kind: FilterKind;
  expanded: boolean;
  onToggleExpand: (key: string) => void;
  onToggleFolder: (skillNames: string[]) => void;
  onToggleSkill: (entry: SyncMatrixEntry) => void;
}) {
  const t = useT();
  const paddingLeft = `${0.75 + row.depth * 1.1}rem`;

  if (row.type === 'folder') {
    const state = checkState(row.selectedCount, row.totalCount);
    const disabled = row.totalCount === 0;
    return (
      <div
        className="flex items-center gap-2 px-3 py-2 border-b border-dashed border-pencil-light/30 text-sm bg-muted/10"
        style={{ paddingLeft }}
      >
        <button
          type="button"
          onClick={() => onToggleExpand(row.key)}
          className="text-pencil-light hover:text-pencil"
          aria-label={expanded ? t('filterStudio.collapseFolder') : t('filterStudio.expandFolder')}
        >
          {expanded ? <ChevronDown size={15} strokeWidth={2.5} /> : <ChevronRight size={15} strokeWidth={2.5} />}
        </button>
        <button
          type="button"
          onClick={() => onToggleFolder(row.skillNames)}
          disabled={disabled}
          className="text-pencil hover:text-pencil-light disabled:opacity-40"
          aria-label={t('filterStudio.toggleFolder', { name: row.label })}
        >
          <CheckGlyph state={state} />
        </button>
        {expanded ? <FolderOpen size={15} strokeWidth={2.5} className="text-pencil-light" /> : <Folder size={15} strokeWidth={2.5} className="text-pencil-light" />}
        <span className="font-mono text-pencil flex-1 min-w-0 truncate">{row.label}</span>
        <span className="text-xs text-pencil-light shrink-0">
          {row.selectedCount}/{row.totalCount}
        </span>
      </div>
    );
  }

  const { leaf } = row;
  const isMismatch = leaf.entry.status === 'skill_target_mismatch';
  return (
    <div
      role={leaf.selectable ? 'button' : undefined}
      tabIndex={leaf.selectable ? 0 : undefined}
      onClick={leaf.selectable ? () => onToggleSkill(leaf.entry) : undefined}
      onKeyDown={leaf.selectable ? (e) => { if (e.key === 'Enter') onToggleSkill(leaf.entry); } : undefined}
      className={`
        flex items-center gap-2 px-3 py-2 border-b border-dashed border-pencil-light/30 text-sm
        ${leaf.selectable ? 'cursor-pointer hover:bg-muted/20 transition-all duration-150' : 'cursor-default opacity-70'}
      `}
      style={{ paddingLeft }}
      title={
        isMismatch
          ? syncMatrixReasonText(leaf.entry, t)
          : leaf.selected
            ? t('filterStudio.clickToRemove', { kind })
            : t('filterStudio.clickToAdd', { kind })
      }
    >
      <CheckGlyph state={leaf.selected ? 'checked' : 'empty'} />
      <span className="font-mono text-pencil flex-1 min-w-0 truncate">
        {leaf.label}
      </span>
      {isMismatch && (
        <span className="flex items-center gap-1 text-xs text-pencil-light shrink-0">
          <Info size={12} strokeWidth={2.5} />
          {syncMatrixReasonText(leaf.entry, t)}
        </span>
      )}
    </div>
  );
});

function CheckGlyph({ state }: { state: CheckState }) {
  if (state === 'checked') return <CheckSquare size={16} strokeWidth={2.5} className="text-success shrink-0" />;
  if (state === 'partial') return <Minus size={16} strokeWidth={3} className="text-warning shrink-0" />;
  return <Square size={16} strokeWidth={2.2} className="text-pencil-light shrink-0" />;
}

function StatBox({ label, value }: { label: string; value: string }) {
  return (
    <div
      className="p-3 bg-muted/10 border border-muted"
      style={{ borderRadius: radius.md }}
    >
      <div className="text-2xl font-bold text-pencil leading-none">{value}</div>
      <div className="text-xs text-pencil-light mt-1">{label}</div>
    </div>
  );
}

function PatternList({
  patterns,
  emptyLabel,
  danger = false,
}: {
  patterns: string[];
  emptyLabel: string;
  danger?: boolean;
}) {
  if (patterns.length === 0) {
    return (
      <div
        className="px-3 py-2 text-sm text-pencil-light bg-muted/10 border border-muted"
        style={{ borderRadius: radius.sm }}
      >
        {emptyLabel}
      </div>
    );
  }

  return (
    <div className="flex flex-wrap gap-1.5">
      {patterns.map((pattern) => (
        <span
          key={pattern}
          className={`inline-flex max-w-full items-center gap-1 text-xs font-bold px-2 py-1 border font-mono ${
            danger
              ? 'bg-danger-light text-danger border-danger'
              : 'bg-info-light text-blue border-blue'
          }`}
          style={{ borderRadius: radius.sm }}
        >
          <span className="truncate">{pattern}</span>
        </span>
      ))}
    </div>
  );
}

function StatusIcon({ status }: { status: SyncMatrixEntry['status'] }) {
  switch (status) {
    case 'synced':
      return <Check size={14} strokeWidth={3} className="text-success shrink-0" />;
    case 'excluded':
      return <X size={14} strokeWidth={3} className="text-danger shrink-0" />;
    case 'not_included':
      return <X size={14} strokeWidth={3} className="text-warning shrink-0" />;
    case 'skill_target_mismatch':
      return <Info size={14} strokeWidth={2.5} className="text-pencil-light shrink-0" />;
    default:
      return <span className="w-3.5 shrink-0" />;
  }
}
