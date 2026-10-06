import { useEffect, useMemo, useState } from 'react';
import { History, ChevronUp, ChevronDown, ChevronsUpDown, Search } from 'lucide-react';
import { AppShell } from '../components/layout/AppShell';
import { PageHeader } from '../components/layout/PageHeader';
import { StatusBadge } from '../components/common/Badge';
import { ScopeChips } from '../components/consent/ScopeChips';
import { EmptyState } from '../components/common/EmptyState';
import { TableSkeleton } from '../components/common/Skeleton';
import { Button } from '../components/common/Button';
import { useAuth } from '../context/AuthContext';
import { useConsent } from '../context/ConsentContext';
import { useInstitutionNames } from '../hooks/useAudit';
import { formatDateTime, cn } from '../utils/format';
import type { ConsentStatus } from '../types';

type SortKey = 'createdAt' | 'status' | 'institutionId';
type SortDir = 'asc' | 'desc';

const PAGE_SIZE = 10;
const STATUS_FILTERS: Array<ConsentStatus | 'all'> = ['all', 'approved', 'pending', 'rejected', 'revoked', 'expired'];

function SortHeader({
  label,
  sortKey,
  active,
  dir,
  onSort,
}: {
  label: string;
  sortKey: SortKey;
  active: SortKey;
  dir: SortDir;
  onSort: (k: SortKey) => void;
}) {
  const Icon = active === sortKey ? (dir === 'asc' ? ChevronUp : ChevronDown) : ChevronsUpDown;
  return (
    <button
      onClick={() => onSort(sortKey)}
      className="inline-flex items-center gap-1 uppercase tracking-label hover:text-ink transition-colors"
    >
      {label}
      <Icon size={12} className={active === sortKey ? 'text-primary-600' : 'text-ink-faint'} />
    </button>
  );
}

export default function ConsentHistory() {
  const { citizen } = useAuth();
  const { history, loadingHistory, loadHistory, error } = useConsent();
  const [statusFilter, setStatusFilter] = useState<ConsentStatus | 'all'>('all');
  const [query, setQuery] = useState('');
  const [sortKey, setSortKey] = useState<SortKey>('createdAt');
  const [sortDir, setSortDir] = useState<SortDir>('desc');
  const [page, setPage] = useState(1);

  useEffect(() => {
    if (!citizen) return;
    void loadHistory(citizen.id);
  }, [citizen, loadHistory]);

  const institutions = useInstitutionNames(history.map((h) => h.institutionId));

  const filtered = useMemo(() => {
    let items = [...history];
    if (statusFilter !== 'all') items = items.filter((r) => r.status === statusFilter);
    if (query.trim()) {
      const q = query.trim().toLowerCase();
      items = items.filter(
        (r) =>
          r.purpose.toLowerCase().includes(q) ||
          r.dataScope.some((s) => s.toLowerCase().includes(q)) ||
          (institutions[r.institutionId]?.name.toLowerCase().includes(q) ?? false),
      );
    }
    items.sort((a, b) => {
      const av = a[sortKey] ?? '';
      const bv = b[sortKey] ?? '';
      const cmp = av < bv ? -1 : av > bv ? 1 : 0;
      return sortDir === 'asc' ? cmp : -cmp;
    });
    return items;
  }, [history, statusFilter, query, sortKey, sortDir, institutions]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const pageItems = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  const handleSort = (k: SortKey) => {
    if (k === sortKey) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(k);
      setSortDir(k === 'institutionId' ? 'asc' : 'desc');
    }
    setPage(1);
  };

  return (
    <AppShell>
      <PageHeader
        title="Consent history"
        description="Every request made against your data and the decision you recorded."
      />

      {/* Filters */}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-1.5" role="group" aria-label="Filter by status">
          {STATUS_FILTERS.map((s) => (
            <button
              key={s}
              onClick={() => {
                setStatusFilter(s);
                setPage(1);
              }}
              className={cn(
                'rounded-md border px-2.5 py-1.5 text-[12.5px] font-medium capitalize transition-colors',
                statusFilter === s
                  ? 'border-primary-500 bg-primary-50 text-primary-700'
                  : 'border-line bg-surface text-ink-muted hover:border-lineStrong hover:text-ink',
              )}
              aria-pressed={statusFilter === s}
            >
              {s}
            </button>
          ))}
        </div>
        <div className="relative w-full sm:w-64">
          <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-muted" />
          <input
            type="search"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setPage(1);
            }}
            placeholder="Search purpose, scope, institution…"
            aria-label="Search consent history"
            className="h-9 w-full rounded-md border border-lineStrong bg-surface pl-9 pr-3 text-[13px] placeholder:text-ink-faint focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100"
          />
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-md border border-red-100 bg-red-50 px-3.5 py-2.5 text-[13px] text-red-700">
          {error}
        </div>
      )}

      <div className="panel overflow-x-auto">
        {loadingHistory && history.length === 0 ? (
          <TableSkeleton rows={8} cols={5} />
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={<History size={20} />}
            title={query || statusFilter !== 'all' ? 'No matching decisions' : 'No consent decisions yet'}
            description={
              query || statusFilter !== 'all'
                ? 'Try clearing the search or switching the status filter.'
                : 'When institutions request your data, your decisions will appear here.'
            }
          />
        ) : (
          <table className="data-table min-w-[880px]">
            <thead>
              <tr>
                <th className="num">
                  <SortHeader label="Requested" sortKey="createdAt" active={sortKey} dir={sortDir} onSort={handleSort} />
                </th>
                <th>
                  <SortHeader label="Institution" sortKey="institutionId" active={sortKey} dir={sortDir} onSort={handleSort} />
                </th>
                <th>Purpose</th>
                <th>Data scope</th>
                <th>
                  <SortHeader label="Status" sortKey="status" active={sortKey} dir={sortDir} onSort={handleSort} />
                </th>
                <th className="num">Expires</th>
              </tr>
            </thead>
            <tbody>
              {pageItems.map((req) => (
                <tr key={req.id}>
                  <td className="num data-value whitespace-nowrap">{formatDateTime(req.createdAt)}</td>
                  <td className="text-[13px] font-medium text-ink">
                    {institutions[req.institutionId]?.name ?? req.institutionId.slice(0, 8)}
                    {institutions[req.institutionId] && (
                      <span className="ml-1.5 font-mono text-[11px] text-ink-faint">
                        {institutions[req.institutionId].sector}
                      </span>
                    )}
                  </td>
                  <td className="max-w-[260px]">
                    <p className="truncate text-[13px] text-ink-secondary" title={req.purpose}>
                      {req.purpose}
                    </p>
                  </td>
                  <td>
                    <ScopeChips scopes={req.dataScope} />
                  </td>
                  <td>
                    <StatusBadge status={req.status} />
                  </td>
                  <td className="num data-value whitespace-nowrap">{formatDateTime(req.expiresAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Pagination */}
      {filtered.length > 0 && (
        <div className="mt-4 flex items-center justify-between">
          <p className="text-[12.5px] text-ink-muted">
            Showing{' '}
            <span className="font-mono font-medium text-ink-secondary">
              {(safePage - 1) * PAGE_SIZE + 1}–{Math.min(safePage * PAGE_SIZE, filtered.length)}
            </span>{' '}
            of <span className="font-mono font-medium text-ink-secondary">{filtered.length}</span> decisions
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
      )}
    </AppShell>
  );
}
