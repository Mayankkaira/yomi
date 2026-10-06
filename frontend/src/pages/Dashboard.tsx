import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  Inbox,
  CheckCircle2,
  Building2,
  ScrollText,
  ArrowRight,
  FileQuestion,
} from 'lucide-react';
import { AppShell } from '../components/layout/AppShell';
import { PageHeader } from '../components/layout/PageHeader';
import { StatCard } from '../components/common/StatCard';
import { StatCardSkeleton } from '../components/common/Skeleton';
import { EmptyState } from '../components/common/EmptyState';
import { useAuth } from '../context/AuthContext';
import { useConsent } from '../context/ConsentContext';
import { useAudit, useInstitutionNames } from '../hooks/useAudit';
import { EventLabel } from '../components/audit/EventLabel';
import { formatRelative, truncateHash } from '../utils/format';

export default function Dashboard() {
  const { citizen } = useAuth();
  const { pending, history, loadPending, loadHistory } = useConsent();
  const audit = useAudit(citizen?.id ?? null);

  useEffect(() => {
    if (!citizen) return;
    void loadPending(citizen.id);
    void loadHistory(citizen.id);
  }, [citizen, loadPending, loadHistory]);

  const institutions = useInstitutionNames(history.map((h) => h.institutionId));

  const approved = history.filter((h) => h.status === 'approved').length;
  const rejected = history.filter((h) => h.status === 'rejected').length;
  const revoked = history.filter((h) => h.status === 'revoked').length;
  // Active consents can't go below zero (a revocation implies a prior approval).
  const active = Math.max(0, approved - revoked);

  const recent = audit.entries.slice(0, 6);

  return (
    <AppShell>
      <PageHeader
        title={`Welcome, ${citizen?.fullName.split(' ')[0] ?? 'Citizen'}`}
        description="Here is the current state of your data consent and its verifiable trail."
      />

      {/* Stat band — Mercury-style: hairline-divided columns, no card chrome */}
      <div className="border-y border-line">
        <div className="grid grid-cols-2 divide-x divide-line lg:grid-cols-4">
          {audit.loading && history.length === 0 ? (
            <>
              <div className="px-5 py-5"><StatCardSkeleton /></div>
              <div className="px-5 py-5"><StatCardSkeleton /></div>
              <div className="px-5 py-5"><StatCardSkeleton /></div>
              <div className="px-5 py-5"><StatCardSkeleton /></div>
            </>
          ) : (
            <>
              <StatCard
                label="Pending requests"
                value={pending.length}
                icon={<Inbox size={17} />}
                tone="amber"
                hint={pending.length > 0 ? 'Awaiting your decision' : 'All clear'}
                className="px-5 py-5"
              />
              <StatCard
                label="Active consents"
                value={active}
                icon={<CheckCircle2 size={17} />}
                tone="primary"
                hint={`${revoked} revoked`}
                className="px-5 py-5"
              />
              <StatCard
                label="Institutions"
                value={Object.keys(institutions).length}
                icon={<Building2 size={17} />}
                tone="blue"
                hint="Encountered to date"
                className="px-5 py-5"
              />
              <StatCard
                label="Ledger events"
                value={audit.entries.length}
                icon={<ScrollText size={17} />}
                tone="slate"
                hint="Signed & hash-chained"
                className="px-5 py-5"
              />
            </>
          )}
        </div>
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-5">
        {/* Recent activity */}
        <section className="panel lg:col-span-3" aria-label="Recent ledger activity">
          <div className="panel-header">
            <div>
              <h2 className="text-[15px] font-semibold text-ink">Recent activity</h2>
              <p className="text-[12.5px] text-ink-muted">Latest entries on your audit chain</p>
            </div>
            <Link
              to="/audit"
              className="inline-flex items-center gap-1 text-[13px] font-semibold text-primary-600 hover:text-primary-700"
            >
              Full ledger <ArrowRight size={14} />
            </Link>
          </div>

          {audit.loading ? (
            <div className="divide-y divide-line">
              {[0, 1, 2, 3].map((i) => (
                <div key={i} className="flex items-center gap-4 px-5 py-3.5">
                  <div className="skeleton h-6 w-6 rounded" />
                  <div className="flex-1 space-y-1.5">
                    <div className="skeleton h-3 w-1/3" />
                    <div className="skeleton h-2.5 w-1/4" />
                  </div>
                </div>
              ))}
            </div>
          ) : recent.length === 0 ? (
            <EmptyState
              icon={<ScrollText size={20} />}
              title="No ledger activity yet"
              description="When institutions request or access your data, every event will be recorded here with its cryptographic hash."
            />
          ) : (
            <ul className="divide-y divide-line">
              {recent.map((entry) => {
                const instName = entry.institutionId
                  ? (institutions[entry.institutionId]?.name ?? entry.institutionId.slice(0, 8))
                  : null;
                return (
                  <li
                    key={entry.id}
                    className="group flex items-center justify-between gap-4 px-5 py-3 transition-colors duration-100 hover:bg-tint"
                  >
                    <div className="flex min-w-0 items-center gap-3">
                      <EventLabel eventType={entry.eventType} />
                      {instName && (
                        <span className="truncate text-[12.5px] text-ink-muted">
                          <span className="text-ink-faint">·</span> {instName}
                        </span>
                      )}
                    </div>
                    <div className="flex shrink-0 items-center gap-4">
                      <span className="data-value hidden sm:inline">{truncateHash(entry.entryHash, 8, 6)}</span>
                      <span className="w-16 text-right font-mono text-[11.5px] text-ink-muted">
                        {formatRelative(entry.createdAt)}
                      </span>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        {/* Quick actions + rejected summary */}
        <div className="space-y-6 lg:col-span-2">
          <section className="panel" aria-label="Quick actions">
            <div className="panel-header">
              <h2 className="text-[15px] font-semibold text-ink">Quick actions</h2>
            </div>
            <div className="space-y-2.5 p-4">
              <Link
                to="/consent/pending"
                className="flex items-center justify-between rounded-md border border-line bg-surface px-3.5 py-3 text-[13.5px] font-medium text-ink transition-colors hover:border-lineStrong hover:bg-canvas"
              >
                Review pending requests
                <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-amber-100 px-1.5 font-mono text-[11px] font-semibold text-amber-700">
                  {pending.length}
                </span>
              </Link>
              <Link
                to="/consent/history"
                className="flex items-center justify-between rounded-md border border-line bg-surface px-3.5 py-3 text-[13.5px] font-medium text-ink transition-colors hover:border-lineStrong hover:bg-canvas"
              >
                Review consent history
                <ArrowRight size={15} className="text-ink-muted" />
              </Link>
              <Link
                to="/audit"
                className="flex items-center justify-between rounded-md border border-line bg-surface px-3.5 py-3 text-[13.5px] font-medium text-ink transition-colors hover:border-lineStrong hover:bg-canvas"
              >
                Verify the audit ledger
                <ArrowRight size={15} className="text-ink-muted" />
              </Link>
            </div>
          </section>

          <section className="panel" aria-label="Decisions summary">
            <div className="panel-header">
              <h2 className="text-[15px] font-semibold text-ink">Your decisions</h2>
            </div>
            <dl className="divide-y divide-line">
              {[
                { label: 'Approved', value: approved, cls: 'text-primary-600' },
                { label: 'Rejected', value: rejected, cls: 'text-red-600' },
                { label: 'Revoked', value: revoked, cls: 'text-slate-500' },
              ].map((row) => (
                <div key={row.label} className="flex items-center justify-between px-5 py-3">
                  <dt className="text-[13px] text-ink-secondary">{row.label}</dt>
                  <dd className={`font-mono text-[15px] font-semibold ${row.cls}`}>{row.value}</dd>
                </div>
              ))}
              {history.length === 0 && !audit.loading && (
                <div className="px-5 py-6 text-center">
                  <FileQuestion size={18} className="mx-auto text-ink-faint" />
                  <p className="mt-2 text-[12.5px] text-ink-muted">No decisions recorded yet.</p>
                </div>
              )}
            </dl>
          </section>
        </div>
      </div>
    </AppShell>
  );
}
