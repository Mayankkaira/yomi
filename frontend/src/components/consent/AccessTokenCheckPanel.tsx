import { useState } from 'react';
import { Search, ShieldCheck, ShieldX, ShieldAlert, Loader2 } from 'lucide-react';
import { consentApi } from '../../api/consent';
import { errorMessage } from '../../api/client';
import { Button } from '../common/Button';
import { useToast } from '../../context/ToastContext';
import type { AccessTokenCheck } from '../../types';
import { formatDateTime } from '../../utils/format';

const REASON_TEXT: Record<string, string> = {
  EXPIRED: 'This access token has passed its expiry and no longer works.',
  REVOKED: 'This access token was revoked. The institution cannot use it anymore.',
  INVALID: 'This is not a token this ledger ever issued, or it has been altered.',
};

export function AccessTokenCheckPanel() {
  const { push } = useToast();
  const [value, setValue] = useState('');
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<AccessTokenCheck | null>(null);

  const check = async () => {
    const token = value.trim();
    if (!token) return;
    setBusy(true);
    setResult(null);
    try {
      setResult(await consentApi.checkAccessToken(token));
    } catch (err) {
      push('error', errorMessage(err).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel" aria-label="Check an access token">
      <div className="panel-header">
        <div>
          <h2 className="flex items-center gap-2 text-[15px] font-semibold text-ink">
            <Search size={15} className="text-primary-600" />
            Check an access token
          </h2>
          <p className="text-[12.5px] text-ink-muted">
            Paste a token an institution gave you and see whether it still works right now.
          </p>
        </div>
      </div>

      <div className="px-5 py-4">
        <div className="flex flex-wrap items-center gap-2">
          <input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="yomi_access_…"
            aria-label="Access token"
            spellCheck={false}
            className="h-9 min-w-0 flex-1 rounded-md border border-lineStrong bg-surface px-3 font-mono text-[12.5px] placeholder:text-ink-faint focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100"
          />
          <Button variant="primary" size="sm" loading={busy} onClick={() => void check()}>
            Check token
          </Button>
        </div>

        {busy && (
          <p className="mt-3 flex items-center gap-2 text-[12.5px] text-ink-muted">
            <Loader2 size={13} className="animate-spin" /> Asking the ledger…
          </p>
        )}

        {result && !busy && (
          <div
            className={
              result.active
                ? 'mt-3 rounded-md border border-primary-100 bg-primary-50 p-3'
                : 'mt-3 rounded-md border border-red-100 bg-red-50 p-3'
            }
          >
            {result.active ? (
              <>
                <p className="flex items-center gap-2 text-[13px] font-semibold text-primary-700">
                  <ShieldCheck size={15} /> Still valid
                </p>
                <dl className="mt-2 grid grid-cols-1 gap-x-6 gap-y-1.5 sm:grid-cols-2">
                  <div className="flex justify-between gap-3">
                    <dt className="text-[12px] text-ink-muted">Consent</dt>
                    <dd className="data-value">{result.requestId?.slice(0, 8)}</dd>
                  </div>
                  <div className="flex justify-between gap-3">
                    <dt className="text-[12px] text-ink-muted">Expires</dt>
                    <dd className="data-value">{result.expiresAt ? formatDateTime(result.expiresAt) : '—'}</dd>
                  </div>
                </dl>
              </>
            ) : (
              <>
                <p className="flex items-center gap-2 text-[13px] font-semibold text-red-700">
                  {result.reason === 'INVALID' ? <ShieldAlert size={15} /> : <ShieldX size={15} />}
                  Not usable
                </p>
                <p className="mt-1 text-[12.5px] text-red-700">
                  {REASON_TEXT[result.reason ?? 'INVALID'] ?? REASON_TEXT.INVALID}
                </p>
              </>
            )}
          </div>
        )}
      </div>
    </section>
  );
}