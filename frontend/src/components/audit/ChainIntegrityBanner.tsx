import { ShieldCheck, ShieldAlert, Link2 } from 'lucide-react';
import type { ChainVerification } from '../../types';
import { IntegrityBadge } from '../common/Badge';
import { truncateHash } from '../../utils/format';

interface ChainIntegrityBannerProps {
  verification: ChainVerification | null;
}

/** Banner summarizing ledger integrity plus the genesis anchor. The
 *  "last verified" stamp is a GOV.UK-style transparency signal: it tells the
 *  citizen exactly when the integrity check was last performed. */
export function ChainIntegrityBanner({ verification }: ChainIntegrityBannerProps) {
  const intact = verification?.intact ?? true;
  const checked = verification?.entriesChecked ?? 0;
  const genesis = verification?.chain[0]?.entry.entryHash;

  return (
    <div className="panel overflow-hidden">
      <div className="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
        <div className="flex items-center gap-3">
          <span
            className={`flex h-9 w-9 items-center justify-center rounded-md border ${
              checked === 0
                ? 'border-line bg-canvas text-ink-muted'
                : intact
                  ? 'border-primary-100 bg-primary-50 text-primary-600'
                  : 'border-red-100 bg-red-50 text-red-600'
            }`}
          >
            {checked === 0 ? <Link2 size={17} /> : intact ? <ShieldCheck size={17} /> : <ShieldAlert size={17} />}
          </span>
          <div>
            <p className="text-[14px] font-semibold text-ink">Hash chain integrity</p>
            <p className="text-[12.5px] text-ink-muted">
              {checked === 0
                ? 'The ledger is empty — the chain will anchor at the first recorded event.'
                : intact
                  ? 'Every recorded entry recomputes to its stored hash. Nothing has been altered.'
                  : 'A recomputed hash does not match its stored value. The ledger has been tampered with.'}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-4">
          {checked > 0 && (
            <div className="text-right">
              <p className="label">Last verified</p>
              <p className="data-value mt-0.5">{new Date().toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}</p>
            </div>
          )}
          {genesis && (
            <div className="text-right">
              <p className="label">Genesis anchor</p>
              <p className="data-value mt-0.5">{truncateHash(genesis, 12, 8)}</p>
            </div>
          )}
          <IntegrityBadge intact={intact} checked={checked} />
        </div>
      </div>
    </div>
  );
}
