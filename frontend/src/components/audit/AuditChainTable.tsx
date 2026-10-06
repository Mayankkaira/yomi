import { useState } from 'react';
import { ArrowRight, ChevronDown, Copy, ShieldAlert } from 'lucide-react';
import type { AuditEntry } from '../../types';
import { formatDateTime, shortId, truncateHash } from '../../utils/format';
import { EventLabel } from './EventLabel';
import { cn } from '../../utils/format';
import { useToast } from '../../context/ToastContext';

interface AuditChainTableProps {
  /** Entries ordered oldest → newest so the chain reads top to bottom. */
  entries: AuditEntry[];
  /** Map of entryId → integrity flag (from chain verification). */
  validity: Record<string, boolean>;
  /** Optional institution-name resolver for display. */
  institutionNames?: Record<string, string>;
}

/** Copyable mono value with a small copy affordance. */
function Copyable({ label, value }: { label: string; value: string }) {
  const { push } = useToast();
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      push('info', `${label} copied to clipboard.`);
    } catch {
      push('error', 'Clipboard access denied by the browser.');
    }
  };
  return (
    <div className="min-w-0">
      <p className="label">{label}</p>
      <button
        onClick={copy}
        title={value}
        className="group mt-0.5 flex max-w-full items-center gap-1.5 font-mono text-[11.5px] text-ink-secondary transition-colors hover:text-primary-700"
      >
        <span className="truncate">{value}</span>
        <Copy size={11} className="shrink-0 text-ink-faint transition-colors group-hover:text-primary-600" />
      </button>
    </div>
  );
}

/** The ledger rendered as a hash chain: each row links to the previous row's
 *  entry hash, with a visual rail connecting consecutive nodes. Clicking a row
 *  reveals the full cryptographic detail for that entry. */
export function AuditChainTable({ entries, validity, institutionNames }: AuditChainTableProps) {
  const [expandedId, setExpandedId] = useState<string | null>(null);

  if (entries.length === 0) return null;

  return (
    <div className="panel overflow-x-auto">
      <table className="data-table min-w-[880px]">
        <thead>
          <tr>
            <th className="w-10"></th>
            <th className="num">Seq</th>
            <th>Event</th>
            <th>Institution</th>
            <th>Prev hash → Entry hash</th>
            <th>Recorded at</th>
            <th className="text-right">Integrity</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry, idx) => {
            const isTail = idx === entries.length - 1;
            const valid = validity[entry.id] ?? true;
            const expanded = expandedId === entry.id;
            return (
              <FragmentRow
                key={entry.id}
                entry={entry}
                isTail={isTail}
                valid={valid}
                expanded={expanded}
                institutionName={
                  entry.institutionId
                    ? (institutionNames?.[entry.institutionId] ?? shortId(entry.institutionId))
                    : null
                }
                onToggle={() => setExpandedId(expanded ? null : entry.id)}
              />
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function FragmentRow({
  entry,
  isTail,
  valid,
  expanded,
  institutionName,
  onToggle,
}: {
  entry: AuditEntry;
  isTail: boolean;
  valid: boolean;
  expanded: boolean;
  institutionName: string | null;
  onToggle: () => void;
}) {
  return (
    <>
      <tr className={cn(valid ? '' : 'bg-red-50/50', 'cursor-pointer')} onClick={onToggle}>
        <td className="p-0">
          <div className="flex h-full items-stretch justify-center py-3">
            <div className="flex flex-col items-center">
              <span
                className={cn(
                  'mt-1 h-2.5 w-2.5 shrink-0 rounded-sm border-2',
                  valid ? 'border-primary-500 bg-primary-100' : 'border-red-500 bg-red-100',
                )}
              />
              {!isTail && <span className={cn('w-px flex-1', valid ? 'bg-lineStrong' : 'bg-red-400')} />}
            </div>
          </div>
        </td>
        <td className="num data-value">#{entry.sequenceNo}</td>
        <td>
          <EventLabel eventType={entry.eventType} />
        </td>
        <td className="data-value">{institutionName ?? '—'}</td>
        <td>
          <div className="flex items-center gap-1.5 font-mono text-[11.5px] text-ink-secondary">
            <span title={entry.prevHash}>{truncateHash(entry.prevHash, 8, 6)}</span>
            <ArrowRight size={11} className="text-ink-faint" />
            <span title={entry.entryHash} className="text-primary-700">
              {truncateHash(entry.entryHash, 8, 6)}
            </span>
          </div>
        </td>
        <td className="data-value whitespace-nowrap">{formatDateTime(entry.createdAt)}</td>
        <td className="text-right">
          <div className="flex items-center justify-end gap-2">
            {valid ? (
              <span className="inline-flex items-center gap-1 text-[11.5px] font-medium text-primary-600">verified</span>
            ) : (
              <span className="inline-flex items-center gap-1 text-[11.5px] font-semibold text-red-600">
                <ShieldAlert size={12} /> tampered
              </span>
            )}
            <ChevronDown
              size={14}
              className={cn('shrink-0 text-ink-faint transition-transform duration-150', expanded && 'rotate-180')}
            />
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="bg-tint">
          <td colSpan={7} className="px-5 py-4">
            <div className="rounded-md border border-line bg-surface p-4">
              <div className="flex items-center justify-between gap-3">
                <p className="text-[12.5px] font-semibold uppercase tracking-label text-ink-muted">
                  Entry {entry.sequenceNo} — cryptographic detail
                </p>
                <span className="font-mono text-[11px] text-ink-faint">{entry.id}</span>
              </div>
              <div className="mt-4 grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
                <Copyable label="Entry hash" value={entry.entryHash} />
                <Copyable label="Previous hash" value={entry.prevHash} />
                <Copyable label="Event ID" value={entry.id} />
                {entry.citizenId && <Copyable label="Citizen ID" value={entry.citizenId} />}
                {entry.institutionId && <Copyable label="Institution ID" value={entry.institutionId} />}
                {entry.requestId && <Copyable label="Request ID" value={entry.requestId} />}
              </div>
              {entry.metadata && (
                <div className="mt-4">
                  <p className="label">Metadata</p>
                  <pre className="mt-1.5 overflow-x-auto rounded-md border border-line bg-canvas px-3 py-2.5 font-mono text-[11.5px] leading-relaxed text-ink-secondary">
                    {JSON.stringify(entry.metadata, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  );
}
