"use client";

import type { ReactNode } from "react";

interface ProductDetailEmptyStateProps {
  title: string;
  description?: string;
  action?: ReactNode;
}

export function ProductDetailEmptyState({ title, description, action }: ProductDetailEmptyStateProps) {
  return (
    <div className="product-detail-empty" role="status">
      <div className="product-detail-empty-art" aria-hidden="true">
        <svg viewBox="0 0 200 160" fill="none">
          <rect x="40" y="48" width="72" height="56" rx="6" fill="var(--primary-soft)" stroke="var(--primary)" strokeWidth="1.5" />
          <rect x="52" y="60" width="48" height="8" rx="2" fill="color-mix(in srgb, var(--primary) 25%, white)" />
          <rect x="52" y="74" width="32" height="6" rx="2" fill="color-mix(in srgb, var(--primary) 15%, white)" />
          <circle cx="148" cy="72" r="18" fill="var(--surface-muted)" stroke="var(--border)" strokeWidth="1.5" />
          <path d="M140 72l6 6 12-12" stroke="var(--primary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          <circle cx="96" cy="124" r="10" fill="var(--surface)" stroke="var(--border)" strokeWidth="1.5" />
          <path d="M96 118v12M90 124h12" stroke="var(--primary)" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </div>
      <p className="product-detail-empty-title">{title}</p>
      {description ? <p className="product-detail-empty-desc">{description}</p> : null}
      {action ? <div className="product-detail-empty-action">{action}</div> : null}
    </div>
  );
}
