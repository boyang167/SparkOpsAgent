"use client";

import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  WORKFLOW_NOTIFY_CHANNELS,
  WORKFLOW_NOTIFY_EVENTS,
  channelNeedsWebhook,
  notifyChannelLabel,
  type NotifyConfig,
} from "./workflow-notify";

interface WorkflowNotifyFieldsProps {
  value: NotifyConfig;
  error?: string | null;
  onChange: (value: NotifyConfig) => void;
}

export function WorkflowNotifyFields({ value, error, onChange }: WorkflowNotifyFieldsProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuStyle, setMenuStyle] = useState<{ top: number; left: number; width: number; maxHeight: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const listId = useId();
  const allChecked = value.events.length === WORKFLOW_NOTIFY_EVENTS.length;
  const someChecked = value.events.length > 0 && !allChecked;

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

  function placeMenu() {
    const rect = triggerRef.current?.getBoundingClientRect();
    if (!rect) return;
    const spaceBelow = window.innerHeight - rect.bottom - 12;
    const spaceAbove = rect.top - 12;
    const openUp = spaceBelow < 180 && spaceAbove > spaceBelow;
    const maxHeight = Math.max(120, Math.min(240, openUp ? spaceAbove : spaceBelow));
    setMenuStyle({
      top: openUp ? Math.max(8, rect.top - maxHeight - 4) : rect.bottom + 4,
      left: rect.left,
      width: rect.width,
      maxHeight,
    });
  }

  function toggleEvent(id: string) {
    const events = value.events.includes(id) ? value.events.filter((item) => item !== id) : [...value.events, id];
    onChange({ ...value, events });
  }

  return (
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
            onChange={() =>
              onChange({
                ...value,
                events: allChecked ? [] : WORKFLOW_NOTIFY_EVENTS.map((item) => item.id),
              })
            }
          />
          全选
        </label>
        <div className="pwf-notify-events">
          {WORKFLOW_NOTIFY_EVENTS.map((item) => (
            <label key={item.id} className="pwf-check">
              <input type="checkbox" checked={value.events.includes(item.id)} onChange={() => toggleEvent(item.id)} />
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
            <span>{value.channel ? notifyChannelLabel(value.channel) : "请选择"}</span>
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
                        aria-selected={value.channel === item.id}
                        className={value.channel === item.id ? "is-active" : undefined}
                        onClick={() => {
                          onChange({ ...value, channel: item.id });
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
        {channelNeedsWebhook(value.channel) ? (
          <label className="field">
            <span className="text-xs text-muted">Webhook 地址</span>
            <input
              className="input"
              placeholder="https://"
              value={value.webhook_url}
              onChange={(e) => onChange({ ...value, webhook_url: e.target.value })}
            />
          </label>
        ) : null}
        {value.channel === "mail" ? (
          <label className="field">
            <span className="text-xs text-muted">收件人</span>
            <input
              className="input"
              placeholder="name@example.com，多个用逗号分隔"
              value={value.targets}
              onChange={(e) => onChange({ ...value, targets: e.target.value })}
            />
            <span className="text-xs text-muted">收件人会随任务保存。当前未接入邮件服务，暂不会发出。</span>
          </label>
        ) : null}
        {value.channel === "feishu_user" ? (
          <label className="field">
            <span className="text-xs text-muted">成员 Open ID</span>
            <input
              className="input"
              placeholder="多个用逗号分隔"
              value={value.targets}
              onChange={(e) => onChange({ ...value, targets: e.target.value })}
            />
            <span className="text-xs text-muted">成员通知会保存。发送需要飞书自建应用，当前尚未接入。</span>
          </label>
        ) : null}
      </section>
      {error ? <p className="text-xs text-danger">{error}</p> : null}
    </div>
  );
}
