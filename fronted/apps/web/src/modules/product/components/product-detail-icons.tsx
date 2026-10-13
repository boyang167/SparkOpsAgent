"use client";

export const productDetailTabIcons = {
  overview: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <rect x="3" y="3" width="7" height="7" rx="1" />
      <rect x="14" y="3" width="7" height="7" rx="1" />
      <rect x="3" y="14" width="7" height="7" rx="1" />
      <rect x="14" y="14" width="7" height="7" rx="1" />
    </svg>
  ),
  iterations: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <rect x="3" y="4" width="5" height="16" rx="1" />
      <rect x="10" y="4" width="5" height="11" rx="1" />
      <rect x="17" y="4" width="5" height="14" rx="1" />
    </svg>
  ),
  releases: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 3v12M8 11l4 4 4-4M5 21h14" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
  workflow: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <circle cx="6" cy="6" r="2.5" />
      <circle cx="18" cy="6" r="2.5" />
      <circle cx="12" cy="18" r="2.5" />
      <path d="M8.2 7.5 10 14M15.8 7.5 14 14M8 6h8" strokeLinecap="round" />
    </svg>
  ),
  environment: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 3c-4 4-7 7-7 11a7 7 0 1 0 14 0c0-4-3-7-7-11Z" strokeLinejoin="round" />
      <circle cx="12" cy="14" r="2.5" />
    </svg>
  ),
  projects: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <rect x="4" y="4" width="16" height="16" rx="2" />
      <path d="M9 9h6v6H9z" />
    </svg>
  ),
  versions: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 3 15 8h5l-4 3.5 1.5 6.5L12 16l-5.5 2L8 14.5 4 11h5L12 3Z" strokeLinejoin="round" />
    </svg>
  ),
  process: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <rect x="3" y="5" width="5" height="14" rx="1" />
      <rect x="10" y="5" width="5" height="10" rx="1" />
      <rect x="17" y="5" width="5" height="12" rx="1" />
    </svg>
  ),
  plus: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M10 4v12M4 10h12" strokeLinecap="round" />
    </svg>
  ),
  search: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M9 3.5a5.5 5.5 0 1 1 0 11 5.5 5.5 0 0 1 0-11Z" />
      <path d="M13.5 13.5 17 17" strokeLinecap="round" />
    </svg>
  ),
  more: (
    <svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
      <circle cx="5" cy="10" r="1.4" />
      <circle cx="10" cy="10" r="1.4" />
      <circle cx="15" cy="10" r="1.4" />
    </svg>
  ),
  sort: (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M6 4v12M6 16l-2.5-2.5M6 16l2.5-2.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M14 16V4M14 4l2.5 2.5M14 4l-2.5 2.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
  chevronRight: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M6 4l4 4-4 4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
  chevronDown: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M4 6l4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
} as const;
