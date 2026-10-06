import { useEffect, useState } from 'react';
import { Check, X, RotateCcw, Clock3 } from 'lucide-react';
import type { ConsentRequest, Institution } from '../../types';
import { formatDateTime, cn } from '../../utils/format';
import { Button } from '../common/Button';
import { SectorBadge } from '../common/Badge';
import { Modal } from '../common/Modal';
import { KeyValue, ScopeChips } from './ScopeChips';

interface ConsentRequestCardProps {
  request: ConsentRequest;
  institution?: Institution;
  onApprove: (requestId: string) => Promise<void>;
  onReject: (requestId: string) => Promise<void>;
  busy: boolean;
}

/** A pending consent request card with full context and decision actions. */
/** Institution monogram tile: initials on a neutral hairline square, matching
 *  the quiet identity system — color is reserved for status semantics. */
function InstitutionMark({ name }: { name: string }) {
  const initials = name
    .split(' ')
    .filter(Boolean)
    .map((p) => p[0])
    .slice(0, 2)
    .join('')
    .toUpperCase();
  return (
    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-line bg-canvas text-[13px] font-bold text-ink">
      {initials}
    </div>
  );
}

/** Live expiry countdown: refreshes every 30s so the urgency stays current. */
function ExpiryChip({ expiresAt }: { expiresAt: string }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(id);
  }, []);

  const remaining = new Date(expiresAt).getTime() - now;
  if (remaining <= 0) {
    return (
      <span className="inline-flex items-center gap-1.5 rounded border border-red-100 bg-red-50 px-2 py-0.5 text-[11.5px] font-semibold text-red-700">
        <Clock3 size={12} strokeWidth={2.2} /> Expired
      </span>
    );
  }
  const days = Math.floor(remaining / 86_400_000);
  const hours = Math.floor((remaining % 86_400_000) / 3_600_000);
  const label = days > 0 ? `${days}d ${hours}h left` : `${Math.max(1, Math.ceil(remaining / 3_600_000))}h left`;
  const urgent = days === 0;
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded border px-2 py-0.5 text-[11.5px] font-semibold',
        urgent ? 'border-amber-100 bg-amber-50 text-amber-700' : 'border-line bg-canvas text-ink-muted',
      )}
    >
      <Clock3 size={12} strokeWidth={2.2} /> {label}
    </span>
  );
}

export function ConsentRequestCard({ request, institution, onApprove, onReject, busy }: ConsentRequestCardProps) {
  const [confirming, setConfirming] = useState<'approve' | 'reject' | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const runAction = async (action: 'approve' | 'reject') => {
    setActionError(null);
    try {
      if (action === 'approve') await onApprove(request.id);
      else await onReject(request.id);
      setConfirming(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Action failed');
    }
  };

  const institutionName = institution?.name ?? `Institution ${request.institutionId.slice(0, 8)}`;

  return (
    <article className="px-5 py-5 transition-colors duration-150 hover:bg-tint">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3.5">
          <InstitutionMark name={institutionName} />
          <div className="min-w-0">
            <h3 className="text-[15px] font-semibold text-ink">{institutionName}</h3>
            <div className="mt-1 flex flex-wrap items-center gap-2">
              {institution && <SectorBadge sector={institution.sector} />}
              <span className="font-mono text-[11px] text-ink-faint">{request.id.slice(0, 8)}</span>
            </div>
          </div>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1.5">
          <span className="font-mono text-[11.5px] text-ink-muted">{formatDateTime(request.createdAt)}</span>
          <ExpiryChip expiresAt={request.expiresAt} />
        </div>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-x-8 gap-y-4 sm:grid-cols-2 lg:grid-cols-3">
        <div className="sm:col-span-2">
          <p className="label mb-1.5">Purpose</p>
          <p className="text-[13.5px] leading-relaxed text-ink">{request.purpose}</p>
        </div>
        <div>
          <p className="label mb-1.5">Data requested</p>
          <ScopeChips scopes={request.dataScope} />
        </div>
        <div>
          <p className="label mb-1.5">Access window</p>
          <p className="data-value">until {formatDateTime(request.expiresAt)}</p>
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between gap-3">
        <p className="text-[12px] text-ink-muted">
          Approving grants a signed, revocable access token recorded on the audit ledger.
        </p>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="secondary" size="sm" icon={<X size={14} />} disabled={busy} onClick={() => setConfirming('reject')}>
            Reject
          </Button>
          <Button variant="primary" size="sm" icon={<Check size={14} />} disabled={busy} onClick={() => setConfirming('approve')}>
            Approve
          </Button>
        </div>
      </div>

      <Modal
        open={confirming !== null}
        onClose={() => setConfirming(null)}
        title={confirming === 'approve' ? 'Approve this data request?' : 'Reject this data request?'}
        description={
          confirming === 'approve'
            ? `${institution?.name ?? 'This institution'} will be able to access the requested data until ${formatDateTime(request.expiresAt)}. You can revoke access at any time.`
            : `${institution?.name ?? 'This institution'} will not receive access to the requested data.`
        }
        footer={
          <div className="flex items-center justify-end gap-2">
            {actionError && <p className="mr-auto text-[12.5px] text-red-600">{actionError}</p>}
            <Button variant="secondary" size="sm" onClick={() => setConfirming(null)}>
              Cancel
            </Button>
            <Button
              variant={confirming === 'approve' ? 'primary' : 'danger'}
              size="sm"
              loading={busy}
              onClick={() => confirming && runAction(confirming)}
            >
              {confirming === 'approve' ? 'Approve access' : 'Reject request'}
            </Button>
          </div>
        }
      >
        <div className="space-y-3">
          <KeyValue label="Purpose" mono={false}>
            {request.purpose}
          </KeyValue>
          <KeyValue label="Data scope" mono>
            {request.dataScope.join(', ')}
          </KeyValue>
        </div>
      </Modal>
    </article>
  );
}

/** Inline revoke control for active consents (used on profile). */
export function RevokeButton({ onRevoke, busy }: { onRevoke: () => Promise<void>; busy: boolean }) {
  return (
    <Button variant="secondary" size="sm" icon={<RotateCcw size={13} />} disabled={busy} onClick={onRevoke}>
      Revoke
    </Button>
  );
}
