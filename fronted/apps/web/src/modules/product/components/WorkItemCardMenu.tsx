"use client";

import { useEffect, useLayoutEffect, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import { governance, product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SprintStage, WorkItemCardVM } from "@sparkops/contracts/product";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

interface WorkItemCardMenuProps {
  item: WorkItemCardVM;
  stages: SprintStage[];
  workspaceId: string;
  productId: string;
  sprintActive: boolean;
  disabled?: boolean;
  open: boolean;
  onToggle: () => void;
  onClose: () => void;
  onChanged: () => void;
}

type DialogKind = "move" | "assignee" | "delete" | null;

const MENU_WIDTH = 168;
const MENU_HEIGHT = 132;
const GAP = 6;

function placeMenu(anchor: HTMLElement): CSSProperties {
  const rect = anchor.getBoundingClientRect();
  const margin = 8;
  const spaceBelow = window.innerHeight - rect.bottom - margin;
  const openUp = spaceBelow < MENU_HEIGHT && rect.top > MENU_HEIGHT;
  const right = Math.max(margin, window.innerWidth - rect.right);
  if (openUp) {
    return {
      position: "fixed",
      right,
      bottom: window.innerHeight - rect.top + GAP,
      width: MENU_WIDTH,
      zIndex: 80,
    };
  }
  return {
    position: "fixed",
    right,
    top: Math.min(rect.bottom + GAP, window.innerHeight - MENU_HEIGHT - margin),
    width: MENU_WIDTH,
    zIndex: 80,
  };
}

function IconMenuLines() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M5 7h14M5 12h14M5 17h14" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </svg>
  );
}

export function WorkItemCardMenu({
  item,
  stages,
  workspaceId,
  productId,
  sprintActive,
  disabled,
  open,
  onToggle,
  onClose,
  onChanged,
}: WorkItemCardMenuProps) {
  const [anchor, setAnchor] = useState<HTMLButtonElement | null>(null);
  const [style, setStyle] = useState<CSSProperties | null>(null);
  const [dialog, setDialog] = useState<DialogKind>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [targetStageId, setTargetStageId] = useState(item.stage_id);
  const [assigneeId, setAssigneeId] = useState(item.assignee_id ?? "");
  const [members, setMembers] = useState<WorkspaceMemberVM[]>([]);
  const [membersLoading, setMembersLoading] = useState(false);

  useLayoutEffect(() => {
    if (!open || !anchor) {
      setStyle(null);
      return;
    }
    const update = () => setStyle(placeMenu(anchor));
    update();
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
      window.removeEventListener("keydown", onKey);
    };
  }, [open, anchor, onClose]);

  useEffect(() => {
    if (dialog !== "assignee") return;
    setMembersLoading(true);
    setError(null);
    void governance
      .listMembers(workspaceId)
      .then(setMembers)
      .catch(() => setMembers([]))
      .finally(() => setMembersLoading(false));
  }, [dialog, workspaceId]);

  useEffect(() => {
    if (dialog === "move") setTargetStageId(item.stage_id);
    if (dialog === "assignee") setAssigneeId(item.assignee_id ?? "");
    if (dialog) setError(null);
  }, [dialog, item.assignee_id, item.stage_id]);

  function openDialog(kind: DialogKind) {
    onClose();
    setDialog(kind);
  }

  async function onConfirmMove() {
    if (targetStageId === item.stage_id) {
      setDialog(null);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await product.moveWorkItem(workspaceId, productId, item.id, {
        stage_id: targetStageId,
        sort_order: item.sort_order,
        version: item.version ?? 0,
      });
      setDialog(null);
      onChanged();
    } catch (err) {
      const msg =
        err instanceof BffError && err.code === "STAGE_GATE_DENIED"
          ? "阶段准入未满足（WIP 已满、缺少分支或工作流未成功等）"
          : err instanceof BffError && err.code === "WORK_ITEM_VERSION_CONFLICT"
            ? "工作项已被他人修改，请刷新后重试"
            : err instanceof BffError
              ? err.message
              : "移动工作项失败";
      setError(msg);
    } finally {
      setBusy(false);
    }
  }

  async function onConfirmAssignee() {
    setBusy(true);
    setError(null);
    try {
      if (!assigneeId) {
        await product.updateWorkItem(workspaceId, productId, item.id, {
          clear_assignee: true,
        });
      } else {
        await product.updateWorkItem(workspaceId, productId, item.id, {
          assignee_id: assigneeId,
        });
      }
      setDialog(null);
      onChanged();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更改负责人失败");
    } finally {
      setBusy(false);
    }
  }

  async function onConfirmDelete() {
    setBusy(true);
    setError(null);
    try {
      await product.deleteWorkItem(workspaceId, productId, item.id);
      setDialog(null);
      onChanged();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除工作项失败");
    } finally {
      setBusy(false);
    }
  }

  const moveDisabled = !sprintActive || disabled;

  return (
    <>
      <div className="sprint-board-card-menu-wrap">
        <button
          ref={setAnchor}
          type="button"
          className={`sprint-board-card-menu-btn${open ? " is-active" : ""}`}
          aria-haspopup="menu"
          aria-expanded={open}
          aria-label={`${item.title} 更多操作`}
          disabled={disabled}
          onClick={(event) => {
            event.stopPropagation();
            onToggle();
          }}
        >
          <IconMenuLines />
        </button>
        {open && style
          ? createPortal(
              <div
                className="wf-list-menu sprint-board-card-menu"
                role="menu"
                style={style}
                onClick={(event) => event.stopPropagation()}
              >
                <button
                  type="button"
                  className="wf-list-menu-item"
                  role="menuitem"
                  disabled={moveDisabled}
                  onClick={() => openDialog("move")}
                >
                  移动工作项
                </button>
                <button
                  type="button"
                  className="wf-list-menu-item"
                  role="menuitem"
                  disabled={disabled}
                  onClick={() => openDialog("assignee")}
                >
                  更改负责人
                </button>
                <button
                  type="button"
                  className="wf-list-menu-item is-danger"
                  role="menuitem"
                  disabled={disabled}
                  onClick={() => openDialog("delete")}
                >
                  删除
                </button>
              </div>,
              document.body,
            )
          : null}
      </div>

      <Modal
        open={dialog === "move"}
        onClose={() => !busy && setDialog(null)}
        title="移动工作项"
        description={item.title}
        size="sm"
        nested
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setDialog(null)}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onConfirmMove()}>
              {busy ? "移动中…" : "确认移动"}
            </button>
          </div>
        }
      >
        <div className="stack-sm">
          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
          <label className="stack-xs">
            <span className="text-sm font-medium">目标阶段</span>
            <select
              className="input"
              value={targetStageId}
              disabled={busy}
              onChange={(e) => setTargetStageId(e.target.value)}
            >
              {stages.map((stage) => (
                <option key={stage.id} value={stage.id}>
                  {stage.name}
                </option>
              ))}
            </select>
          </label>
        </div>
      </Modal>

      <Modal
        open={dialog === "assignee"}
        onClose={() => !busy && setDialog(null)}
        title="更改负责人"
        description={item.title}
        size="sm"
        nested
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setDialog(null)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={busy || membersLoading}
              onClick={() => void onConfirmAssignee()}
            >
              {busy ? "保存中…" : "保存"}
            </button>
          </div>
        }
      >
        <div className="stack-sm">
          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
          <label className="stack-xs">
            <span className="text-sm font-medium">负责人</span>
            <select
              className="input"
              value={assigneeId}
              disabled={busy || membersLoading}
              onChange={(e) => setAssigneeId(e.target.value)}
            >
              <option value="">未分配</option>
              {members.map((member) => (
                <option key={member.member_id} value={member.member_id}>
                  {member.display_name || member.email}
                </option>
              ))}
            </select>
          </label>
        </div>
      </Modal>

      <Modal
        open={dialog === "delete"}
        onClose={() => !busy && setDialog(null)}
        title="删除工作项"
        size="sm"
        nested
        panelClassName="confirm-dialog"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary text-sm" disabled={busy} onClick={() => setDialog(null)}>
              取消
            </button>
            <button
              type="button"
              className="btn-primary project-service-danger text-sm"
              disabled={busy}
              onClick={() => void onConfirmDelete()}
            >
              {busy ? "删除中…" : "删除"}
            </button>
          </div>
        }
      >
        <div className="stack-sm">
          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
          <div className="confirm-dialog-body">
            <div className="confirm-dialog-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M12 9v4M12 17h.01" strokeLinecap="round" />
                <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" strokeLinejoin="round" />
              </svg>
            </div>
            <p className="confirm-dialog-text">
              确定删除工作项 <strong>{item.title}</strong>？删除后无法恢复。
            </p>
          </div>
        </div>
      </Modal>
    </>
  );
}
