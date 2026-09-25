"use client";

import { useEffect, useMemo, useState } from "react";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { governance } from "@sparkops/web-sdk";
import { Drawer } from "@/components/ui/Drawer";
import { BuildNotifyList } from "./BuildNotifyList";
import {
  approvalJobSpecToRecord,
  formatApprovalProvider,
  parseApprovalJobSpec,
  type ApprovalJobSpec,
} from "./approval-job-spec";

type ApprovalTab = "basic" | "strategy" | "notify";

interface ApprovalJobConfigDrawerProps {
  open: boolean;
  workspaceId: string;
  jobName: string;
  spec: Record<string, unknown> | undefined;
  onClose: () => void;
  onConfirm: (name: string, spec: Record<string, unknown>) => void;
}

function memberLabel(member: WorkspaceMemberVM): string {
  return member.display_name?.trim() || member.email || member.member_id;
}

export function ApprovalJobConfigDrawer({
  open,
  workspaceId,
  jobName,
  spec,
  onClose,
  onConfirm,
}: ApprovalJobConfigDrawerProps) {
  const [tab, setTab] = useState<ApprovalTab>("basic");
  const [name, setName] = useState(jobName);
  const [draft, setDraft] = useState<ApprovalJobSpec>(() => parseApprovalJobSpec(spec));
  const [members, setMembers] = useState<WorkspaceMemberVM[]>([]);
  const [loadingMembers, setLoadingMembers] = useState(false);

  useEffect(() => {
    if (!open) return;
    setTab("basic");
    setName(jobName);
    setDraft(parseApprovalJobSpec(spec));
  }, [open, jobName, spec]);

  useEffect(() => {
    if (!open || !workspaceId) return;
    let cancelled = false;
    setLoadingMembers(true);
    void governance
      .listMembers(workspaceId)
      .then((items) => {
        if (!cancelled) setMembers(items);
      })
      .catch(() => {
        if (!cancelled) setMembers([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingMembers(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, workspaceId]);

  const maxRequired = Math.max(draft.approverIds.length, 1);
  const canConfirm = name.trim().length > 0 && draft.approverIds.length > 0 && draft.requiredApprovals >= 1;

  const selectedMembers = useMemo(
    () => members.filter((item) => draft.approverIds.includes(item.member_id)),
    [draft.approverIds, members],
  );

  function toggleApprover(memberId: string) {
    setDraft((current) => {
      const has = current.approverIds.includes(memberId);
      const approverIds = has
        ? current.approverIds.filter((id) => id !== memberId)
        : [...current.approverIds, memberId];
      const requiredApprovals = Math.min(current.requiredApprovals, Math.max(approverIds.length, 1));
      return { ...current, approverIds, requiredApprovals: approverIds.length === 0 ? 1 : requiredApprovals };
    });
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title="人工审批"
      panelClassName="drawer-panel-wide pwf-build-drawer"
      tabs={
        <>
          {(
            [
              ["basic", "基础配置"],
              ["strategy", "策略配置"],
              ["notify", "通知配置"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              className={`drawer-tab${tab === id ? " drawer-tab-active" : ""}`}
              onClick={() => setTab(id)}
            >
              {label}
            </button>
          ))}
          <span className="pwf-build-tab-spacer" />
          <button
            type="button"
            className="btn btn-primary text-sm"
            disabled={!canConfirm}
            onClick={() => onConfirm(name.trim(), approvalJobSpecToRecord(draft))}
          >
            确定
          </button>
          <button type="button" className="btn btn-secondary text-sm" onClick={onClose}>
            取消
          </button>
        </>
      }
    >
      {tab === "basic" ? (
        <div className="pwf-build-form">
          <label className="pwf-build-field">
            <span>任务名称</span>
            <span className="pwf-build-control">
              <span className="pwf-required" aria-hidden>
                *
              </span>
              <input className="input" value={name} onChange={(e) => setName(e.target.value)} aria-label="任务名称" />
            </span>
          </label>

          <label className="pwf-build-field">
            <span>审批方式</span>
            <span className="pwf-build-control">
              <select
                className="input"
                value={draft.provider}
                onChange={(e) =>
                  setDraft((current) => ({
                    ...current,
                    provider: e.target.value === "sparkops" ? "sparkops" : "sparkops",
                  }))
                }
                aria-label="审批方式"
              >
                <option value="sparkops">{formatApprovalProvider("sparkops")}</option>
              </select>
            </span>
          </label>

          <div className="pwf-build-field">
            <span>
              审批人
              <span className="pwf-required" aria-hidden>
                *
              </span>
            </span>
            <div className="pwf-approver-panel">
              {loadingMembers ? <p className="text-sm text-muted">加载成员…</p> : null}
              {!loadingMembers && members.length === 0 ? (
                <p className="text-sm text-muted">暂无可用工作空间成员</p>
              ) : null}
              {!loadingMembers && members.length > 0 ? (
                <div className="pwf-approver-list" role="listbox" aria-label="审批人" aria-multiselectable="true">
                  {members.map((member) => {
                    const selected = draft.approverIds.includes(member.member_id);
                    return (
                      <label key={member.member_id} className={`pwf-approver-option${selected ? " is-selected" : ""}`}>
                        <input
                          type="checkbox"
                          checked={selected}
                          onChange={() => toggleApprover(member.member_id)}
                        />
                        <span className="pwf-approver-option-copy">
                          <strong>{memberLabel(member)}</strong>
                          <span>{member.email}</span>
                        </span>
                      </label>
                    );
                  })}
                </div>
              ) : null}
              {selectedMembers.length > 0 ? (
                <div className="pwf-approver-pills">
                  {selectedMembers.map((member) => (
                    <span key={member.member_id} className="pwf-approver-pill">
                      {memberLabel(member)}
                    </span>
                  ))}
                </div>
              ) : null}
            </div>
          </div>

          <label className="pwf-build-field">
            <span>需要审批人数</span>
            <span className="pwf-build-control">
              <input
                className="input"
                type="number"
                min={1}
                max={maxRequired}
                value={draft.requiredApprovals}
                onChange={(e) =>
                  setDraft((current) => ({
                    ...current,
                    requiredApprovals: Math.min(
                      maxRequired,
                      Math.max(1, Number(e.target.value) || 1),
                    ),
                  }))
                }
                aria-label="需要审批人数"
              />
            </span>
          </label>

          <label className="pwf-build-field">
            <span>
              超时时间（分钟）
              <span className="pwf-required" aria-hidden>
                *
              </span>
            </span>
            <span className="pwf-build-control">
              <input
                className="input"
                type="number"
                min={1}
                value={draft.timeoutMinutes}
                onChange={(e) =>
                  setDraft((current) => ({
                    ...current,
                    timeoutMinutes: Math.max(1, Number(e.target.value) || 60),
                  }))
                }
                aria-label="超时时间（分钟）"
              />
            </span>
          </label>
        </div>
      ) : null}

      {tab === "strategy" ? (
        <div className="pwf-build-form stack-sm">
          <p className="text-sm text-muted">审批超时后，工作流将自动失败并停止后续任务。</p>
          <label className="pwf-build-field">
            <span>超时（分钟）</span>
            <input
              className="input"
              type="number"
              min={1}
              value={draft.timeoutMinutes}
              onChange={(e) =>
                setDraft((current) => ({
                  ...current,
                  timeoutMinutes: Math.max(1, Number(e.target.value) || 60),
                }))
              }
              aria-label="超时（分钟）"
            />
          </label>
        </div>
      ) : null}

      {tab === "notify" ? (
        <BuildNotifyList items={draft.notify} onChange={(notify) => setDraft((current) => ({ ...current, notify }))} />
      ) : null}
    </Drawer>
  );
}
