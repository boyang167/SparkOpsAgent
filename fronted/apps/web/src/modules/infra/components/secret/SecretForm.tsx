"use client";

import type { CreateSecretCommand } from "@sparkops/contracts/secret";
import { LockIcon, SettingIcon } from "./SecretIcons";
import { SECRET_SERVICE_TYPES, SECRET_STATUS_OPTIONS } from "./secret-utils";

interface SecretFormProps {
  form: CreateSecretCommand;
  isEdit: boolean;
  submitting: boolean;
  onChange: (form: CreateSecretCommand) => void;
  onSubmit: (e: React.FormEvent) => void;
  onCancel: () => void;
}

export function SecretForm({ form, isEdit, submitting, onChange, onSubmit, onCancel }: SecretFormProps) {
  const showGitLabFields = form.service_type === "GitLab";
  const showJenkinsFields = form.service_type === "Jenkins";

  return (
    <form onSubmit={onSubmit} className="secret-form stack-md">
      <section className="secret-form-section">
        <div className="secret-form-section-head">
          <SettingIcon />
          <span>基本信息</span>
        </div>
        <div className="secret-form-section-body stack-md">
          {!isEdit ? (
            <label className="template-field">
              <span className="template-field-label">配置名称</span>
              <input
                className="input"
                required
                minLength={3}
                maxLength={100}
                placeholder="请输入配置名称"
                value={form.name}
                onChange={(e) => onChange({ ...form, name: e.target.value })}
              />
            </label>
          ) : null}
          <div className="template-form-grid">
            <label className="template-field">
              <span className="template-field-label">显示名称</span>
              <input
                className="input"
                placeholder="请输入显示名称"
                value={form.display_name}
                onChange={(e) => onChange({ ...form, display_name: e.target.value })}
              />
            </label>
            <label className="template-field">
              <span className="template-field-label">服务类型</span>
              <select
                className="input"
                value={form.service_type}
                onChange={(e) => onChange({ ...form, service_type: e.target.value })}
                required
              >
                {SECRET_SERVICE_TYPES.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <label className="template-field">
            <span className="template-field-label">服务地址</span>
            <input
              className="input"
              required
              placeholder={
                form.service_type === "Harbor"
                  ? "https://192.168.5.24 或 http://harbor.example.com"
                  : "https://gitlab.example.com"
              }
              value={form.service_url}
              onChange={(e) => onChange({ ...form, service_url: e.target.value })}
            />
          </label>
          <label className="template-field">
            <span className="template-field-label">配置描述</span>
            <textarea
              className="input min-h-20"
              placeholder="请输入配置描述"
              value={form.description}
              onChange={(e) => onChange({ ...form, description: e.target.value })}
            />
          </label>
        </div>
      </section>

      <section className="secret-form-section">
        <div className="secret-form-section-head">
          <LockIcon />
          <span>认证信息</span>
        </div>
        <div className="secret-form-section-body stack-md">
          {showGitLabFields ? (
            <>
              <label className="template-field">
                <span className="template-field-label">访问令牌</span>
                <input
                  type="password"
                  className="input"
                  autoComplete="new-password"
                  placeholder={isEdit ? "留空则不修改" : "请输入访问令牌"}
                  value={form.credentials?.token ?? ""}
                  onChange={(e) =>
                    onChange({ ...form, credentials: { ...form.credentials, token: e.target.value } })
                  }
                />
              </label>
              <label className="template-field">
                <span className="template-field-label">额外配置 (JSON)</span>
                <textarea
                  className="input font-mono text-xs min-h-20"
                  placeholder={isEdit ? "留空则不修改" : ""}
                  value={form.credentials?.extra_config ?? ""}
                  onChange={(e) =>
                    onChange({
                      ...form,
                      credentials: { ...form.credentials, extra_config: e.target.value },
                    })
                  }
                />
              </label>
            </>
          ) : showJenkinsFields ? (
            <>
              <div className="template-form-grid">
                <label className="template-field">
                  <span className="template-field-label">用户名</span>
                  <input
                    className="input"
                    value={form.username}
                    onChange={(e) => onChange({ ...form, username: e.target.value })}
                  />
                </label>
                <label className="template-field">
                  <span className="template-field-label">密码</span>
                  <input
                    type="password"
                    className="input"
                    autoComplete="new-password"
                    placeholder={isEdit ? "留空则不修改" : ""}
                    value={form.credentials?.password ?? ""}
                    onChange={(e) =>
                      onChange({ ...form, credentials: { ...form.credentials, password: e.target.value } })
                    }
                  />
                </label>
              </div>
              <div className="secret-credentials-grid">
                <label className="template-field">
                  <span className="template-field-label">Token</span>
                  <input type="password" className="input" autoComplete="new-password" placeholder={isEdit ? "留空则不修改" : ""} value={form.credentials?.token ?? ""} onChange={(e) => onChange({ ...form, credentials: { ...form.credentials, token: e.target.value } })} />
                </label>
                <label className="template-field">
                  <span className="template-field-label">API Key</span>
                  <input type="password" className="input" autoComplete="new-password" placeholder={isEdit ? "留空则不修改" : ""} value={form.credentials?.api_key ?? ""} onChange={(e) => onChange({ ...form, credentials: { ...form.credentials, api_key: e.target.value } })} />
                </label>
              </div>
            </>
          ) : (
            <>
              <div className="template-form-grid">
                <label className="template-field">
                  <span className="template-field-label">用户名</span>
                  <input className="input" value={form.username} onChange={(e) => onChange({ ...form, username: e.target.value })} />
                </label>
                <label className="template-field">
                  <span className="template-field-label">密码</span>
                  <input type="password" className="input" autoComplete="new-password" placeholder={isEdit ? "留空则不修改" : ""} value={form.credentials?.password ?? ""} onChange={(e) => onChange({ ...form, credentials: { ...form.credentials, password: e.target.value } })} />
                </label>
              </div>
              <div className="secret-credentials-grid">
                {(["token", "api_key"] as const).map((key) => (
                  <label key={key} className="template-field">
                    <span className="template-field-label">{key === "api_key" ? "API Key" : "Token"}</span>
                    <input type="password" className="input" autoComplete="new-password" placeholder={isEdit ? "留空则不修改" : ""} value={form.credentials?.[key] ?? ""} onChange={(e) => onChange({ ...form, credentials: { ...form.credentials, [key]: e.target.value } })} />
                  </label>
                ))}
              </div>
              <label className="template-field">
                <span className="template-field-label">私钥</span>
                <textarea className="input font-mono text-xs min-h-24" placeholder={isEdit ? "留空则不修改" : ""} value={form.credentials?.private_key ?? ""} onChange={(e) => onChange({ ...form, credentials: { ...form.credentials, private_key: e.target.value } })} />
              </label>
            </>
          )}
        </div>
      </section>

      <section className="secret-form-section">
        <div className="secret-form-section-head">
          <SettingIcon />
          <span>配置选项</span>
        </div>
        <div className="secret-form-section-body">
          <div className="template-form-grid">
            <label className="template-field">
              <span className="template-field-label">状态</span>
              <select className="input" value={form.status} onChange={(e) => onChange({ ...form, status: e.target.value as CreateSecretCommand["status"] })}>
                {SECRET_STATUS_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </label>
            <div className="secret-form-switches">
              <label className="secret-switch">
                <input type="checkbox" checked={form.is_default} onChange={(e) => onChange({ ...form, is_default: e.target.checked })} />
                <span>默认配置</span>
              </label>
              <label className="secret-switch">
                <input type="checkbox" checked={form.is_shared} onChange={(e) => onChange({ ...form, is_shared: e.target.checked })} />
                <span>共享配置</span>
              </label>
            </div>
          </div>
        </div>
      </section>

      <div className="template-form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel}>取消</button>
        <button type="submit" className="btn" disabled={submitting}>{submitting ? "保存中…" : isEdit ? "更新" : "创建"}</button>
      </div>
    </form>
  );
}
