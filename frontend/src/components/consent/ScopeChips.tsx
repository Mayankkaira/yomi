import { ReactNode } from 'react';

interface ScopeChipsProps {
  scopes: string[];
}

/** Data-scope tags rendered as small mono chips. */
export function ScopeChips({ scopes }: ScopeChipsProps) {
  if (!scopes || scopes.length === 0) {
    return <span className="text-[12.5px] text-ink-faint">—</span>;
  }
  return (
    <div className="flex flex-wrap gap-1.5">
      {scopes.map((s) => (
        <span
          key={s}
          className="inline-flex items-center rounded border border-line bg-canvas px-1.5 py-0.5 font-mono text-[11px] font-medium text-ink-secondary"
        >
          {s}
        </span>
      ))}
    </div>
  );
}

interface KeyValueProps {
  label: string;
  children: ReactNode;
  mono?: boolean;
}

/** Label + value pair used across cards and detail rows. */
export function KeyValue({ label, children, mono = false }: KeyValueProps) {
  return (
    <div className="min-w-0">
      <p className="label">{label}</p>
      <p className={`mt-0.5 truncate text-[13px] ${mono ? 'data-value' : 'text-ink'}`}>{children}</p>
    </div>
  );
}
