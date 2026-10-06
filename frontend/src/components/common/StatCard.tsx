import { ReactNode } from 'react';
import { cn } from '../../utils/format';

interface StatCardProps {
  label: string;
  value: string | number;
  icon: ReactNode;
  /** Optional secondary line under the value. */
  hint?: string;
  tone?: 'primary' | 'amber' | 'red' | 'slate' | 'blue';
  className?: string;
}

const toneClasses = {
  primary: 'text-primary-600',
  amber: 'text-amber-600',
  red: 'text-red-600',
  slate: 'text-ink-muted',
  blue: 'text-blue-600',
};

/** Stat block — Mercury-style: a borderless column where the mono numeral is
 *  the hero and the label sits quietly beneath. No card chrome, no shadow;
 *  structure comes from the hairline dividers between blocks. */
export function StatCard({ label, value, icon, hint, tone = 'slate', className }: StatCardProps) {
  return (
    <div className={cn('min-w-0', className)}>
      <div className="flex items-baseline justify-between gap-3">
        <p className="font-mono text-[30px] font-semibold leading-none tracking-tight text-ink">{value}</p>
        <span className={cn('flex h-8 w-8 items-center justify-center rounded-md', toneClasses[tone])}>{icon}</span>
      </div>
      <p className="label mt-2.5">{label}</p>
      {hint && <p className="mt-1 text-[12px] text-ink-faint">{hint}</p>}
    </div>
  );
}
