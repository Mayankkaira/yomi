import { useEffect, useMemo, useState } from 'react';
import {
  Mail,
  Phone,
  Fingerprint,
  Building2,
  KeyRound,
  CheckCircle2,
  ShieldCheck,
} from 'lucide-react';
import { AppShell } from '../components/layout/AppShell';
import { PageHeader } from '../components/layout/PageHeader';
import { StatusBadge, SectorBadge } from '../components/common/Badge';
import { ScopeChips } from '../components/consent/ScopeChips';
import { EmptyState } from '../components/common/EmptyState';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { AccessTokenCheckPanel } from '../components/consent/AccessTokenCheckPanel';
import { useAuth } from '../context/AuthContext';
import { useConsent } from '../context/ConsentContext';
import { useToast } from '../context/ToastContext';
import { useInstitutionNames } from '../hooks/useAudit';
import { consentApi } from '../api/consent';
import { formatDate, formatDateTime, maskAadhaar, cn } from '../utils/format';
import { scopeLabels } from '../utils/scopes';
import type { ScopeSummary } from '../types';

export default function Profile() {
  const { citizen } = useAuth();
  const { history, loadHistory, revoke } = useConsent();
  const { push } = useToast();
  const [revokingId, setRevokingId] = useState<string | null>(null);
  const [confirmRevoke, setConfirmRevoke] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [catalog, setCatalog] = useState<ScopeSummary[]>([]);

  useEffect(() => {
    if (!citizen) return;
    void loadHistory(citizen.id);
  }, [citizen, loadHistory]);

  useEffect(() => {
    let cancelled = false;
    consentApi
      .scopeCatalog('admin')
      .then((data) => {
        if (!cancelled) setCatalog(data.scopes);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  const labels = useMemo(() => scopeLabels(catalog), [catalog]);

  const institutions = useInstitutionNames(history.map((h) => h.institutionId));

  const activeConsents = useMemo(
    () =>
      history.filter(
        (h) => h.status === 'approved' && new Date(h.expiresAt).getTime() > Date.now(),
      ),
    [history],
  );

  const encountered = useMemo(
    () => Array.from(new Set(history.map((h) => h.institutionId))).map((id) => institutions[id]).filter(Boolean),
    [history, institutions],
  );

  const handleRevoke = async (requestId: string) => {
    setRevokingId(requestId);
    setActionError(null);
    try {
      await revoke(requestId);
      setConfirmRevoke(null);
      push('success', 'Access revoked — the revocation is recorded on the audit ledger.');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Revocation failed';
      setActionError(message);
      push('error', message);
    } finally {
      setRevokingId(null);
    }
  };

  const selected = history.find((h) => h.id === confirmRevoke);

  return (
    <AppShell>
      <PageHeader title="Profile" description="Your identity, the institutions you have encountered, and active access tokens." />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <section className="panel h-fit" aria-label="Citizen identity">
          <div className="px-5 py-5">
            <div className="flex items-center gap-4">
              <div className="flex h-14 w-14 items-center justify-center rounded-panel bg-primary-50 text-[18px] font-bold text-primary-700">
                {citizen?.fullName.split(' ').map((p) => p[0]).slice(0, 2).join('').toUpperCase()}
              </div>
              <div>
                <h2 className="text-[16px] font-semibold text-ink">{citizen?.fullName}</h2>
                <p className="mt-0.5 flex items-center gap-1.5 text-[12.5px] text-ink-muted">
                  <ShieldCheck size={13} className="text-primary-600" />
                  Verified citizen
                </p>
              </div>
            </div>

            <dl className="mt-5 space-y-3 border-t border-line pt-5">
              <div className="flex items-center justify-between gap-3">
                <dt className="flex items-center gap-2 text-[12.5px] text-ink-muted">
                  <Fingerprint size={14} /> Aadhaar
                </dt>
                <dd className="font-mono text-[13px] font-medium text-ink">
                  {citizen ? maskAadhaar(citizen.aadhaarId) : '—'}
                </dd>
              </div>
              <div className="flex items-center justify-between gap-3">
                <dt className="flex items-center gap-2 text-[12.5px] text-ink-muted">
                  <Mail size={14} /> Email
                </dt>
                <dd className="text-[13px] text-ink">{citizen?.email}</dd>
              </div>
              <div className="flex items-center justify-between gap-3">
                <dt className="flex items-center gap-2 text-[12.5px] text-ink-muted">
                  <Phone size={14} /> Phone
                </dt>
                <dd className="font-mono text-[13px] text-ink">{citizen?.phone ?? '—'}</dd>
              </div>
              <div className="flex items-center justify-between gap-3">
                <dt className="text-[12.5px] text-ink-muted">Member since</dt>
                <dd className="data-value">{citizen ? formatDate(citizen.createdAt) : '—'}</dd>
              </div>
            </dl>
          </div>
        </section>

        <section className="panel lg:col-span-2" aria-label="Active consent tokens">
          <div className="panel-header">
            <div>
              <h2 className="flex items-center gap-2 text-[15px] font-semibold text-ink">
                <KeyRound size={15} className="text-primary-600" />
                Active consent tokens
              </h2>
              <p className="text-[12.5px] text-ink-muted">
                Approved authorizations that institutions can currently use
              </p>
            </div>
            <StatusBadge status={activeConsents.length > 0 ? 'approved' : 'expired'} />
          </div>

          {activeConsents.length === 0 ? (
            <EmptyState
              icon={<KeyRound size={20} />}
              title="No active access tokens"
              description="When you approve a data request, its authorization appears here and can be revoked at any time."
            />
          ) : (
            <ul className="divide-y divide-line">
              {activeConsents.map((consent) => {
                const inst = institutions[consent.institutionId];
                return (
                  <li key={consent.id} className="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <Building2 size={15} className="text-ink-muted" />
                        <p className="text-[13.5px] font-semibold text-ink">
                          {inst?.name ?? consent.institutionId.slice(0, 8)}
                        </p>
                        {inst && <SectorBadge sector={inst.sector} />}
                      </div>
                      <div className="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1.5">
                        <ScopeChips scopes={consent.dataScope} labels={labels} />
                        <span className="font-mono text-[11.5px] text-ink-muted">
                          expires {formatDateTime(consent.expiresAt)}
                        </span>
                      </div>
                      <p className="mt-1.5 text-[11.5px] text-ink-faint">
                        Revoking takes effect immediately — the institution's next check fails.
                      </p>
                    </div>
                    <Button variant="secondary" size="sm" onClick={() => setConfirmRevoke(consent.id)}>
                      Revoke access
                    </Button>
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      </div>

      <div className="mt-6">
        <AccessTokenCheckPanel />
      </div>

      <section className="panel mt-6" aria-label="Institutions encountered">
        <div className="panel-header">
          <div>
            <h2 className="text-[15px] font-semibold text-ink">Institutions encountered</h2>
            <p className="text-[12.5px] text-ink-muted">Organizations that have interacted with your consent records</p>
          </div>
        </div>

        {encountered.length === 0 ? (
          <EmptyState
            icon={<Building2 size={20} />}
            title="No institutions yet"
            description="Institutions that request your data will be listed here with their sector."
          />
        ) : (
          <ul className="grid grid-cols-1 gap-px bg-line sm:grid-cols-2 lg:grid-cols-3">
            {encountered.map((inst) => (
              <li key={inst.id} className={cn('flex items-center justify-between gap-3 bg-surface px-5 py-4')}>
                <div className="flex min-w-0 items-center gap-3">
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-line bg-canvas text-ink-muted">
                    <Building2 size={15} />
                  </span>
                  <div className="min-w-0">
                    <p className="truncate text-[13.5px] font-semibold text-ink">{inst.name}</p>
                    <p className="font-mono text-[11px] text-ink-faint">{inst.id.slice(0, 8)}</p>
                  </div>
                </div>
                <SectorBadge sector={inst.sector} />
              </li>
            ))}
          </ul>
        )}
      </section>

      <Modal
        open={confirmRevoke !== null}
        onClose={() => setConfirmRevoke(null)}
        title="Revoke this access token?"
        description="The institution will immediately lose the ability to access your data under this authorization. The revocation is recorded on the audit ledger."
        footer={
          <div className="flex items-center justify-end gap-2">
            {actionError && <p className="mr-auto text-[12.5px] text-red-600">{actionError}</p>}
            <Button variant="secondary" size="sm" onClick={() => setConfirmRevoke(null)}>
              Cancel
            </Button>
            <Button
              variant="danger"
              size="sm"
              loading={revokingId === confirmRevoke}
              onClick={() => confirmRevoke && void handleRevoke(confirmRevoke)}
            >
              Revoke access
            </Button>
          </div>
        }
      >
        {selected && (
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-[13.5px] font-medium text-ink">
              <CheckCircle2 size={15} className="text-primary-600" />
              {institutions[selected.institutionId]?.name ?? selected.institutionId.slice(0, 8)}
            </div>
            <p className="text-[12.5px] text-ink-muted">
              {selected.dataScope.map((s) => labels[s] ?? s).join(', ')}
            </p>
            <p className="data-value">approved {formatDateTime(selected.createdAt)}</p>
          </div>
        )}
      </Modal>
    </AppShell>
  );
}