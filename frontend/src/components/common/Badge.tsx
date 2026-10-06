import { ReactNode } from 'react';
import {
  CheckCircle2,
  Clock3,
  XCircle,
  Ban,
  CalendarOff,
  Building2,
  ShieldCheck,
  Info,
} from 'lucide-react';
import { cn, humanize } from '../../utils/format';

export type BadgeTone = 'primary' | 'amber' | 'red' | 'slate' | 'blue' | 'neutral';

interface BadgeProps {
  tone: BadgeTone;
  children: ReactNode;
  icon?: ReactNode;
  className?: string;
}

const toneClasses: Record<BadgeTone, string> = {
  primary: 'bg-primary-50 text-primary-700 border-primary-200',
  amber: 'bg-amber-50 text-amber-700 border-amber-100',
  red: 'bg-red-50 text-red-700 border-red-100',
  slate: 'bg-slate-50 text-slate-600 border-slate-100',
  blue: 'bg-blue-50 text-blue-600 border-blue-100',
  neutral: 'bg-canvas text-ink-secondary border-line',
};

/** Badge pairs color with an icon so status is never communicated by color alone. */
export function Badge({ tone, children, icon, className }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded border px-2 py-0.5 text-[11.5px] font-semibold leading-5 whitespace-nowrap',
        toneClasses[tone],
        className,
      )}
    >
      {icon}
      {children}
    </span>
  );
}

/** Map a consent status to a Badge with icon + label. */
export function StatusBadge({ status }: { status: string }) {
  switch (status) {
    case 'approved':
      return (
        <Badge tone="primary" icon={<CheckCircle2 size={13} strokeWidth={2.2} />}>
          Approved
        </Badge>
      );
    case 'pending':
      return (
        <Badge tone="amber" icon={<Clock3 size={13} strokeWidth={2.2} />}>
          Pending
        </Badge>
      );
    case 'rejected':
      return (
        <Badge tone="red" icon={<XCircle size={13} strokeWidth={2.2} />}>
          Rejected
        </Badge>
      );
    case 'revoked':
      return (
        <Badge tone="slate" icon={<Ban size={13} strokeWidth={2.2} />}>
          Revoked
        </Badge>
      );
    case 'expired':
      return (
        <Badge tone="slate" icon={<CalendarOff size={13} strokeWidth={2.2} />}>
          Expired
        </Badge>
      );
    default:
      return <Badge tone="neutral">{humanize(status)}</Badge>;
  }
}

/** Sector badge for institutions — neutral so color stays reserved for
 *  consent status semantics. */
export function SectorBadge({ sector }: { sector: string }) {
  return (
    <Badge tone="neutral" icon={<Building2 size={13} strokeWidth={2.2} />}>
      {sector}
    </Badge>
  );
}

/** Chain integrity badge for the ledger. */
export function IntegrityBadge({ intact, checked }: { intact: boolean; checked: number }) {
  if (checked === 0) {
    return (
      <Badge tone="neutral" icon={<Info size={13} />}>
        No entries yet
      </Badge>
    );
  }
  return intact ? (
    <Badge tone="primary" icon={<ShieldCheck size={13} strokeWidth={2.2} />}>
      Chain intact · {checked} entries verified
    </Badge>
  ) : (
    <Badge tone="red" icon={<XCircle size={13} strokeWidth={2.2} />}>
      Integrity breach detected
    </Badge>
  );
}
