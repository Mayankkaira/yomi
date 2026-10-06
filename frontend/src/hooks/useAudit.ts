import { useCallback, useEffect, useMemo, useState } from 'react';
import { auditApi } from '../api/audit';
import { errorMessage } from '../api/client';
import { institutionApi } from '../api/institution';
import type { AuditEntry, ChainLink, ChainVerification, Institution } from '../types';

export interface AuditView {
  entries: AuditEntry[];
  chain: ChainLink[];
  verification: ChainVerification | null;
  loading: boolean;
  error: string | null;
  refresh: () => void;
}

// useAudit loads a citizen's audit trail plus the full-chain verification, so
// the ledger view can show per-entry integrity. `refresh` re-runs both calls.
export function useAudit(citizenId: string | null, enabled = true): AuditView {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [chain, setChain] = useState<ChainLink[]>([]);
  const [verification, setVerification] = useState<ChainVerification | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    if (!enabled || !citizenId) return;
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const [trail, verify] = await Promise.all([
          auditApi.forCitizen(citizenId),
          auditApi.verify(),
        ]);
        if (!cancelled) {
          setEntries(trail);
          setChain(verify.chain);
          setVerification(verify);
        }
      } catch (err) {
        if (!cancelled) setError(errorMessage(err).message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    load();
  }, [citizenId, enabled, tick]);

  const refresh = useCallback(() => setTick((t) => t + 1), []);
  return { entries, chain, verification, loading, error, refresh };
}

// useInstitutionNames resolves a map of institutionId -> Institution for a set
// of institution IDs. Unknown or inaccessible IDs are skipped gracefully and
// simply stay absent from the map.
export function useInstitutionNames(ids: Array<string | null | undefined>): Record<string, Institution> {
  const [map, setMap] = useState<Record<string, Institution>>({});

  // The caller passes a freshly-built array every render; key on a sorted,
  // joined string so the memoized id list (and thus the fetch effect) only
  // changes when the actual set of IDs changes.
  const idKey = useMemo(
    () =>
      Array.from(new Set(ids.filter((id): id is string => Boolean(id))))
        .sort()
        .join(','),
    [ids],
  );

  const uniqueIds = useMemo(() => (idKey ? idKey.split(',') : []), [idKey]);

  useEffect(() => {
    if (uniqueIds.length === 0) return;
    let cancelled = false;

    const load = async () => {
      const entries = await Promise.all(
        uniqueIds.map(async (id) => {
          try {
            return [id, await institutionApi.get(id)] as const;
          } catch {
            return [id, null] as const;
          }
        }),
      );
      if (!cancelled) {
        setMap((prev) => {
          const next = { ...prev };
          let changed = false;
          for (const [id, inst] of entries) {
            if (inst && next[id]?.id !== inst.id) {
              next[id] = inst;
              changed = true;
            }
          }
          return changed ? next : prev;
        });
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [uniqueIds]);

  return map;
}
