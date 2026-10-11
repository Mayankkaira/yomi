import { useEffect, useMemo, useState } from 'react';
import { ShieldCheck, ShieldAlert, Download, FileCheck2, Lock } from 'lucide-react';
import { auditApi } from '../../api/audit';
import { errorMessage } from '../../api/client';
import { useToast } from '../../context/ToastContext';
import { Button } from '../common/Button';
import { Spinner } from '../common/Spinner';
import type { AuditEntry, Receipt, ReceiptVerification } from '../../types';
import { formatDateTime } from '../../utils/format';

interface ReceiptPanelProps {
  entry: AuditEntry;
  institutionName?: string;
}

type Stage = 'idle' | 'fetching' | 'verified' | 'failed';

export function ReceiptPanel({ entry, institutionName }: ReceiptPanelProps) {
  const { push } = useToast();
  const [stage, setStage] = useState<Stage>('idle');
  const [receipt, setReceipt] = useState<Receipt | null>(null);
  const [result, setResult] = useState<ReceiptVerification | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setStage('idle');
    setReceipt(null);
    setResult(null);
    setError(null);
  }, [entry.id]);

  const fileName = useMemo(() => `yomi-receipt-${entry.sequenceNo}.json`, [entry.sequenceNo]);

  const check = async () => {
    setStage('fetching');
    setError(null);
    try {
      const fetched = await auditApi.receiptFor(entry.id);
      setReceipt(fetched);
      const verified = await auditApi.verifyReceipt(fetched);
      setResult(verified);
      setStage(verified.valid ? 'verified' : 'failed');
    } catch (err) {
      const { message } = errorMessage(err);
      setError(message);
      setStage('idle');
      push('error', message);
    }
  };

  const download = () => {
    if (!receipt) return;
    const blob = new Blob([JSON.stringify(receipt, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = fileName;
    anchor.click();
    URL.revokeObjectURL(url);
    push('success', `${fileName} downloaded — keep it, it is your proof.`);
  };

  return (
    <div className="mt-4 rounded-md border border-line bg-surface p-4">
      <div className="flex items-start gap-3">
        <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-line bg-canvas text-ink-muted">
          <Lock size={15} />
        </span>
        <div className="min-w-0">
          <p className="text-[13px] font-semibold text-ink">Proof this record is genuine</p>
          <p className="mt-0.5 text-[12.5px] leading-relaxed text-ink-muted">
            A sealed copy of this entry, signed by the ledger. Anyone can re-check it later — even after
            {' '}
            {institutionName ?? 'the institution'}
            {' '}
            revokes its access.
          </p>
        </div>
      </div>

      {stage === 'idle' && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <Button variant="primary" size="sm" icon={<FileCheck2 size={14} />} onClick={() => void check()}>
            Check this record
          </Button>
          {error && <p className="text-[12.5px] text-red-600">{error}</p>}
        </div>
      )}

      {stage === 'fetching' && (
        <div className="mt-3">
          <Spinner label="Signing a fresh copy and checking it…" />
        </div>
      )}

      {stage === 'verified' && receipt && result?.valid && (
        <div className="mt-3 rounded-md border border-primary-100 bg-primary-50 p-3">
          <p className="flex items-center gap-2 text-[13px] font-semibold text-primary-700">
            <ShieldCheck size={15} /> This record is genuine
          </p>
          <dl className="mt-2 grid grid-cols-1 gap-x-6 gap-y-1.5 sm:grid-cols-2">
            <div className="flex justify-between gap-3">
              <dt className="text-[12px] text-ink-muted">Recorded</dt>
              <dd className="data-value">{formatDateTime(receipt.recordedAt)}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[12px] text-ink-muted">Ledger position</dt>
              <dd className="data-value">#{receipt.sequenceNo}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[12px] text-ink-muted">Sealed copy</dt>
              <dd className="data-value">{receipt.receiptId.slice(0, 8)}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[12px] text-ink-muted">Still in the ledger</dt>
              <dd className="data-value">{result.chainAnchored ? 'yes' : 'no'}</dd>
            </div>
          </dl>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button variant="secondary" size="sm" icon={<Download size={14} />} onClick={download}>
              Save your copy
            </Button>
            <Button variant="secondary" size="sm" onClick={() => void check()}>
              Check again
            </Button>
          </div>
        </div>
      )}

      {stage === 'failed' && (
        <div className="mt-3 rounded-md border border-red-100 bg-red-50 p-3">
          <p className="flex items-center gap-2 text-[13px] font-semibold text-red-700">
            <ShieldAlert size={15} /> This record could not be confirmed
          </p>
          <p className="mt-1 text-[12.5px] text-red-700">
            {result?.reason === 'NOT_ISSUED'
              ? 'The ledger has no entry with this reference. Treat it as untrusted and report it.'
              : 'The sealed copy does not match the ledger. Treat it as untrusted and report it.'}
          </p>
        </div>
      )}
    </div>
  );
}