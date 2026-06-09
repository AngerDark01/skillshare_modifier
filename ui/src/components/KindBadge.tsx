interface KindBadgeProps {
  kind: 'skill' | 'agent';
}

const styles = {
  agent: 'text-blue bg-info-light',
  skill: 'text-pencil-light bg-muted',
};

export default function KindBadge({ kind }: KindBadgeProps) {
  return (
    <span
      className={`ss-badge text-[10px] font-bold uppercase tracking-wider px-1.5 py-0 rounded-[var(--radius-sm)] shrink-0 ${styles[kind]}`}
    >
      {kind}
    </span>
  );
}
