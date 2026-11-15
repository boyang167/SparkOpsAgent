"use client";

import type { StorageProviderVM } from "@sparkops/contracts/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { StoragePoolMembersEditor } from "./StoragePoolMembersEditor";
import type { StoragePoolDraftMember } from "./storage-pool-panel-view";

const POOL_STATUSES = ["ACTIVE", "DISABLED"] as const;

export type StoragePoolModalFormProps = {
  error: string | null;
  name: string;
  status?: string;
  showStatus?: boolean;
  onNameChange: (value: string) => void;
  onStatusChange?: (value: string) => void;
  activeProviders: StorageProviderVM[];
  members: StoragePoolDraftMember[];
  onMembersChange: (members: StoragePoolDraftMember[]) => void;
  providerNameById: Map<string, string>;
  onClearError: () => void;
};

export function StoragePoolModalForm({
  error,
  name,
  status = "ACTIVE",
  showStatus = true,
  onNameChange,
  onStatusChange,
  activeProviders,
  members,
  onMembersChange,
  providerNameById,
  onClearError,
}: StoragePoolModalFormProps) {
  return (
    <div className="storage-pool-modal-form">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      <div className="storage-pool-modal-fields">
        <label className="form-field storage-pool-modal-field">
          <span className="form-label">名称</span>
          <input
            className="input"
            value={name}
            autoComplete="off"
            onChange={(e) => {
              onClearError();
              onNameChange(e.target.value);
            }}
          />
        </label>
        {showStatus && onStatusChange ? (
          <label className="form-field storage-pool-modal-field">
            <span className="form-label">状态</span>
            <select
              className="input"
              value={status}
              onChange={(e) => {
                onClearError();
                onStatusChange(e.target.value);
              }}
            >
              {POOL_STATUSES.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>
        ) : null}
      </div>
      <StoragePoolMembersEditor
        activeProviders={activeProviders}
        members={members}
        providerNameById={providerNameById}
        onChange={onMembersChange}
        onClearError={onClearError}
      />
    </div>
  );
}

export function StoragePoolModalFooter(props: {
  pending: boolean;
  submitDisabled?: boolean;
  onCancel: () => void;
  onSubmit: () => void;
  submitLabel: string;
}) {
  return (
    <div className="modal-footer-actions">
      <button type="button" className="btn btn-ghost" disabled={props.pending} onClick={props.onCancel}>
        取消
      </button>
      <button
        type="button"
        className="btn btn-primary"
        disabled={props.pending || props.submitDisabled}
        onClick={props.onSubmit}
      >
        {props.submitLabel}
      </button>
    </div>
  );
}