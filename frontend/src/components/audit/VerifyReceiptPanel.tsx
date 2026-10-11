import { useState } from 'react';
import { ShieldCheck, ShieldAlert, FileSearch, Loader2 } from 'lucide-react';
import { auditApi } from '../../api/audit';
import { errorMessage } from '../../api/client';
import { Button } from '../common/Button';
import { useToast } from '../../context/ToastContext';
import type { Receipt, ReceiptVerification } from '../../types';
import { ReceiptInspector } from './ReceiptInspector';
import { formatDateTime } from '../../utils/format';

export function VerifyReceiptPanel() {
  const { push } = useToast();
  const [raw, setRaw] = useState('');
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ReceiptVerification | null>(null);
  const [receipt, setReceipt] = useState<Receipt | null>(null);
  const [alteredFields, setAlteredFields] = useState<string[]>([]);

  const check = async () => {
    const text = raw.trim();
    if (!text) return;
    let parsed: Receipt;
    try {
      parsed = JSON.parse(text) as Receipt;
    } catch {
      push('error', 'That does not look like a receipt file. Paste the JSON exactly as it was saved.');
      return;
    }
    if (!parsed?.receiptId || !parsed?.signature) {
      push('error', 'The receipt is missing its id or signature.');
      return;
    }
    setBusy(true);
    setResult(null);
    try {
      const verified = await auditApi.verifyReceipt(parsed);
      setReceipt(parsed);
      setResult(verified);
      if (verified.valid && verified.receipt) {
        const drifted = (Object.keys(verified.receipt) as Array<keyof Receipt>).filter(
          (k) => JSON.stringify(verified.receipt?.[k]) !== JSON.stringify(parsed[k]),
        );
        setAlteredFields(drifted);
      } else {
        setAlteredFields([]);
      }
    } catch (err) {
      push('error', errorMessage(err).message);
    } finally {
      setBusy(false);
    }
  };

  const clear = () => {
    setRaw('');
    setResult(null);
    setReceipt(null);
    setAlteredFields([]);
  };

  return (
    <section className="panel" aria-label="Verify a saved receipt">
      <div className="panel-header">
        <div>
          <h2 className="flex items-center gap-2 text-[15px] font-semibold text-ink">
            <FileSearch size={15} className="text-primary-600" />
            Verify a saved receipt
          </h2>
          <p className="text-[12.5px] text-ink-muted">
            Paste a receipt you kept from an earlier approval, or one someone handed you, and check it against the
            ledger.
          </p>
        </div>
      </div>

      <div className="px-5 py-4">
        <textarea
          value={raw}
          onChange={(e) => setRaw(e.target.value)}
          rows={5}
          spellCheck={false}
          placeholder='{"receiptId":"…","version":"yomi-receipt/1", … }'
          aria-label="Receipt JSON"
          className="w-full rounded-md border border-lineStrong bg-surface px-3 py-2.5 font-mono text-[11.5px] leading-relaxed placeholder:text-ink-faint focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100"
        />
        <div className="mt-2 flex items-center gap-2">
          <Button variant="primary" size="sm" loading={busy} onClick={() => void check()}>
            Verify receipt
          </Button>
          <Button variant="secondary" size="sm" onClick={clear}>
            Clear
          </Button>
        </div>

        {busy && (
          <p className="mt-3 flex items-center gap-2 text-[12.5px] text-ink-muted">
            <Loader2 size={13} className="animate-spin" /> Rebuilding the seal from the ledger…
          </p>
        )}

        {result && !busy && (
          <>
            <div
              className={
                result.valid
                  ? 'mt-3 rounded-md border border-primary-100 bg-primary-50 p-3'
                  : 'mt-3 rounded-md border border-red-100 bg-red-50 p-3'
              }
            >
              {result.valid && result.receipt ? (
                <>
                  <p className="flex items-center gap-2 text-[13px] font-semibold text-primary-700">
                    <ShieldCheck size={15} /> Genuine — the ledger issued this
                  </p>
                  <p className="mt-1 text-[12.5px] text-primary-700">
                    Ledger position #{result.receipt.sequenceNo}, recorded {formatDateTime(result.receipt.recordedAt)}.
                    {result.chainAnchored
                      ? ' The entry still sits at that position in the chain.'
                      : ' The entry has moved — treat this as a red flag.'}
                  </p>
                  {alteredFields.length > 0 && (
                    <p className="mt-2 rounded border border-amber-200 bg-amber-50 px-2.5 py-2 text-[12.5px] text-amber-800">
                      Careful — the copy you pasted differs from the ledger in: {alteredFields.join(', ')}. The
                      authoritative values are the ones shown below, taken from the ledger, not from your file.
                    </p>
                  )}
                </>
              ) : (
                <>
                  <p className="flex items-center gap-2 text-[13px] font-semibold text-red-700">
                    <ShieldAlert size={15} /> Not genuine
                  </p>
                  <p className="mt-1 text-[12.5px] text-red-700">
                    {result.reason === 'NOT_ISSUED'
                      ? 'No entry in the ledger matches this reference, so the receipt was never issued here.'
                      : 'The signature does not match the ledger entry. The receipt has been altered or forged.'}
                  </p>
                </>
              )}
            </div>
            {receipt && <ReceiptInspector receipt={receipt} />}
          </>
        )}
      </div>
    </section>
  );
}