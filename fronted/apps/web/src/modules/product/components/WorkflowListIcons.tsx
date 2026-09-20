export function IconStar({ filled }: { filled?: boolean }) {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path
        d="M12 2l2.9 6.26L22 9.27l-5 4.87L18.18 22 12 18.56 5.82 22 7 14.14l-5-4.87 7.1-1.01L12 2z"
        fill={filled ? "currentColor" : "none"}
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function IconSort() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M8 6h12M8 12h8M8 18h4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M4 6v12M4 6l-2 2M4 6l2 2M4 18l-2-2M4 18l2-2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconSearch() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="11" cy="11" r="6.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path d="M16 16l4.5 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

const iconStroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.75,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

export function IconPlay() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M9 7.1v9.8c0 .7.78 1.12 1.36.72l7.1-4.9a.85.85 0 0 0 0-1.44l-7.1-4.9A.85.85 0 0 0 9 7.1Z" fill="currentColor" />
    </svg>
  );
}

export function IconSettings() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path
        d="M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <path
        d="M19.4 13a1 1 0 0 0 .2-1.1l-1-1.74a1 1 0 0 0-1-.45l-2 .8-1.2-.7-.24-1.88A1 1 0 0 0 13.2 7h-2.4a1 1 0 0 0-1 .92l-.24 1.88-1.2.7-2-.8a1 1 0 0 0-1 .45l-1 1.74a1 1 0 0 0 .2 1.1l1 1.74-.2 1.88 1.2.7 2-.8 1 .45 1 1.74a1 1 0 0 0 1 .45l2-.8 1.2.7.24 1.88a1 1 0 0 0 1 .92h2.4a1 1 0 0 0 1-.92l.24-1.88 1.2-.7 2 .8 1-.45 1-1.74a1 1 0 0 0-.2-1.1l-1-1.74.2-1.88-1.2-.7-2 .8-1-.45-1-1.74a1 1 0 0 0-1-.45l-2 .8-1.2-.7-.24-1.88a1 1 0 0 0-1-.92z"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function IconMore() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="6" cy="12" r="1.5" fill="currentColor" />
      <circle cx="12" cy="12" r="1.5" fill="currentColor" />
      <circle cx="18" cy="12" r="1.5" fill="currentColor" />
    </svg>
  );
}

export function IconPencil() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M12 20h9" {...iconStroke} />
      <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5Z" {...iconStroke} />
    </svg>
  );
}

export function IconRun() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <circle cx="12" cy="12" r="8" {...iconStroke} />
      <path d="M10.2 8.7v6.6L15.6 12 10.2 8.7Z" fill="currentColor" />
    </svg>
  );
}

export function IconProduct() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M21 8.2 12 3.5 3 8.2v7.6L12 20.5l9-4.7V8.2Z" {...iconStroke} />
      <path d="M3.4 8.2 12 12.6l8.6-4.4" {...iconStroke} />
      <path d="M12 20.5V12.6" {...iconStroke} />
    </svg>
  );
}
