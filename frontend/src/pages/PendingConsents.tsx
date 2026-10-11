import { useEffect, useMemo, useState } from 'react';
import { Inbox, RefreshCw } from 'lucide-react';
import { AppShell } from '../components/layout/AppShell';
import { PageHeader } from '../components/layout/PageHeader';
import { ConsentRequestCard } from '../components/consent/ConsentRequestCard';
import { EmptyState } from '../components/common/EmptyState';
import { Button } from '../components/common/Button';
import { useAuth } from '../context/AuthContext';
import { useConsent } from '../context/ConsentContext';
import { useToast } from '../context/ToastContext';
import { useInstitutionNames } from '../hooks/useAudit';
import { usePoll } from '../hooks/usePoll';
import { consentApi } from '../api/consent';
import { scopeLabels } from '../utils/scopes';
import type { ScopeSummary } from '../types';

export default function PendingConsents() {
  const { citizen } = useAuth();
  const { pending, loadPending, approve, reject } = useConsent();
  const { push } = useToast();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [catalog, setCatalog] = useState<ScopeSummary[]>([]);

  const poll = usePoll(async () => {
    if (!citizen) return null;
    return consentApi.pending(citizen.id);
  }, 8000);

  useEffect(() => {
    if (!citizen) return;
    void loadPending(citizen.id);
  }, [citizen, loadPending]);

  useEffect(() => {
    if (poll.data && citizen) {
      void loadPending(citizen.id);
    }
  }, [poll.data, citizen, loadPending]);

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
  const institutions = useInstitutionNames(pending.map((r) => r.institutionId));

  const handleApprove = async (requestId: string) => {
    setBusyId(requestId);
    setError(null);
    try {
      const result = await approve(requestId);
      push('success', 'Approved. A receipt is ready in your audit ledger.');
      return result.accessToken;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Approval failed';
      setError(message);
      push('error', message);
      throw err;
    } finally {
      setBusyId(null);
    }
  };

  const handleReject = async (requestId: string) => {
    setBusyId(requestId);
    setError(null);
    try {
      await reject(requestId);
      push('info', 'Request rejected — the institution will not receive access.');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Rejection failed';
      setError(message);
      push('error', message);
      throw err;
    } finally {
      setBusyId(null);
    }
  };

  return (
    <AppShell>
      <PageHeader
        title="Pending requests"
        description="Institutions are waiting for your explicit decision. Approving issues a signed access token you can revoke at any time."
        actions={
          <Button
            variant="secondary"
            size="sm"
            icon={<RefreshCw size={14} />}
            onClick={() => citizen && void loadPending(citizen.id)}
          >
            Refresh
          </Button>
        }
      />

      {error && (
        <div className="mb-4 rounded-md border border-red-100 bg-red-50 px-3.5 py-2.5 text-[13px] text-red-700">
          {error}
        </div>
      )}

      {poll.loading && pending.length === 0 ? (
        <div className="space-y-4">
          {[0, 1].map((i) => (
            <div key={i} className="panel p-5">
              <div className="flex items-center gap-3">
                <div className="skeleton h-10 w-10 rounded-md" />
                <div className="flex-1 space-y-2">
                  <div className="skeleton h-4 w-1/3" />
                  <div className="skeleton h-3 w-1/4" />
                </div>
              </div>
              <div className="mt-4 space-y-2">
                <div className="skeleton h-3 w-full" />
                <div className="skeleton h-3 w-2/3" />
              </div>
            </div>
          ))}
        </div>
      ) : pending.length === 0 ? (
        <div className="panel">
          <EmptyState
            icon={<Inbox size={20} />}
            title="Nothing awaiting your decision"
            description="No institution has requested access to your data right now. New requests appear here automatically."
          />
        </div>
      ) : (
        <div className="panel divide-y divide-line">
          {pending.map((req) => (
            <ConsentRequestCard
              key={req.id}
              request={req}
              institution={institutions[req.institutionId]}
              scopeLabels={labels}
              onApprove={handleApprove}
              onReject={handleReject}
              busy={busyId === req.id}
            />
          ))}
          <p className="flex items-center gap-2 px-5 py-2.5 text-[12px] text-ink-faint">
            <RefreshCw size={12} className={poll.loading ? 'animate-spin' : ''} />
            Auto-refreshes every 8 seconds
          </p>
        </div>
      )}
    </AppShell>
  );
}