import { useState } from 'react';
import { Copy, Check, Terminal } from 'lucide-react';
import type { Receipt } from '../../types';
import { useToast } from '../../context/ToastContext';

interface ReceiptInspectorProps {
  receipt: Receipt;
}

function CopyRow({ label, value }: { label: string; value: string }) {
  const { push } = useToast();
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      push('error', 'Clipboard access denied by the browser.');
    }
  };

  return (
    <div className="min-w-0">
      <p className="label">{label}</p>
      <button
        onClick={() => void copy()}
        title={value}
        className="group mt-0.5 flex max-w-full items-center gap-1.5 font-mono text-[11.5px] text-ink-secondary transition-colors hover:text-primary-700"
      >
        <span className="truncate">{value}</span>
        {copied ? (
          <Check size={11} className="shrink-0 text-primary-600" />
        ) : (
          <Copy size={11} className="shrink-0 text-ink-faint transition-colors group-hover:text-primary-600" />
        )}
      </button>
    </div>
  );
}

export function ReceiptInspector({ receipt }: ReceiptInspectorProps) {
  const [open, setOpen] = useState(false);
  const command = `curl -sX POST http://localhost:8080/audit/receipt/verify -H 'Content-Type: application/json' -d @yomi-receipt-${receipt.sequenceNo}.json`;

  return (
    <div className="mt-4 rounded-md border border-line bg-canvas">
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left"
      >
        <span className="flex items-center gap-2 text-[12.5px] font-semibold text-ink">
          <Terminal size={14} className="text-ink-muted" />
          Verify it yourself, without trusting this page
        </span>
        <span className="text-[12px] text-ink-muted">{open ? 'hide' : 'show'}</span>
      </button>

      {open && (
        <div className="border-t border-line px-4 py-4">
          <p className="text-[12.5px] leading-relaxed text-ink-muted">
            The seal is an HMAC-SHA256 over the canonical fields below. Reproduce the string, hash it with the
            receipt key, and compare to the signature. Any edit to any field breaks the match.
          </p>

          <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <CopyRow label="Receipt id" value={receipt.receiptId} />
            <CopyRow label="Canonical form" value={receipt.content} />
            <CopyRow label="Entry hash (SHA-256)" value={receipt.entryHash} />
            <CopyRow label="Previous entry hash" value={receipt.prevHash} />
            <CopyRow label="HMAC-SHA256 signature" value={receipt.signature} />
          </div>

          <div className="mt-4">
            <p className="label">Canonical string</p>
            <pre className="mt-1.5 overflow-x-auto rounded-md border border-line bg-surface px-3 py-2.5 font-mono text-[11.5px] leading-relaxed text-ink-secondary">
              {receipt.content}
            </pre>
          </div>

          <div className="mt-4">
            <p className="label">Re-verify against the service</p>
            <pre className="mt-1.5 overflow-x-auto rounded-md border border-line bg-surface px-3 py-2.5 font-mono text-[11.5px] leading-relaxed text-ink-secondary">
              {command}
            </pre>
            <p className="mt-1.5 text-[11.5px] text-ink-faint">
              Or rebuild the signature offline: HMAC-SHA256(secret, canonical string), hex encoded, where secret is
              the receipt key shared by the consent and audit services.
            </p>
          </div>

          {receipt.metadata && (
            <div className="mt-4">
              <p className="label">Signed metadata</p>
              <pre className="mt-1.5 overflow-x-auto rounded-md border border-line bg-surface px-3 py-2.5 font-mono text-[11.5px] leading-relaxed text-ink-secondary">
                {JSON.stringify(receipt.metadata, null, 2)}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}