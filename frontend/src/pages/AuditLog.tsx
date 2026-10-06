import { useEffect, useMemo, useState } from 'react';
import { ScrollText, RefreshCw } from 'lucide-react';
import { AppShell } from '../components/layout/AppShell';
import { PageHeader } from '../components/layout/PageHeader';
import { ChainIntegrityBanner } from '../components/audit/ChainIntegrityBanner';
import { AuditChainTable } from '../components/audit/AuditChainTable';
import { EmptyState } from '../components/common/EmptyState';
import { Spinner } from '../components/common/Spinner';
import { Button } from '../components/common/Button';
import { useAuth } from '../context/AuthContext';
import { useAudit, useInstitutionNames } from '../hooks/useAudit';

const PAGE_SIZE = 25;

export default function AuditLog() {
  const { citizen } = useAuth();
  const audit = useAudit(citizen?.id ?? null);
  const [page, setPage] = useState(1);

  useEffect(() => {
    setPage(1);
  }, [citizen?.id]);

  // Entries arrive newest-first; the chain must read oldest → newest.
  const ascending = useMemo(() => {
    return [...audit.entries].sort((a, b) => a.sequenceNo - b.sequenceNo);
  }, [audit.entries]);

  const validity = useMemo(() => {
    const map: Record<string, boolean> = {};
    for (const link of audit.chain) {
      map[link.entry.id] = link.valid;
    }
    return map;
  }, [audit.chain]);

  // Resolve institution names for the row display.
  const institutions = useInstitutionNames(audit.entries.map((e) => e.institutionId));
  const institutionNames = useMemo(() => {
    const names: Record<string, string> = {};
    for (const inst of Object.values(institutions)) {
      names[inst.id] = inst.name;
    }
    return names;
  }, [institutions]);

  const totalPages = Math.max(1, Math.ceil(ascending.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const pageItems = ascending.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  return (
    <AppShell>
      <PageHeader
        title="Audit ledger"
        description="An append-only, hash-chained record of every event touching your data. Each entry is cryptographically linked to the one before it."
        actions={
          <Button variant="secondary" size="sm" icon={<RefreshCw size={14} />} onClick={audit.refresh}>
            Re-verify chain
          </Button>
        }
      />

      <div className="mb-6">
        <ChainIntegrityBanner verification={audit.verification} />
      </div>

      {audit.loading && audit.entries.length === 0 ? (
        <div className="panel">
          <Spinner label="Verifying ledger…" />
        </div>
      ) : audit.entries.length === 0 ? (
        <div className="panel">
          <EmptyState
            icon={<ScrollText size={20} />}
            title="The ledger is empty"
            description="Once institutions request or access your data, each event will be appended here with a cryptographic hash chained to the previous entry."
          />
        </div>
      ) : (
        <>
          <AuditChainTable entries={pageItems} validity={validity} institutionNames={institutionNames} />

          <div className="mt-4 flex items-center justify-between">
            <p className="text-[12.5px] text-ink-muted">
              Showing{' '}
              <span className="font-mono font-medium text-ink-secondary">
                {(safePage - 1) * PAGE_SIZE + 1}–{Math.min(safePage * PAGE_SIZE, ascending.length)}
              </span>{' '}
              of <span className="font-mono font-medium text-ink-secondary">{ascending.length}</span> events
            </p>
            <div className="flex items-center gap-1.5">
              <Button variant="secondary" size="sm" disabled={safePage <= 1} onClick={() => setPage((p) => p - 1)}>
                Previous
              </Button>
              <span className="px-2 font-mono text-[12.5px] text-ink-muted">
                {safePage} / {totalPages}
              </span>
              <Button variant="secondary" size="sm" disabled={safePage >= totalPages} onClick={() => setPage((p) => p + 1)}>
                Next
              </Button>
            </div>
          </div>
        </>
      )}
    </AppShell>
  );
}
