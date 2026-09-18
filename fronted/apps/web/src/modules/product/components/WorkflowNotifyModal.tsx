"use client";

import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { WorkflowNotificationVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import {
  WORKFLOW_NOTIFY_CHANNELS,
  WORKFLOW_NOTIFY_EVENTS,
  channelNeedsWebhook,
  notifyChannelLabel,
  validateWorkflowNotification,
} from "./workflow-notify";

interface WorkflowNotifyModalProps {
  open: boolean;
  initial: WorkflowNotificationVM | null;
  onClose: () => void;
  onConfirm: (item: WorkflowNotificationVM) => void;
}

export function WorkflowNotifyModal({ open, initial, onClose, onConfirm }: WorkflowNotifyModalProps) {
  const [draft, setDraft] = useState<WorkflowNotificationVM | null>(initial);
  const [error, setError] = useState<string | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuStyle, setMenuStyle] = useState<{ top: number; left: number; width: number; maxHeight: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const listId = useId();

  useEffect(() => {
    if (!open) return;
    setDraft(initial);
    setError(null);
    setMenuOpen(false);
  }, [open, initial]);

  useEffect(() => {
    if (!menuOpen) return;
    function onPointer(event: MouseEvent) {
      const target = event.target;
      if (target instanceof Node && triggerRef.current?.contains(target)) return;
      if (target instanceof Element && target.closest(`[data-menu="${listId}"]`)) return;
      setMenuOpen(false);
    }
    document.addEventListener("mousedown", onPointer);
    return () => document.removeEventListener("mousedown", onPointer);
  }, [menuOpen, listId]);

  if (!draft) return null;

  const allChecked = draft.events.length === WORKFLOW_NOTIFY_EVENTS.length;
  const someChecked = draft.events.length > 0 && !allChecked;
  const editing = Boolean(initial?.channel || (initial?.events.length ?? 0) > 0);

  function placeMenu() {
    const rect = triggerRef.current?.getBoundingClientRect();
    if (!rect) return;
    const spaceBelow = window.innerHeight - rect.bottom - 12;
    const maxHeight = Math.max(120, Math.min(240, spaceBelow));
    setMenuStyle({ top: rect.bottom + 4, left: rect.left, width: rect.width, maxHeight });
  }

  function toggleEvent(id: string) {
    setDraft((current) => {
      if (!current) return current;
      const events = current.events.includes(id)
        ? current.events.filter((item) => item !== id)
        : [...current.events, id];
      return { ...current, events };
    });
  }

  function toggleAll() {
    setDraft((current) => {
      if (!current) return current;
      return {
        ...current,
        events: current.events.length === WORKFLOW_NOTIFY_EVENTS.length ? [] : WORKFLOW_NOTIFY_EVENTS.map((item) => item.id),
      };
    });
  }

  return (
    <Modal
      open={open}
      priority
      size="lg"
      title={editing ? "编辑通知" : "新建通知"}
      onClose={onClose}
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn btn-secondary text-sm" onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn btn-primary text-sm"
            onClick={() => {
              const message = validateWorkflowNotification(draft);
              if (message) {
                setError(message);
                return;
              }
              onConfirm({
                ...draft,
                webhook_url: draft.webhook_url.trim(),
                targets: draft.targets.trim(),
              });
            }}
          >
            确定
          </button>
        </div>
      }
    >
      <div className="pwf-notify-form">
        <section className="pwf-notify-section">
          <span className="pwf-notify-label">
            通知事件 <span className="pwf-req">*</span>
          </span>
          <label className="pwf-check">
            <input
              type="checkbox"
              checked={allChecked}
              ref={(node) => {
                if (node) node.indeterminate = someChecked;
              }}
              onChange={toggleAll}
            />
            全选
          </label>
          <div className="pwf-notify-events">
            {WORKFLOW_NOTIFY_EVENTS.map((item) => (
              <label key={item.id} className="pwf-check">
                <input
                  type="checkbox"
                  checked={draft.events.includes(item.id)}
                  onChange={() => toggleEvent(item.id)}
                />
                {item.label}
              </label>
            ))}
          </div>
        </section>

        <section className="pwf-notify-section">
          <span className="pwf-notify-label">
            通知类型 <span className="pwf-req">*</span>
          </span>
          <div className="pwf-combo">
            <button
              ref={triggerRef}
              type="button"
              className="input pwf-combo-trigger"
              aria-label="通知类型"
              aria-expanded={menuOpen}
              aria-controls={listId}
              onClick={() => {
                if (menuOpen) {
                  setMenuOpen(false);
                  return;
                }
                placeMenu();
                setMenuOpen(true);
              }}
            >
              <span>{draft.channel ? notifyChannelLabel(draft.channel) : "请选择"}</span>
              <span aria-hidden>{menuOpen ? "▴" : "▾"}</span>
            </button>
            {menuOpen && menuStyle
              ? createPortal(
                  <ul id={listId} data-menu={listId} className="pwf-combo-menu" role="listbox" style={menuStyle}>
                    {WORKFLOW_NOTIFY_CHANNELS.map((item) => (
                      <li key={item.id}>
                        <button
                          type="button"
                          role="option"
                          aria-selected={draft.channel === item.id}
                          className={draft.channel === item.id ? "is-active" : undefined}
                          onClick={() => {
                            setDraft((current) => (current ? { ...current, channel: item.id } : current));
                            setMenuOpen(false);
                          }}
                        >
                          {item.label}
                        </button>
                      </li>
                    ))}
                  </ul>,
                  document.body,
                )
              : null}
          </div>
          {channelNeedsWebhook(draft.channel) ? (
            <label className="field">
              <span className="text-xs text-muted">Webhook 地址</span>
              <input
                className="input"
                placeholder="https://"
                value={draft.webhook_url}
                onChange={(e) => setDraft({ ...draft, webhook_url: e.target.value })}
              />
            </label>
          ) : null}
          {draft.channel === "mail" ? (
            <label className="field">
              <span className="text-xs text-muted">收件人</span>
              <input
                className="input"
                placeholder="name@example.com，多个用逗号分隔"
                value={draft.targets}
                onChange={(e) => setDraft({ ...draft, targets: e.target.value })}
              />
              <span className="text-xs text-muted">收件人会随工作流保存。当前未接入邮件服务，暂不会发出。</span>
            </label>
          ) : null}
          {draft.channel === "feishu_user" ? (
            <label className="field">
              <span className="text-xs text-muted">成员 Open ID</span>
              <input
                className="input"
                placeholder="多个用逗号分隔"
                value={draft.targets}
                onChange={(e) => setDraft({ ...draft, targets: e.target.value })}
              />
              <span className="text-xs text-muted">成员通知会保存。发送需要飞书自建应用，当前尚未接入。</span>
            </label>
          ) : null}
        </section>
        {error ? <p className="text-xs text-danger">{error}</p> : null}
      </div>
    </Modal>
  );
}
