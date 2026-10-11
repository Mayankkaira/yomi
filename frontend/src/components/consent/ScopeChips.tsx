import { ReactNode } from 'react';

interface ScopeChipsProps {
  scopes: string[];
  labels?: Record<string, string>;
}

export function ScopeChips({ scopes, labels }: ScopeChipsProps) {
  if (!scopes || scopes.length === 0) {
    return <span className="text-[12.5px] text-ink-faint">—</span>;
  }
  return (
    <div className="flex flex-wrap gap-1.5">
      {scopes.map((s) => {
        const label = labels?.[s];
        return (
          <span
            key={s}
            title={s}
            className={
              label
                ? 'inline-flex items-center rounded border border-line bg-canvas px-1.5 py-0.5 text-[11px] font-medium text-ink-secondary'
                : 'inline-flex items-center rounded border border-line bg-canvas px-1.5 py-0.5 font-mono text-[11px] font-medium text-ink-secondary'
            }
          >
            {label ?? s}
          </span>
        );
      })}
    </div>
  );
}

interface KeyValueProps {
  label: string;
  children: ReactNode;
  mono?: boolean;
}

export function KeyValue({ label, children, mono = false }: KeyValueProps) {
  return (
    <div className="min-w-0">
      <p className="label">{label}</p>
      <p className={`mt-0.5 truncate text-[13px] ${mono ? 'data-value' : 'text-ink'}`}>{children}</p>
    </div>
  );
}