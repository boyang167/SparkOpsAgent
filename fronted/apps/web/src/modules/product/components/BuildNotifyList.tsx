"use client";

import { useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { WorkflowNotifyFields } from "./WorkflowNotifyFields";
import {
  emptyNotifyItem,
  notifyChannelLabel,
  validateWorkflowNotification,
  WORKFLOW_NOTIFY_EVENTS,
  type NotifyItem,
} from "./workflow-notify";

interface BuildNotifyListProps {
  items: NotifyItem[];
  onChange: (items: NotifyItem[]) => void;
}

export function BuildNotifyList({ items, onChange }: BuildNotifyListProps) {
  const [draft, setDraft] = useState<NotifyItem | null>(null);
  const [error, setError] = useState<string | null>(null);
  const editing = Boolean(draft && items.some((item) => item.id === draft.id));

  function save() {
    if (!draft) return;
    const message = validateWorkflowNotification(draft, true);
    if (message) {
      setError(message);
      return;
    }
    const next = { ...draft, webhook_url: draft.webhook_url.trim(), targets: draft.targets.trim() };
    onChange(items.some((item) => item.id === next.id) ? items.map((item) => (item.id === next.id ? next : item)) : [...items, next]);
    setDraft(null);
    setError(null);
  }

  return (
    <div className="pwf-notify-panel">
      <div className="pwf-notify-toolbar">
        <button
          type="button"
          className="btn btn-secondary text-sm"
          onClick={() => {
            setError(null);
            setDraft(emptyNotifyItem());
          }}
        >
          + 添加
        </button>
        <span className="text-xs text-muted">已配置 {items.length} 个通知</span>
      </div>
      <table className="pwf-notify-table">
        <thead>
          <tr>
            <th>类型</th>
            <th>通知事件</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {items.length === 0 ? (
            <tr>
              <td colSpan={3} className="pwf-notify-empty">
                还没有通知
              </td>
            </tr>
          ) : (
            items.map((item) => (
              <tr key={item.id}>
                <td>
                  <div className="pwf-notify-type">
                    <button
                      type="button"
                      className={`menu-switch${item.enabled ? " menu-switch-on" : ""}`}
                      aria-pressed={item.enabled}
                      aria-label={item.enabled ? "关闭通知" : "开启通知"}
                      onClick={() => onChange(items.map((row) => (row.id === item.id ? { ...row, enabled: !row.enabled } : row)))}
                    >
                      <span className="menu-switch-thumb" />
                    </button>
                    <span>{notifyChannelLabel(item.channel) || "未选择"}</span>
                  </div>
                </td>
                <td>
                  <div className="pwf-notify-tags">
                    {WORKFLOW_NOTIFY_EVENTS.filter((event) => item.events.includes(event.id)).map((event) => (
                      <span key={event.id} className="pwf-notify-tag">
                        {event.label}
                      </span>
                    ))}
                  </div>
                </td>
                <td>
                  <div className="pwf-notify-item-actions">
                    <button type="button" className="pwf-icon-btn" aria-label="编辑通知" onClick={() => {
                      setError(null);
                      setDraft(item);
                    }}>
                      <svg viewBox="0 0 24 24" aria-hidden>
                        <path d="M4 20h4l10.5-10.5a2.1 2.1 0 0 0-3-3L5 17v3Z" />
                        <path d="M13.5 6.5l3 3" />
                      </svg>
                    </button>
                    <button
                      type="button"
                      className="pwf-icon-btn is-danger"
                      aria-label="删除通知"
                      onClick={() => onChange(items.filter((row) => row.id !== item.id))}
                    >
                      <svg viewBox="0 0 24 24" aria-hidden>
                        <path d="M4 7h16" />
                        <path d="M9 7V5h6v2" />
                        <path d="M8 7l1 12h6l1-12" />
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
      <Modal
        open={draft !== null}
        priority
        size="lg"
        title={editing ? "编辑通知" : "新建通知"}
        onClose={() => setDraft(null)}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setDraft(null)}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" onClick={save}>
              确定
            </button>
          </div>
        }
      >
        {draft ? (
          <WorkflowNotifyFields
            value={draft}
            error={error}
            onChange={(value) => setDraft({ ...draft, ...value })}
          />
        ) : null}
      </Modal>
    </div>
  );
}
