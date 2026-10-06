import {
  FilePlus2,
  CheckCircle2,
  XCircle,
  Ban,
  DatabaseZap,
  Link2,
  ShieldCheck,
} from 'lucide-react';
import type { ConsentEventType } from '../../types';

const eventMeta: Record<ConsentEventType, { label: string; icon: typeof FilePlus2; tone: string }> = {
  consent_requested: {
    label: 'Consent requested',
    icon: FilePlus2,
    tone: 'text-blue-600 bg-blue-50 border-blue-100',
  },
  consent_approved: {
    label: 'Consent approved',
    icon: CheckCircle2,
    tone: 'text-primary-600 bg-primary-50 border-primary-100',
  },
  consent_rejected: {
    label: 'Consent rejected',
    icon: XCircle,
    tone: 'text-red-600 bg-red-50 border-red-100',
  },
  consent_revoked: {
    label: 'Consent revoked',
    icon: Ban,
    tone: 'text-slate-500 bg-slate-50 border-slate-100',
  },
  data_accessed: {
    label: 'Data accessed',
    icon: DatabaseZap,
    tone: 'text-amber-600 bg-amber-50 border-amber-100',
  },
};

/** Icon + label for an event type; icon carries the semantics, not just color. */
export function EventLabel({ eventType }: { eventType: ConsentEventType }) {
  const meta = eventMeta[eventType] ?? { label: eventType, icon: Link2, tone: 'text-ink-muted bg-canvas border-line' };
  const Icon = meta.icon;
  return (
    <span className="inline-flex items-center gap-2">
      <span className={`flex h-6 w-6 items-center justify-center rounded border ${meta.tone}`}>
        <Icon size={13} strokeWidth={2.2} />
      </span>
      <span className="text-[13px] font-medium text-ink">{meta.label}</span>
    </span>
  );
}

/** Tiny genesis/chain glyph used in the ledger rail. */
export function ChainNode({ valid, isTail }: { valid: boolean; isTail: boolean }) {
  return (
    <span className="flex flex-col items-center">
      <span
        className={`h-2.5 w-2.5 rounded-sm border-2 ${
          valid ? 'border-primary-500 bg-primary-100' : 'border-red-500 bg-red-100'
        }`}
      />
      {!isTail && (
        <span className={`mt-0.5 h-full w-px ${valid ? 'bg-lineStrong' : 'bg-red-400'}`} />
      )}
    </span>
  );
}

export { ShieldCheck };
