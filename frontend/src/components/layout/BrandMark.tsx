/** Yomi brand mark: a ledger of three lines sealed with a verification check. */
export function BrandMark({ size = 28 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <rect x="1" y="1" width="30" height="30" rx="7" fill="#12694A" />
      <path
        d="M9 11.5h14M9 16h10M9 20.5h6"
        stroke="#D8EBE0"
        strokeWidth="2.2"
        strokeLinecap="round"
      />
      <path
        d="M20.5 19.5l2 2.5 3.5-4.5"
        stroke="#FFFFFF"
        strokeWidth="2.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
