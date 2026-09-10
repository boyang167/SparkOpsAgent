"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { ProjectWebhookVM, WebhookEventCatalogItem } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import {
  WEBHOOK_EVENT_OPTIONS,
  isValidWebhookName,
  isValidWebhookURL,
  moveWebhookEvents,
} from "./webhook-form";

interface ProjectWebhookTabProps {
  workspaceId: string;
  project: ProjectVM;
  active: boolean;
}

function formatWebhookError(err: unknown, fallback: string) {
  if (!(err instanceof BffError)) return fallback;
  if (err.status === 404 || /invalid response|not found/i.test(err.message)) {
    return "Webhook 服务未就绪（请确认后端已重启并加载最新接口）";
  }
  return err.message || fallback;
}

export function ProjectWebhookTab({ workspaceId, project, active }: ProjectWebhookTabProps) {
  const [items, setItems] = useState<ProjectWebhookVM[]>([]);
  const [catalog, setCatalog] = useState<WebhookEventCatalogItem[]>([...WEBHOOK_EVENT_OPTIONS]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ProjectWebhookVM | null>(null);
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [token, setToken] = useState("");
  const [enabled, setEnabled] = useState(false);
  const [events, setEvents] = useState<string[]>([]);
  const [picked, setPicked] = useState<{ side: "available" | "selected"; keys: string[] }>({
    side: "available",
    keys: [],
  });
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await delivery.listWebhooks(workspaceId, project.id);
      setItems(data.items ?? []);
      if (data.catalog?.length) setCatalog(data.catalog);
    } catch (err) {
      setError(formatWebhookError(err, "Webhook 加载失败"));
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    if (!active) return;
    void load();
  }, [active, load]);

  const available = useMemo(
    () => catalog.filter((item) => !events.includes(item.key)),
    [catalog, events],
  );
  const selected = useMemo(
    () => catalog.filter((item) => events.includes(item.key)),
    [catalog, events],
  );

  function openCreate() {
    setEditing(null);
    setName("");
    setUrl("");
    setToken("");
    setEnabled(false);
    setEvents([]);
    setPicked({ side: "available", keys: [] });
    setFormError(null);
    setOpen(true);
  }

  function openEdit(item: ProjectWebhookVM) {
    setEditing(item);
    setName(item.name);
    setUrl(item.url);
    setToken(item.token ?? "");
    setEnabled(item.enabled);
    setEvents(item.events ?? []);
    setPicked({ side: "available", keys: [] });
    setFormError(null);
    setOpen(true);
  }

  function move(toSelected: boolean) {
    const keys = picked.side === (toSelected ? "available" : "selected") ? picked.keys : [];
    if (!keys.length) return;
    setEvents((current) => moveWebhookEvents(current, keys, toSelected));
    setPicked({ side: toSelected ? "selected" : "available", keys });
  }

  async function handleSave() {
    if (!isValidWebhookName(name)) {
      setFormError("请输入名称");
      return;
    }
    if (!isValidWebhookURL(url)) {
      setFormError("请填写正确的 URL，例如 https://example.com/hook");
      return;
    }
    if (events.length < 1) {
      setFormError("请选择关注事件");
      return;
    }
    setSaving(true);
    setFormError(null);
    const input = { name: name.trim(), url: url.trim(), token: token.trim(), events, enabled };
    try {
      if (editing) {
        await delivery.updateWebhook(workspaceId, project.id, editing.id, input);
      } else {
        await delivery.createWebhook(workspaceId, project.id, input);
      }
      setOpen(false);
      await load();
    } catch (err) {
      setFormError(formatWebhookError(err, "保存 Webhook 失败"));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(item: ProjectWebhookVM) {
    if (!window.confirm(`确定删除 Webhook「${item.name}」？`)) return;
    setError(null);
    try {
      await delivery.deleteWebhook(workspaceId, project.id, item.id);
      await load();
    } catch (err) {
      setError(formatWebhookError(err, "删除失败"));
    }
  }

  const showEmpty = !loading && !error && items.length === 0;

  return (
    <div className="project-configmap-page">
      <header className="project-configmap-toolbar">
        <div>
          <h4 className="project-configmap-title">Webhooks</h4>
          <p className="project-configmap-subtitle">
            订阅构建、发布、回滚事件。事件发生后平台会向配置的 URL 发送 HTTP POST，可选带上 token。
          </p>
        </div>
        <div className="project-configmap-toolbar-actions">
          <button type="button" className="btn btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
            刷新
          </button>
          <button type="button" className="btn btn-primary text-sm" onClick={openCreate}>
            + 创建
          </button>
        </div>
      </header>

      {loading ? <p className="project-configmap-loading">Webhook 加载中…</p> : null}

      {error ? (
        <div className="project-configmap-alert">
          <p>{error}</p>
          <button type="button" className="btn btn-secondary text-sm" onClick={() => void load()}>
            重试
          </button>
        </div>
      ) : null}

      {showEmpty ? (
        <div className="project-configmap-empty">
          <strong>还没有 Webhook</strong>
          <p>点击右上角「创建」，填写回调地址并选择关注事件。</p>
        </div>
      ) : null}

      {!error && items.length > 0 ? (
        <div className="project-configmap-table-wrap">
          <table className="project-configmap-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>URL</th>
                <th>状态</th>
                <th>关注事件</th>
                <th style={{ width: 140 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td>{item.name}</td>
                  <td>
                    <code className="project-configmap-key">{item.url}</code>
                  </td>
                  <td>
                    <span className={`badge ${item.enabled ? "badge-success" : "badge-neutral"}`}>
                      {item.enabled ? "激活" : "关闭"}
                    </span>
                  </td>
                  <td>{item.events?.join(", ") || "-"}</td>
                  <td>
                    <div className="project-configmap-row-actions">
                      <button type="button" className="project-runtime-pod-link" onClick={() => openEdit(item)}>
                        编辑
                      </button>
                      <button
                        type="button"
                        className="project-runtime-pod-link is-danger"
                        onClick={() => void handleDelete(item)}
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      <Modal
        open={open}
        onClose={() => setOpen(false)}
        title={editing ? "编辑 Webhook" : "创建 Webhook"}
        size="lg"
        description="通过自定义 Webhook 监听构建、发布、回滚。激活后事件会触发 HTTP POST。"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn btn-secondary" onClick={() => setOpen(false)} disabled={saving}>
              取消
            </button>
            <button type="button" className="btn btn-primary" onClick={() => void handleSave()} disabled={saving}>
              {saving ? "保存中…" : "确定"}
            </button>
          </div>
        }
      >
        <div className="stack-md">
          {formError ? <p className="text-sm text-[var(--danger)]">{formError}</p> : null}
          <label className="stack-sm">
            <span>名称</span>
            <input className="input" value={name} onChange={(event) => setName(event.target.value)} placeholder="Webhook的名称" />
          </label>
          <label className="stack-sm">
            <span>URL</span>
            <input className="input" value={url} onChange={(event) => setUrl(event.target.value)} placeholder="调用URL地址" />
          </label>
          <fieldset className="stack-sm">
            <legend>激活状态</legend>
            <div className="project-webhook-radios">
              <label>
                <input type="radio" checked={!enabled} onChange={() => setEnabled(false)} /> 关闭
              </label>
              <label>
                <input type="radio" checked={enabled} onChange={() => setEnabled(true)} /> 激活
              </label>
            </div>
          </fieldset>
          <label className="stack-sm">
            <span>token</span>
            <input
              className="input"
              value={token}
              onChange={(event) => setToken(event.target.value)}
              placeholder="用于验证请求是该平台发出的（可选）"
            />
          </label>
          <div className="stack-sm">
            <span>关注事件</span>
            <div className="project-webhook-transfer">
              <EventList
                title={`${available.length} 项`}
                subtitle="可选项"
                items={available}
                selectedKeys={picked.side === "available" ? picked.keys : []}
                onToggle={(key) =>
                  setPicked((current) => {
                    if (current.side !== "available") return { side: "available", keys: [key] };
                    return {
                      side: "available",
                      keys: current.keys.includes(key)
                        ? current.keys.filter((item) => item !== key)
                        : [...current.keys, key],
                    };
                  })
                }
              />
              <div className="project-webhook-transfer-actions">
                <button type="button" className="btn btn-secondary text-sm" onClick={() => move(true)}>
                  &gt;
                </button>
                <button type="button" className="btn btn-secondary text-sm" onClick={() => move(false)}>
                  &lt;
                </button>
              </div>
              <EventList
                title={`${selected.length} 项`}
                subtitle="已选项"
                items={selected}
                selectedKeys={picked.side === "selected" ? picked.keys : []}
                empty="暂无数据"
                onToggle={(key) =>
                  setPicked((current) => {
                    if (current.side !== "selected") return { side: "selected", keys: [key] };
                    return {
                      side: "selected",
                      keys: current.keys.includes(key)
                        ? current.keys.filter((item) => item !== key)
                        : [...current.keys, key],
                    };
                  })
                }
              />
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
}

function EventList({
  title,
  subtitle,
  items,
  selectedKeys,
  empty,
  onToggle,
}: {
  title: string;
  subtitle: string;
  items: WebhookEventCatalogItem[];
  selectedKeys: string[];
  empty?: string;
  onToggle: (key: string) => void;
}) {
  return (
    <div className="project-webhook-list">
      <header>
        <strong>{title}</strong>
        <span>{subtitle}</span>
      </header>
      <ul>
        {items.length === 0 ? <li className="is-empty">{empty || "暂无数据"}</li> : null}
        {items.map((item) => (
          <li key={item.key}>
            <label>
              <input
                type="checkbox"
                checked={selectedKeys.includes(item.key)}
                onChange={() => onToggle(item.key)}
              />
              {item.label}
            </label>
          </li>
        ))}
      </ul>
    </div>
  );
}
