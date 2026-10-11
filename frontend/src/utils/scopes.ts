import type { ScopeSummary } from '../types';

export function scopeLabels(catalog: ScopeSummary[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const s of catalog) out[s.key] = s.label;
  return out;
}