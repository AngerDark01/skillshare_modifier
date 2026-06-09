import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  CheckCircle2,
  Database,
  ExternalLink,
  FolderInput,
  GitBranch,
  Library,
  Plus,
  RefreshCw,
  Search,
  Trash2,
} from 'lucide-react';
import { useT } from '../i18n';
import { api, type DiscoveredSkill, type DiscoverResult, type ManagedSource } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import { formatSkillDisplayName } from '../lib/resourceNames';
import { useToast } from '../components/Toast';
import PageHeader from '../components/PageHeader';
import Card from '../components/Card';
import Button from '../components/Button';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import Spinner from '../components/Spinner';
import ConfirmDialog from '../components/ConfirmDialog';
import { Checkbox, Input, Select, type SelectOption } from '../components/Input';

const TEST_SOURCE = {
  label: 'Matt Pocock Skills',
  source: 'mattpocock/skills',
};

function normalizeFolderInput(input: string): string {
  return input.trim().replace(/\\/g, '/').replace(/^\/+|\/+$/g, '').replace(/\/+/g, '/');
}

function sourceHref(source: string): string | null {
  const trimmed = source.trim();
  if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) return trimmed;
  if (trimmed.startsWith('github.com/')) return `https://${trimmed}`;
  if (/^[^/\s]+\/[^/\s]+(?:\/.*)?$/.test(trimmed) && !trimmed.startsWith('./') && !trimmed.startsWith('../')) {
    return `https://github.com/${trimmed}`;
  }
  return null;
}

function folderLabel(folder: string, rootLabel: string): string {
  return folder || rootLabel;
}

function discoveredKey(skill: DiscoveredSkill): string {
  return skill.path || skill.name;
}

function selectedSkillsFrom(discovery: DiscoverResult | null, selected: Set<string>): DiscoveredSkill[] {
  if (!discovery) return [];
  return discovery.skills.filter((skill) => selected.has(discoveredKey(skill)));
}

export default function SourcesPage() {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [selectedSourceId, setSelectedSourceId] = useState<string | null>(null);
  const [formLabel, setFormLabel] = useState(TEST_SOURCE.label);
  const [formSource, setFormSource] = useState(TEST_SOURCE.source);
  const [formBranch, setFormBranch] = useState('');
  const [sourceToDelete, setSourceToDelete] = useState<ManagedSource | null>(null);

  const [discovery, setDiscovery] = useState<DiscoverResult | null>(null);
  const [scanningId, setScanningId] = useState<string | null>(null);
  const [skillFilter, setSkillFilter] = useState('');
  const [selectedSkills, setSelectedSkills] = useState<Set<string>>(new Set());
  const [selectedFolder, setSelectedFolder] = useState('');
  const [customFolder, setCustomFolder] = useState('');
  const [forceInstall, setForceInstall] = useState(false);

  const sourcesQuery = useQuery({
    queryKey: queryKeys.sources.all,
    queryFn: api.listSources,
    staleTime: staleTimes.sources,
  });

  const foldersQuery = useQuery({
    queryKey: queryKeys.skills.folders,
    queryFn: api.listResourceFolders,
    staleTime: staleTimes.skills,
  });

  const sources = useMemo(() => sourcesQuery.data?.sources ?? [], [sourcesQuery.data?.sources]);
  const activeSource = useMemo(
    () => sources.find((source) => source.id === selectedSourceId) ?? sources[0] ?? null,
    [selectedSourceId, sources],
  );

  const selectSource = (id: string) => {
    setSelectedSourceId(id);
    setDiscovery(null);
    setSelectedSkills(new Set());
    setSkillFilter('');
  };

  const folderOptions = useMemo<SelectOption[]>(() => {
    const rootLabel = t('sources.folder.root');
    return [
      { value: '', label: rootLabel },
      ...(foldersQuery.data?.folders ?? []).map((folder) => ({ value: folder, label: folder })),
    ];
  }, [foldersQuery.data?.folders, t]);

  const destinationFolder = normalizeFolderInput(customFolder || selectedFolder);
  const discoveredSkills = useMemo(() => discovery?.skills ?? [], [discovery]);
  const filteredSkills = useMemo(() => {
    const needle = skillFilter.trim().toLowerCase();
    if (!needle) return discoveredSkills;
    return discoveredSkills.filter((skill) =>
      skill.name.toLowerCase().includes(needle) ||
      skill.path.toLowerCase().includes(needle) ||
      (skill.description ?? '').toLowerCase().includes(needle),
    );
  }, [discoveredSkills, skillFilter]);
  const selectedForInstall = selectedSkillsFrom(discovery, selectedSkills);
  const allFilteredSelected = filteredSkills.length > 0 &&
    filteredSkills.every((skill) => selectedSkills.has(discoveredKey(skill)));

  const addSourceMutation = useMutation({
    mutationFn: api.addSource,
    onSuccess: (data) => {
      toast(t('sources.toast.added', { label: data.source.label }), 'success');
      selectSource(data.source.id);
      queryClient.invalidateQueries({ queryKey: queryKeys.sources.all });
    },
    onError: (error: Error) => toast(error.message, 'error'),
  });

  const removeSourceMutation = useMutation({
    mutationFn: api.removeSource,
    onSuccess: () => {
      toast(t('sources.toast.removed'), 'success');
      setSourceToDelete(null);
      setDiscovery(null);
      queryClient.invalidateQueries({ queryKey: queryKeys.sources.all });
    },
    onError: (error: Error) => toast(error.message, 'error'),
  });

  const promoteMutation = useMutation({
    mutationFn: (skills: DiscoveredSkill[]) => {
      if (!activeSource) throw new Error(t('sources.toast.selectSource'));
      return api.installBatch({
        source: activeSource.source,
        branch: activeSource.branch || undefined,
        skills,
        into: destinationFolder,
        force: forceInstall,
      });
    },
    onSuccess: (result) => {
      const errors: string[] = [];
      let hasAuditBlock = false;
      for (const item of result.results) {
        if (item.error) {
          if (item.error.includes('security audit failed')) {
            hasAuditBlock = true;
          }
          errors.push(`${formatSkillDisplayName(item.name)}: ${item.error}`);
        }
        item.warnings?.forEach((warning) => toast(`${formatSkillDisplayName(item.name)}: ${warning}`, 'warning'));
      }
      if (errors.length > 0) {
        toast(t('common.nFailed', { count: errors.length, details: errors.join('; ') }), 'error');
      }
      toast(result.summary, hasAuditBlock ? 'warning' : 'success');
      setSelectedSkills(new Set());
      clearAuditCache(queryClient);
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.folders });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    },
    onError: (error: Error) => toast(error.message, 'error'),
  });

  const handleAddSource = () => {
    const source = formSource.trim();
    if (!source) {
      toast(t('sources.toast.sourceRequired'), 'error');
      return;
    }
    addSourceMutation.mutate({
      label: formLabel.trim(),
      source,
      branch: formBranch.trim() || undefined,
      kind: 'skill',
    });
  };

  const handleScan = async (source: ManagedSource | null = activeSource) => {
    if (!source) {
      toast(t('sources.toast.selectSource'), 'error');
      return;
    }
    setScanningId(source.id);
    setDiscovery(null);
    setSelectedSkills(new Set());
    try {
      const result = await api.discover(source.source, source.branch || undefined);
      setDiscovery(result);
      toast(t('sources.toast.discovered', { count: result.skills.length }), result.skills.length > 0 ? 'success' : 'info');
    } catch (error) {
      toast((error as Error).message, 'error');
    } finally {
      setScanningId(null);
    }
  };

  const toggleSkill = (skill: DiscoveredSkill) => {
    const key = discoveredKey(skill);
    setSelectedSkills((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const toggleFiltered = () => {
    setSelectedSkills((prev) => {
      const next = new Set(prev);
      if (allFilteredSelected) {
        filteredSkills.forEach((skill) => next.delete(discoveredKey(skill)));
      } else {
        filteredSkills.forEach((skill) => next.add(discoveredKey(skill)));
      }
      return next;
    });
  };

  const handlePromote = () => {
    if (!activeSource) {
      toast(t('sources.toast.selectSource'), 'error');
      return;
    }
    if (selectedForInstall.length === 0) {
      toast(t('sources.toast.selectSkills'), 'error');
      return;
    }
    promoteMutation.mutate(selectedForInstall);
  };

  const activeHref = activeSource ? sourceHref(activeSource.source) : null;

  return (
    <div className="space-y-4 animate-fade-in">
      <PageHeader
        icon={<Database size={24} strokeWidth={2.5} />}
        title={t('sources.title')}
        subtitle={t('sources.subtitle')}
        actions={
          <Button
            variant="secondary"
            size="sm"
            onClick={() => sourcesQuery.refetch()}
            loading={sourcesQuery.isFetching}
          >
            {!sourcesQuery.isFetching && <RefreshCw size={14} strokeWidth={2.5} />}
            {t('sources.actions.refresh')}
          </Button>
        }
      />

      <div className="grid gap-4 lg:grid-cols-[360px_minmax(0,1fr)]">
        <div className="space-y-4">
          <Card>
            <div className="flex items-center gap-2 mb-4">
              <Plus size={18} strokeWidth={2.5} className="text-blue" />
              <h3 className="font-bold text-pencil">{t('sources.add.title')}</h3>
            </div>
            <div className="space-y-3">
              <Input
                label={t('sources.add.label')}
                value={formLabel}
                onChange={(event) => setFormLabel(event.target.value)}
                placeholder={TEST_SOURCE.label}
              />
              <Input
                label={t('sources.add.source')}
                value={formSource}
                onChange={(event) => setFormSource(event.target.value)}
                placeholder={TEST_SOURCE.source}
              />
              <Input
                label={t('sources.add.branch')}
                value={formBranch}
                onChange={(event) => setFormBranch(event.target.value)}
                placeholder={t('sources.add.branchPlaceholder')}
              />
              <p className="text-xs text-muted-dark">{t('sources.add.testHint')}</p>
              <Button
                className="w-full"
                onClick={handleAddSource}
                loading={addSourceMutation.isPending}
              >
                {!addSourceMutation.isPending && <Plus size={16} strokeWidth={2.5} />}
                {t('sources.add.save')}
              </Button>
            </div>
          </Card>

          <Card padding="none" overflow>
            <div className="px-4 py-3 border-b border-muted flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Library size={17} strokeWidth={2.5} className="text-blue" />
                <h3 className="font-bold text-pencil">{t('sources.list.title')}</h3>
              </div>
              <Badge variant="info">{sources.length}</Badge>
            </div>

            {sourcesQuery.isLoading ? (
              <div className="py-10 flex justify-center"><Spinner /></div>
            ) : sources.length === 0 ? (
              <div className="px-4">
                <EmptyState
                  icon={Database}
                  title={t('sources.list.empty.title')}
                  description={t('sources.list.empty.description')}
                />
              </div>
            ) : (
              <div className="divide-y divide-muted">
                {sources.map((source) => {
                  const selected = activeSource?.id === source.id;
                  const href = sourceHref(source.source);
                  return (
                    <div
                      key={source.id}
                      className={`group px-4 py-3 transition-colors ${selected ? 'bg-muted/40' : 'hover:bg-muted/20'}`}
                    >
                      <button
                        type="button"
                        className="w-full text-left"
                        onClick={() => selectSource(source.id)}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="font-semibold text-pencil truncate">{source.label}</span>
                              {selected && <CheckCircle2 size={14} strokeWidth={2.5} className="text-success shrink-0" />}
                            </div>
                            <p className="font-mono text-xs text-muted-dark truncate mt-1">{source.source}</p>
                          </div>
                          <Badge>{source.kind ?? 'skill'}</Badge>
                        </div>
                      </button>
                      <div className="mt-2 flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2 text-xs text-muted-dark min-w-0">
                          <GitBranch size={12} strokeWidth={2.5} />
                          <span className="truncate">{source.branch || t('sources.meta.defaultBranch')}</span>
                        </div>
                        <div className="flex items-center gap-1 opacity-70 group-hover:opacity-100 transition-opacity">
                          {href && (
                            <a
                              href={href}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="inline-flex items-center justify-center w-7 h-7 text-pencil-light hover:text-pencil hover:bg-muted/40 rounded-[var(--radius-sm)]"
                              title={t('sources.link.open')}
                            >
                              <ExternalLink size={14} strokeWidth={2.5} />
                            </a>
                          )}
                          <button
                            type="button"
                            className="inline-flex items-center justify-center w-7 h-7 text-danger hover:bg-danger-light rounded-[var(--radius-sm)]"
                            title={t('sources.delete.confirm')}
                            onClick={() => setSourceToDelete(source)}
                          >
                            <Trash2 size={14} strokeWidth={2.5} />
                          </button>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </Card>

          {sourcesQuery.data?.path && (
            <p className="text-xs text-muted-dark px-1">
              {t('sources.storagePath')} <span className="font-mono break-all">{sourcesQuery.data.path}</span>
            </p>
          )}
        </div>

        <div className="space-y-4 min-w-0">
          <Card>
            {activeSource ? (
              <div className="space-y-4">
                <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <h3 className="text-xl font-bold text-pencil truncate">{activeSource.label}</h3>
                      <Badge variant="info">{activeSource.kind ?? 'skill'}</Badge>
                      {activeSource.branch && <Badge variant="default">{activeSource.branch}</Badge>}
                    </div>
                    <p className="font-mono text-xs text-muted-dark mt-1 truncate">{activeSource.source}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    {activeHref && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => window.open(activeHref, '_blank', 'noopener,noreferrer')}
                      >
                        <ExternalLink size={14} strokeWidth={2.5} />
                        {t('sources.link.open')}
                      </Button>
                    )}
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => handleScan(activeSource)}
                      loading={scanningId === activeSource.id}
                    >
                      {scanningId !== activeSource.id && <RefreshCw size={14} strokeWidth={2.5} />}
                      {scanningId === activeSource.id ? t('sources.scan.scanning') : t('sources.scan.button')}
                    </Button>
                  </div>
                </div>

                <div className="grid gap-3 md:grid-cols-3">
                  <div className="border border-muted rounded-[var(--radius-md)] p-3 bg-muted/15">
                    <p className="text-xs text-muted-dark">{t('sources.pipeline.remote')}</p>
                    <p className="text-sm font-semibold text-pencil mt-1">{activeSource.source}</p>
                  </div>
                  <div className="border border-muted rounded-[var(--radius-md)] p-3 bg-muted/15">
                    <p className="text-xs text-muted-dark">{t('sources.pipeline.scan')}</p>
                    <p className="text-sm font-semibold text-pencil mt-1">
                      {discovery ? t('sources.scan.found', { count: discoveredSkills.length }) : t('sources.pipeline.waiting')}
                    </p>
                  </div>
                  <div className="border border-muted rounded-[var(--radius-md)] p-3 bg-muted/15">
                    <p className="text-xs text-muted-dark">{t('sources.pipeline.local')}</p>
                    <p className="text-sm font-semibold text-pencil mt-1">
                      {folderLabel(destinationFolder, t('sources.folder.root'))}
                    </p>
                  </div>
                </div>
              </div>
            ) : (
              <EmptyState
                icon={Database}
                title={t('sources.active.none.title')}
                description={t('sources.active.none.description')}
              />
            )}
          </Card>

          {activeSource && (
            <Card>
              <div className="flex flex-col gap-3">
                <div className="flex flex-col xl:flex-row xl:items-end gap-3">
                  <div className="relative flex-1">
                    <Search
                      size={15}
                      strokeWidth={2.5}
                      className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
                    />
                    <Input
                      value={skillFilter}
                      onChange={(event) => setSkillFilter(event.target.value)}
                      placeholder={t('sources.scan.searchPlaceholder')}
                      className="!pl-9"
                      size="sm"
                    />
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2 xl:w-[24rem]">
                    <Select
                      label={t('sources.folder.existing')}
                      value={selectedFolder}
                      onChange={setSelectedFolder}
                      options={folderOptions}
                      size="sm"
                    />
                    <Input
                      label={t('sources.folder.custom')}
                      value={customFolder}
                      onChange={(event) => setCustomFolder(event.target.value)}
                      placeholder="03-frontend-architecture"
                      size="sm"
                    />
                  </div>
                </div>

                <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3 border-t border-muted pt-3">
                  <div className="flex items-center gap-3 flex-wrap text-sm text-pencil-light">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={toggleFiltered}
                      disabled={filteredSkills.length === 0}
                    >
                      {allFilteredSelected ? t('sources.scan.deselectAll') : t('sources.scan.selectAll')}
                    </Button>
                    <span>{t('sources.scan.selected', { selected: selectedForInstall.length, total: discoveredSkills.length })}</span>
                    <Checkbox
                      label={t('sources.promote.force')}
                      checked={forceInstall}
                      onChange={setForceInstall}
                      size="sm"
                    />
                  </div>
                  <Button
                    onClick={handlePromote}
                    disabled={selectedForInstall.length === 0 || !discovery}
                    loading={promoteMutation.isPending}
                  >
                    {!promoteMutation.isPending && <FolderInput size={16} strokeWidth={2.5} />}
                    {t('sources.promote.button', { count: selectedForInstall.length })}
                  </Button>
                </div>
              </div>
            </Card>
          )}

          {activeSource && !discovery && scanningId !== activeSource.id && (
            <EmptyState
              icon={RefreshCw}
              title={t('sources.scan.empty.title')}
              description={t('sources.scan.empty.description')}
              action={
                <Button variant="secondary" size="sm" onClick={() => handleScan(activeSource)}>
                  <RefreshCw size={14} strokeWidth={2.5} />
                  {t('sources.scan.button')}
                </Button>
              }
            />
          )}

          {activeSource && scanningId === activeSource.id && (
            <div className="py-16 flex flex-col items-center gap-3 text-pencil-light">
              <Spinner size="lg" />
              <p>{t('sources.scan.scanningLong')}</p>
            </div>
          )}

          {discovery && (
            <Card padding="none">
              {filteredSkills.length === 0 ? (
                <EmptyState
                  icon={Search}
                  title={discoveredSkills.length === 0 ? t('sources.scan.noSkills.title') : t('sources.scan.noMatch.title')}
                  description={discoveredSkills.length === 0 ? t('sources.scan.noSkills.description') : t('sources.scan.noMatch.description')}
                />
              ) : (
                <div className="divide-y divide-muted">
                  {filteredSkills.map((skill) => {
                    const key = discoveredKey(skill);
                    const checked = selectedSkills.has(key);
                    return (
                      <div
                        key={key}
                        className={`px-4 py-3 transition-colors ${checked ? 'bg-info-light/70' : 'hover:bg-muted/20'}`}
                      >
                        <div className="flex items-start gap-3">
                          <Checkbox
                            label=""
                            checked={checked}
                            onChange={() => toggleSkill(skill)}
                            size="sm"
                            className="mt-0.5"
                          />
                          <button
                            type="button"
                            className="min-w-0 flex-1 text-left"
                            onClick={() => toggleSkill(skill)}
                          >
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className="font-semibold text-pencil">{formatSkillDisplayName(skill.name)}</span>
                              <Badge variant="default">{skill.kind ?? 'skill'}</Badge>
                            </div>
                            {skill.description && (
                              <p className="text-sm text-pencil-light mt-1 line-clamp-2">{skill.description}</p>
                            )}
                            <p className="font-mono text-xs text-muted-dark mt-2 truncate">{skill.path}</p>
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </Card>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={!!sourceToDelete}
        title={t('sources.delete.title')}
        message={sourceToDelete ? t('sources.delete.message', { label: sourceToDelete.label }) : ''}
        confirmText={t('sources.delete.confirm')}
        variant="danger"
        loading={removeSourceMutation.isPending}
        onCancel={() => setSourceToDelete(null)}
        onConfirm={() => {
          if (sourceToDelete) removeSourceMutation.mutate(sourceToDelete.id);
        }}
      />
    </div>
  );
}
