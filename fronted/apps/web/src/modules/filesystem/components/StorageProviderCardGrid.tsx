"use client";

import type { StoragePoolVM, StorageProviderVM } from "@sparkops/contracts/filesystem";
import {
  boundPoolNamesForProvider,
  formatProviderCapabilities,
  providerHealthStatus,
  providerKindSubtitle,
} from "./storage-pool-panel-view";

function ProviderCloudIcon() {
  return (
    <svg
      className="filesystem-provider-card-icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      aria-hidden
    >
      <path
        d="M7 18h10a4 4 0 00.3-8 5.5 5.5 0 00-10.6-1.2A3.5 3.5 0 007 18z"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function ProviderSettingsIcon() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 15a3 3 0 100-6 3 3 0 000 6z" strokeLinecap="round" />
      <path
        d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"
        strokeLinecap="round"
      />
    </svg>
  );
}

export type StorageProviderCardGridProps = {
  providers: StorageProviderVM[];
  pools: StoragePoolVM[];
  canAdmin: boolean;
  canDelete: (provider: StorageProviderVM) => boolean;
  onEdit: (provider: StorageProviderVM) => void;
  onDelete: (provider: StorageProviderVM) => void;
};

export function StorageProviderCardGrid({
  providers,
  pools,
  canAdmin,
  canDelete,
  onEdit,
  onDelete,
}: StorageProviderCardGridProps) {
  return (
    <div className="filesystem-provider-grid">
      {providers.map((provider) => {
        const health = providerHealthStatus(provider.status);
        const boundPools = boundPoolNamesForProvider(provider.id, pools);
        const endpoint = provider.endpoint?.trim() || "—";

        return (
          <article key={provider.id} className="filesystem-provider-card">
            <div className="filesystem-provider-card-top">
              <ProviderCloudIcon />
              <span className={`filesystem-provider-badge is-${health.tone}`}>{health.label}</span>
            </div>
            <h3 className="filesystem-provider-card-title">{provider.name}</h3>
            <p className="filesystem-provider-card-subtitle">{providerKindSubtitle(provider)}</p>
            <p className="filesystem-provider-card-endpoint font-mono">{endpoint}</p>
            <p className="filesystem-provider-card-meta">{formatProviderCapabilities(provider)}</p>
            <p className="filesystem-provider-card-meta">
              绑定 Pool: {boundPools.length > 0 ? boundPools.join(", ") : "—"}
            </p>
            {canAdmin ? (
              <div className="filesystem-provider-card-actions">
                <button
                  type="button"
                  className="filesystem-provider-card-action"
                  aria-label={`编辑 ${provider.name}`}
                  onClick={() => onEdit(provider)}
                >
                  <ProviderSettingsIcon />
                </button>
                {canDelete(provider) ? (
                  <button
                    type="button"
                    className="filesystem-provider-card-action is-danger"
                    aria-label={`删除 ${provider.name}`}
                    onClick={() => onDelete(provider)}
                  >
                    删除
                  </button>
                ) : null}
              </div>
            ) : null}
          </article>
        );
      })}
    </div>
  );
}