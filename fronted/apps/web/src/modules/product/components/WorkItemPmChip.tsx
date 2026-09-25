"use client";

import type { WorkItemExternalRef } from "@sparkops/contracts/product";
import { PM_PROVIDER_LABEL } from "./pm-labels";

interface WorkItemPmChipProps {
  external?: WorkItemExternalRef;
  disabled?: boolean;
  onManage: () => void;
  onLink?: () => void;
  compact?: boolean;
}

export function WorkItemPmChip({ external, disabled, onManage, onLink, compact = false }: WorkItemPmChipProps) {
  if (!external?.issue_key) {
    if (!onLink) return null;
    return (
      <button
        type="button"
        className="work-item-pm-link-btn"
        disabled={disabled}
        onClick={(event) => {
          event.stopPropagation();
          onLink();
        }}
      >
        关联 PM
      </button>
    );
  }

  const providerLabel = PM_PROVIDER_LABEL[external.provider] ?? external.provider;

  return (
    <button
      type="button"
      className={`work-item-pm-chip${compact ? " is-compact" : ""}`}
      disabled={disabled}
      title={`${providerLabel} · ${external.issue_key}，点击管理关联`}
      onClick={(event) => {
        event.stopPropagation();
        onManage();
      }}
    >
      <span className="work-item-pm-chip-provider">{providerLabel}</span>
      <span className="work-item-pm-chip-key">{external.issue_key}</span>
    </button>
  );
}
