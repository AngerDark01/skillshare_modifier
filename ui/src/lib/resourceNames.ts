export function formatAgentDisplayName(flatName: string): string {
  return flatName.replace(/__/g, '/').replace(/\.md$/i, '');
}

export function formatSkillDisplayName(flatName: string): string {
  return flatName.replace(/__/g, '/');
}

export function formatTrackedRepoName(name: string): string {
  return name.replace(/^_/, '').replace(/__/g, '/');
}

export function formatPreviewResourceName(name: string, kind: 'skill' | 'agent'): string {
  return kind === 'agent' ? formatAgentDisplayName(name) : formatSkillDisplayName(name);
}
