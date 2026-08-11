"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { secret } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { CreateSecretCommand, SecretVM, UpdateSecretCommand } from "@sparkops/contracts/secret";
import {
  SECRET_SERVICE_TYPES,
  SECRET_STATUS_OPTIONS,
  serviceTypeLabel,
} from "@sparkops/contracts/secret";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { TemplateCreator } from "@/modules/workspace/components/template/TemplateCreator";
import { formatDateTime } from "@/modules/workspace/components/template/template-utils";

interface SecretPanelProps {
  workspaceId: string;
}

const EMPTY_FORM: CreateSecretCommand = {
  name: "",
  display_name: "",
  description: "",
  service_type: "GitLab",
  service_url: "",
  username: "",
  credentials: {},
  status: "active",
  is_default: false,
  is_shared: false,
};

function statusBadge(status?: string) {
  return status === "active" ? "badge badge-success" : "badge badge-neutral";
}

export function SecretPanel({ workspaceId }: SecretPanelProps) {
  const [items, setItems] = useState<SecretVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [keyword, setKeyword] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [formOpen, setFormOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [editing, setEditing] = useState<SecretVM | null>(null);
  const [selected, setSelected] = useState<SecretVM | null>(null);
  const [form, setForm] = useState<CreateSecretCommand>(EMPTY_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await secret.listSecrets(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const kw = keyword.trim().toLowerCase();
    return items.filter((item) => {
      if (typeFilter && item.service_type !== typeFilter) return false;
      if (statusFilter && item.status !== statusFilter) return false;
      if (!kw) return true;
      return [item.name, item.display_name, item.service_url, item.username]
        .some((v) => v?.toLowerCase().includes(kw));
    });
  }, [items, keyword, typeFilter, statusFilter]);

  function openCreate() {
    setEditing(null);
    setForm(EMPTY_FORM);
    setFormOpen(true);
  }

  function openEdit(item: SecretVM) {
    setEditing(item);
    setForm({
      name: item.name,
      display_name: item.display_name ?? "",
      description: item.description ?? "",
      service_type: item.service_type,
      service_url: item.service_url,
      username: item.username ?? "",
      credentials: {},
      status: item.status ?? "active",
      is_default: item.is_default ?? false,
      is_shared: item.is_shared ?? false,
    });
    setFormOpen(true);
  }

  function openDetail(item: SecretVM) {
    setSelected(item);
    setDetailOpen(true);
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess(null);
    try {
      if (editing) {
        const payload: UpdateSecretCommand = {
          display_name: form.display_name,
          description: form.description,
          service_type: form.service_type,
          service_url: form.service_url,
          username: form.username,
          credentials: form.credentials,
          status: form.status,
          is_default: form.is_default,
          is_shared: form.is_shared,
        };
        await secret.updateSecret(workspaceId, editing.id, payload);
        setSuccess("配置已更新");
      } else {
        await secret.createSecret(workspaceId, form);
        setSuccess("配置已创建");
      }
      setFormOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function onDelete(item: SecretVM) {
    if (!window.confirm(`确定删除配置「${item.name}」？此操作不可恢复。`)) return;
    setError(null);
    try {
      await secret.deleteSecret(workspaceId, item.id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    }
  }

  async function onTest(item: SecretVM) {
    setTestingId(item.id);
    setError(null);
    setSuccess(null);
    try {
      const result = await secret.testSecretConnection(workspaceId, item.id);
      setSuccess(result.message);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "连接测试失败");
    } finally {
      setTestingId(null);
    }
  }

  return (
    <div className="secret-panel stack-lg">
      <InlineAlert tone="info">
        敏感凭据使用 AES-256-GCM 加密存储，API 仅返回脱敏后的值。编辑时留空表示不修改对应字段。
      </InlineAlert>

      <div className="secret-toolbar">
        <span className="template-total-badge">总数：{filtered.length}</span>
        <button type="button" className="btn" onClick={openCreate}>创建配置</button>
      </div>

      <div className="template-search-bar">
        <div className="template-search-grid">
          <label className="template-search-field">
            <span>配置名称</span>
            <input className="input" value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="搜索名称或地址" />
          </label>
          <label className="template-search-field">
            <span>服务类型</span>
            <select className="input" value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
              <option value="">全部</option>
              {SECRET_SERVICE_TYPES.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </label>
          <label className="template-search-field">
            <span>状态</span>
            <select className="input" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
              <option value="">全部</option>
              {SECRET_STATUS_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </label>
          <div className="template-search-field template-search-reset">
            <button type="button" className="btn-secondary" onClick={() => { setKeyword(""); setTypeFilter(""); setStatusFilter(""); }}>重置</button>
          </div>
        </div>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="info">{success}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : filtered.length === 0 ? (
        <div className="template-empty">
          <p>暂无服务连接配置</p>
          <button type="button" className="btn" onClick={openCreate}>创建第一个配置</button>
        </div>
      ) : (
        <div className="template-table-shell">
          <div className="table-wrap">
            <table className="data-table template-table text-sm">
              <thead>
                <tr>
                  <th>配置名称</th>
                  <th>服务类型</th>
                  <th>服务地址</th>
                  <th>用户名</th>
                  <th>状态</th>
                  <th>标签</th>
                  <th>创建者</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <div className="font-medium">{item.name}</div>
                      {item.display_name ? <div className="text-xs text-muted-foreground">{item.display_name}</div> : null}
                    </td>
                    <td><span className="badge badge-primary">{serviceTypeLabel(item.service_type)}</span></td>
                    <td className="max-w-[12rem] truncate" title={item.service_url}>{item.service_url}</td>
                    <td>{item.username || "-"}</td>
                    <td><span className={statusBadge(item.status)}>{item.status === "active" ? "活跃" : "非活跃"}</span></td>
                    <td>
                      <div className="flex flex-wrap gap-1">
                        {item.is_default ? <span className="badge badge-warning">默认</span> : null}
                        {item.is_shared ? <span className="badge badge-success">共享</span> : null}
                      </div>
                    </td>
                    <td><TemplateCreator template={item} compact /></td>
                    <td>{formatDateTime(item.created_at)}</td>
                    <td>
                      <div className="template-row-actions">
                        <button type="button" onClick={() => openDetail(item)}>查看</button>
                        <button type="button" onClick={() => openEdit(item)}>编辑</button>
                        <button type="button" disabled={testingId === item.id} onClick={() => void onTest(item)}>
                          {testingId === item.id ? "测试中" : "测试"}
                        </button>
                        <button type="button" className="is-danger" onClick={() => void onDelete(item)}>删除</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <Modal open={formOpen} onClose={() => setFormOpen(false)} title={editing ? "编辑配置" : "创建配置"} size="lg">
        <form onSubmit={onSubmit} className="stack-md">
          {!editing ? (
            <label className="template-field">
              <span className="template-field-label">配置名称</span>
              <input className="input" required minLength={3} maxLength={100} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </label>
          ) : null}
          <div className="template-form-grid">
            <label className="template-field">
              <span className="template-field-label">显示名称</span>
              <input className="input" value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })} />
            </label>
            <label className="template-field">
              <span className="template-field-label">服务类型</span>
              <select className="input" value={form.service_type} onChange={(e) => setForm({ ...form, service_type: e.target.value })} required>
                {SECRET_SERVICE_TYPES.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            </label>
          </div>
          <label className="template-field">
            <span className="template-field-label">服务地址</span>
            <input className="input" required value={form.service_url} onChange={(e) => setForm({ ...form, service_url: e.target.value })} placeholder="https://gitlab.example.com" />
          </label>
          <label className="template-field">
            <span className="template-field-label">用户名</span>
            <input className="input" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
          </label>
          <div className="secret-credentials-grid">
            {(["password", "token", "api_key"] as const).map((key) => (
              <label key={key} className="template-field">
                <span className="template-field-label">{key === "api_key" ? "API Key" : key === "token" ? "Token" : "密码"}</span>
                <input
                  type="password"
                  className="input"
                  autoComplete="new-password"
                  placeholder={editing ? "留空则不修改" : ""}
                  value={form.credentials?.[key] ?? ""}
                  onChange={(e) => setForm({ ...form, credentials: { ...form.credentials, [key]: e.target.value } })}
                />
              </label>
            ))}
          </div>
          <label className="template-field">
            <span className="template-field-label">私钥</span>
            <textarea className="input font-mono text-xs min-h-24" placeholder={editing ? "留空则不修改" : ""} value={form.credentials?.private_key ?? ""} onChange={(e) => setForm({ ...form, credentials: { ...form.credentials, private_key: e.target.value } })} />
          </label>
          <div className="template-form-grid">
            <label className="template-field">
              <span className="template-field-label">状态</span>
              <select className="input" value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as CreateSecretCommand["status"] })}>
                {SECRET_STATUS_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            </label>
            <div className="stack-sm pt-4">
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.is_default} onChange={(e) => setForm({ ...form, is_default: e.target.checked })} />默认配置</label>
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.is_shared} onChange={(e) => setForm({ ...form, is_shared: e.target.checked })} />共享配置</label>
            </div>
          </div>
          <div className="template-form-actions">
            <button type="button" className="btn-secondary" onClick={() => setFormOpen(false)}>取消</button>
            <button type="submit" className="btn" disabled={submitting}>{submitting ? "保存中…" : "保存"}</button>
          </div>
        </form>
      </Modal>

      <Modal open={detailOpen} onClose={() => setDetailOpen(false)} title="配置详情" size="xl">
        {selected ? (
          <div className="stack-md">
            <dl className="template-info-grid">
              <div><dt>名称</dt><dd>{selected.name}</dd></div>
              <div><dt>服务类型</dt><dd>{serviceTypeLabel(selected.service_type)}</dd></div>
              <div><dt>服务地址</dt><dd>{selected.service_url}</dd></div>
              <div><dt>用户名</dt><dd>{selected.username || "-"}</dd></div>
              <div><dt>状态</dt><dd>{selected.status === "active" ? "活跃" : "非活跃"}</dd></div>
              <div><dt>创建者</dt><dd><TemplateCreator template={selected} /></dd></div>
            </dl>
            <div className="panel stack-sm">
              <h4 className="font-semibold">认证信息（已脱敏）</h4>
              <div className="secret-mask-grid">
                {selected.has_password ? <span>密码：{selected.credentials?.password ?? "******"}</span> : null}
                {selected.has_token ? <span>Token：{selected.credentials?.token ?? "******"}</span> : null}
                {selected.has_api_key ? <span>API Key：{selected.credentials?.api_key ?? "******"}</span> : null}
                {selected.has_private_key ? <span>私钥：******</span> : null}
                {!selected.has_password && !selected.has_token && !selected.has_api_key && !selected.has_private_key ? (
                  <span className="text-muted-foreground">未配置敏感字段</span>
                ) : null}
              </div>
            </div>
            <div className="flex gap-2">
              <button type="button" className="btn-secondary" onClick={() => { setDetailOpen(false); openEdit(selected); }}>编辑</button>
              <button type="button" className="btn-secondary" onClick={() => void onTest(selected)}>测试连接</button>
            </div>
          </div>
        ) : null}
      </Modal>
    </div>
  );
}
