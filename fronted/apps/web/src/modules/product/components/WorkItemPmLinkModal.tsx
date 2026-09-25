"use client";

import { useEffect, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PmProvider, WorkItemExternalRef, WorkItemSyncMode } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { PM_PROVIDER_LABEL, PM_SYNC_MODE_LABEL } from "./pm-labels";

type ModalMode = "view" | "edit";

interface WorkItemPmLinkModalProps {
  open: boolean;
  onClose: () => void;
  workspaceId: string;
  productId: string;
  workItemId: string;
  workItemTitle?: string;
  external?: WorkItemExternalRef;
  readOnly?: boolean;
  onChanged: () => void;
}

function readPmWorkspaceId(external?: WorkItemExternalRef): string {
  return external?.pm_workspace_id ?? "";
}

export function WorkItemPmLinkModal({
  open,
  onClose,
  workspaceId,
  productId,
  workItemId,
  workItemTitle,
  external,
  readOnly = false,
  onChanged,
}: WorkItemPmLinkModalProps) {
  const [mode, setMode] = useState<ModalMode>("view");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [provider, setProvider] = useState<PmProvider>("jira");
  const [pmWorkspaceId, setPmWorkspaceId] = useState("");
  const [issueKey, setIssueKey] = useState("");
  const [syncMode, setSyncMode] = useState<WorkItemSyncMode>("link_only");
  const [unlinkConfirm, setUnlinkConfirm] = useState(false);

  useEffect(() => {
    if (!open) {
      setMode(external?.issue_key ? "view" : "edit");
      setError(null);
      setBusy(false);
      setUnlinkConfirm(false);
      return;
    }
    if (external?.issue_key) {
      setMode("view");
      setProvider(external.provider);
      setIssueKey(external.issue_key);
      setSyncMode(external.sync_mode ?? "link_only");
      setPmWorkspaceId(readPmWorkspaceId(external));
    } else {
      setMode("edit");
      setProvider("jira");
      setIssueKey("");
      setSyncMode("link_only");
      setPmWorkspaceId("");
    }
    setError(null);
    setUnlinkConfirm(false);
  }, [open, external]);

  async function onSaveLink() {
    if (!issueKey.trim()) return;
    if (provider === "tapd" && !pmWorkspaceId.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await product.linkPmIssue(workspaceId, productId, workItemId, {
        provider,
        pm_workspace_id: pmWorkspaceId.trim() || undefined,
        issue_key: issueKey.trim(),
        sync_mode: syncMode,
      });
      onChanged();
      onClose();
    } catch (err) {
      const msg =
        err instanceof BffError && err.code === "PM_ISSUE_ALREADY_LINKED"
          ? "该 Issue 已在本迭代中关联"
          : err instanceof BffError
            ? err.message
            : "关联 PM Issue 失败";
      setError(msg);
    } finally {
      setBusy(false);
    }
  }

  async function onUnlink() {
    setBusy(true);
    setError(null);
    try {
      await product.unlinkPmIssue(workspaceId, productId, workItemId);
      onChanged();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "解除关联失败");
    } finally {
      setBusy(false);
      setUnlinkConfirm(false);
    }
  }

  const hasLink = Boolean(external?.issue_key);
  const title = hasLink && mode === "view" ? "PM 关联" : hasLink ? "更换 PM 关联" : "关联 PM Issue";

  return (
    <Modal
      open={open}
      onClose={() => !busy && onClose()}
      title={title}
      description={workItemTitle}
      size="sm"
      nested
      priority
      panelClassName={mode === "view" ? "work-item-pm-modal" : undefined}
      footer={
        mode === "view" ? (
          <div className="modal-footer-actions">
            {external?.issue_url ? (
              <a
                className="btn-secondary text-sm"
                href={external.issue_url}
                target="_blank"
                rel="noreferrer"
              >
                在 PM 中打开
              </a>
            ) : null}
            {!readOnly ? (
              <>
                <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setMode("edit")}>
                  更换关联
                </button>
                <button
                  type="button"
                  className="btn-primary project-service-danger text-sm"
                  disabled={busy}
                  onClick={() => setUnlinkConfirm(true)}
                >
                  解除关联
                </button>
              </>
            ) : null}
          </div>
        ) : (
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => (hasLink ? setMode("view") : onClose())}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={busy || !issueKey.trim() || (provider === "tapd" && !pmWorkspaceId.trim())}
              onClick={() => void onSaveLink()}
            >
              {busy ? "保存中…" : hasLink ? "保存更改" : "关联"}
            </button>
          </div>
        )
      }
    >
      <div className="stack-sm">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        {mode === "view" && external ? (
          <div className="work-item-pm-summary stack-sm">
            <div className="work-item-pm-summary-row">
              <span className="work-item-pm-summary-label">工具</span>
              <span className="work-item-pm-summary-value">{PM_PROVIDER_LABEL[external.provider] ?? external.provider}</span>
            </div>
            <div className="work-item-pm-summary-row">
              <span className="work-item-pm-summary-label">Issue</span>
              <span className="work-item-pm-summary-value font-medium">{external.issue_key}</span>
            </div>
            {external.issue_type ? (
              <div className="work-item-pm-summary-row">
                <span className="work-item-pm-summary-label">类型</span>
                <span className="work-item-pm-summary-value">{external.issue_type}</span>
              </div>
            ) : null}
            <div className="work-item-pm-summary-row">
              <span className="work-item-pm-summary-label">同步</span>
              <span className="work-item-pm-summary-value">
                {PM_SYNC_MODE_LABEL[external.sync_mode ?? "link_only"]}
              </span>
            </div>
            <p className="text-xs text-muted-foreground">
              点击「更换关联」可修改 Issue；「在 PM 中打开」将在新标签页查看原始工单。
            </p>
          </div>
        ) : (
          <>
            <label className="stack-xs">
              <span className="text-sm font-medium">PM 工具</span>
              <select className="input" value={provider} disabled={busy} onChange={(e) => setProvider(e.target.value as PmProvider)}>
                {(Object.keys(PM_PROVIDER_LABEL) as PmProvider[]).map((key) => (
                  <option key={key} value={key}>
                    {PM_PROVIDER_LABEL[key]}
                  </option>
                ))}
              </select>
            </label>
            {provider === "tapd" ? (
              <label className="stack-xs">
                <span className="text-sm font-medium">TAPD 项目 ID</span>
                <input
                  className="input"
                  value={pmWorkspaceId}
                  disabled={busy}
                  onChange={(e) => setPmWorkspaceId(e.target.value)}
                  placeholder="例如 12345678"
                />
              </label>
            ) : null}
            <label className="stack-xs">
              <span className="text-sm font-medium">Issue Key / ID</span>
              <input
                className="input"
                value={issueKey}
                disabled={busy}
                onChange={(e) => setIssueKey(e.target.value)}
                placeholder={provider === "jira" ? "例如 PROJ-123" : "TAPD Story ID"}
              />
            </label>
            <label className="stack-xs">
              <span className="text-sm font-medium">同步模式</span>
              <select className="input" value={syncMode} disabled={busy} onChange={(e) => setSyncMode(e.target.value as WorkItemSyncMode)}>
                <option value="link_only">仅关联（不回写 PM）</option>
                <option value="bidirectional">双向同步（工作流成功后更新 PM 状态）</option>
              </select>
            </label>
          </>
        )}
      </div>

      <Modal
        open={unlinkConfirm}
        onClose={() => !busy && setUnlinkConfirm(false)}
        title="解除 PM 关联"
        size="sm"
        nested
        priority
        panelClassName="confirm-dialog"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setUnlinkConfirm(false)}>
              取消
            </button>
            <button type="button" className="btn-primary project-service-danger text-sm" disabled={busy} onClick={() => void onUnlink()}>
              {busy ? "解除中…" : "解除关联"}
            </button>
          </div>
        }
      >
        <p className="confirm-dialog-text">
          确定解除与 <strong>{external?.issue_key}</strong> 的关联？工作项本身不会被删除。
        </p>
      </Modal>
    </Modal>
  );
}
