import Badge from './Badge';

type SourceType = 'tracked' | 'github' | 'local';

interface SourceBadgeProps {
  type?: string;
  isInRepo?: boolean;
  size?: 'sm' | 'md';
}

function resolveSource(type?: string, isInRepo?: boolean): SourceType {
  if (isInRepo) return 'tracked';
  if (type === 'github' || type === 'github-subdir') return 'github';
  return 'local';
}

const config: Record<SourceType, { label: string; variant: 'default' | 'info' }> = {
  tracked: { label: 'Tracked', variant: 'default' },
  github: { label: 'GitHub', variant: 'info' },
  local: { label: 'Local', variant: 'default' },
};

export default function SourceBadge({ type, isInRepo, size = 'sm' }: SourceBadgeProps) {
  const source = resolveSource(type, isInRepo);
  const { label, variant } = config[source];
  return <Badge variant={variant} size={size}>{label}</Badge>;
}
