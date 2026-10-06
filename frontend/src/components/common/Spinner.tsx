import { Loader2 } from 'lucide-react';
import { cn } from '../../utils/format';

/** Centered spinner with optional label. */
export function Spinner({ label, className }: { label?: string; className?: string }) {
  return (
    <div className={cn('flex flex-col items-center justify-center gap-3 py-16 text-ink-muted', className)} role="status">
      <Loader2 size={26} className="animate-spin text-primary-600" />
      {label && <p className="text-[13px]">{label}</p>}
      <span className="sr-only">Loading</span>
    </div>
  );
}
