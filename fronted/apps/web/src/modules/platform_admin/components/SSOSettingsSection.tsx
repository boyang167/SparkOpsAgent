"use client";

import { useCallback, useEffect, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { SSOProvider } from "@sparkops/contracts/identity";
import { identity } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import {
  canEnableProvider,
  createSSODraft,
  type SSOConfigDraft,
  toUpdateCommand,
  updateLDAPDraft,
  updateOIDCDraft,
} from "./sso-config-state";

type ProviderTest = SSOProvider | null;

function SettingsSwitch({
  label,
  description,
  checked,
  disabled,
  onChange,
}: {
  label: string;
  description?: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className="platform-settings-switch-row">
      <div className="platform-settings-switch-copy">
        <div className="platform-settings-switch-label">{label}</div>
        {description ? (
          <div className="platform-settings-switch-desc">{description}</div>
        ) : null}
      </div>
      <button
        type="button"
        className={checked ? "menu-switch menu-switch-on" : "menu-switch"}
        aria-label={label}
        aria-pressed={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
      >
        <span className="menu-switch-thumb" />
      </button>
    </div>
  );
}

function ProviderStatus({
  enabled,
  tested,
}: {
  enabled: boolean;
  tested: boolean;
}) {
  const label = enabled ? "已启用" : tested ? "已测试" : "待测试";
  return (
    <span
      className={
        enabled
          ? "platform-settings-badge platform-settings-badge-on"
          : "platform-settings-badge"
      }
    >
      {label}
    </span>
  );
}

export function SSOSettingsSection({
  platformStyle: _platformStyle = false,
}: {
  platformStyle?: boolean;
}) {
  const [draft, setDraft] = useState<SSOConfigDraft | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<ProviderTest>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [versionConflict, setVersionConflict] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setMessage(null);
    setVersionConflict(false);
    try {
      setDraft(createSSODraft(await identity.getSSOAdminConfig()));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载 SSO 配置失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function testProvider(provider: SSOProvider) {
    if (!draft) return;
    setTesting(provider);
    setError(null);
    setMessage(null);
    try {
      const result = await identity.testSSOProvider({
        provider,
        config: toUpdateCommand(draft),
      });
      setDraft((current) => {
        if (!current) return current;
        return provider === "oidc"
          ? updateOIDCDraft(current, {
              testedFingerprint: result.fingerprint,
              testedAt: result.tested_at,
            })
          : updateLDAPDraft(current, {
              testedFingerprint: result.fingerprint,
              testedAt: result.tested_at,
            });
      });
      setMessage(`${provider === "oidc" ? "OIDC" : "LDAP"} 连接测试成功，可以启用。`);
    } catch (err) {
      setError(
        err instanceof BffError
          ? err.message
          : `${provider === "oidc" ? "OIDC" : "LDAP"} 连接测试失败`,
      );
    } finally {
      setTesting(null);
    }
  }

  async function save() {
    if (!draft) return;
    setSaving(true);
    setError(null);
    setMessage(null);
    setVersionConflict(false);
    try {
      const saved = await identity.updateSSOAdminConfig(toUpdateCommand(draft));
      setDraft(createSSODraft(saved));
      setMessage("SSO 配置已保存并立即生效。");
    } catch (err) {
      if (err instanceof BffError && err.status === 409) {
        setVersionConflict(true);
        setError("配置已被其他管理员更新，请重新加载后再保存。");
      } else {
        setError(err instanceof BffError ? err.message : "保存 SSO 配置失败");
      }
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="platform-sso-loading" role="status">
        加载 SSO 配置…
      </div>
    );
  }

  if (!draft) {
    return (
      <div className="stack-sm">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <button type="button" className="btn btn-secondary" onClick={() => void load()}>
          重新加载
        </button>
      </div>
    );
  }

  const busy = saving || testing !== null;
  const oidcCanEnable = canEnableProvider(draft, "oidc");
  const ldapCanEnable = canEnableProvider(draft, "ldap");

  return (
    <div className="platform-sso stack-lg">
      <p className="platform-settings-section-desc">
        配置企业身份源和登录页策略。连接测试通过后方可启用，保存后无需重启即可生效。
      </p>

      {error ? (
        <div className="platform-settings-alerts">
          <InlineAlert tone="error">
            <span>{error}</span>
            {versionConflict ? (
              <>
                {" "}
                <button
                  type="button"
                  className="platform-sso-alert-action"
                  onClick={() => void load()}
                >
                  重新加载
                </button>
              </>
            ) : null}
          </InlineAlert>
        </div>
      ) : null}
      {message ? (
        <div className="platform-settings-alerts">
          <InlineAlert tone="success">{message}</InlineAlert>
        </div>
      ) : null}
      {testing ? (
        <div className="platform-sso-progress" role="status">
          正在测试 {testing === "oidc" ? "OIDC" : "LDAP"} 连接…
        </div>
      ) : null}

      <section className="platform-sso-section stack-md">
        <div>
          <h2 className="platform-sso-title">登录策略</h2>
          <p className="platform-sso-description">决定登录页展示方式和默认企业身份源。</p>
        </div>
        <div className="platform-sso-policy-grid">
          <SettingsSwitch
            label="登录页展示企业 SSO"
            description="关闭后不展示 OIDC 或 LDAP 登录入口。"
            checked={draft.loginUI.enabled}
            disabled={
              busy ||
              (!draft.loginUI.enabled &&
                !(draft.loginUI.provider === "oidc"
                  ? draft.oidc.enabled
                  : draft.ldap.enabled))
            }
            onChange={(enabled) =>
              setDraft((current) =>
                current
                  ? { ...current, loginUI: { ...current.loginUI, enabled } }
                  : current,
              )
            }
          />
          <label className="platform-settings-field">
            <span className="platform-settings-field-label">默认 SSO 提供商</span>
            <select
              className="input platform-settings-field-input"
              value={draft.loginUI.provider}
              disabled={busy}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  loginUI: {
                    ...draft.loginUI,
                    provider: event.target.value as SSOProvider,
                  },
                })
              }
            >
              <option value="oidc">OIDC</option>
              <option value="ldap">LDAP</option>
            </select>
          </label>
          <SettingsSwitch
            label="允许邮箱密码登录"
            description="建议完成 SSO 验证前保持开启，避免管理员被锁定。"
            checked={draft.loginUI.showPasswordLogin}
            disabled={busy}
            onChange={(showPasswordLogin) =>
              setDraft({
                ...draft,
                loginUI: { ...draft.loginUI, showPasswordLogin },
              })
            }
          />
        </div>
      </section>

      <div className="platform-sso-provider-grid">
        <section className="platform-sso-section stack-md">
          <div className="platform-sso-provider-head">
            <div>
              <h2 className="platform-sso-title">OIDC</h2>
              <p className="platform-sso-description">通过 Discovery 对接企业身份提供商。</p>
            </div>
            <ProviderStatus
              enabled={draft.oidc.enabled}
              tested={oidcCanEnable}
            />
          </div>

          <SettingsSwitch
            label="启用 OIDC"
            description={
              !oidcCanEnable && !draft.oidc.enabled
                ? "请先完成连接测试"
                : undefined
            }
            checked={draft.oidc.enabled}
            disabled={busy || (!draft.oidc.enabled && !oidcCanEnable)}
            onChange={(enabled) =>
              setDraft((current) =>
                current ? updateOIDCDraft(current, { enabled }) : current,
              )
            }
          />

          <div className="platform-sso-form-grid">
            <label className="platform-settings-field platform-sso-span-2">
              <span className="platform-settings-field-label">Issuer URL</span>
              <input
                className="input"
                type="url"
                value={draft.oidc.issuer}
                disabled={busy}
                placeholder="https://idp.example.com"
                onChange={(event) =>
                  setDraft(updateOIDCDraft(draft, { issuer: event.target.value }))
                }
              />
            </label>
            <label className="platform-settings-field">
              <span className="platform-settings-field-label">Client ID</span>
              <input
                className="input"
                value={draft.oidc.clientId}
                disabled={busy}
                onChange={(event) =>
                  setDraft(updateOIDCDraft(draft, { clientId: event.target.value }))
                }
              />
            </label>
            <label className="platform-settings-field">
              <span className="platform-settings-field-label">Client Secret</span>
              <input
                className="input"
                type="password"
                autoComplete="new-password"
                value={draft.oidc.clientSecret}
                disabled={busy || draft.oidc.clearClientSecret}
                placeholder={draft.oidc.hasClientSecret ? "已配置" : "输入密钥"}
                onChange={(event) =>
                  setDraft(
                    updateOIDCDraft(draft, {
                      clientSecret: event.target.value,
                      clearClientSecret: false,
                    }),
                  )
                }
              />
              {draft.oidc.hasClientSecret ? (
                <span className="platform-sso-secret-hint">已配置；留空将保留</span>
              ) : null}
            </label>
            <label className="platform-settings-field platform-sso-span-2">
              <span className="platform-settings-field-label">Redirect URL</span>
              <input
                className="input"
                type="url"
                value={draft.oidc.redirectUrl}
                disabled={busy}
                onChange={(event) =>
                  setDraft(updateOIDCDraft(draft, { redirectUrl: event.target.value }))
                }
              />
            </label>
            <label className="platform-settings-field platform-sso-span-2">
              <span className="platform-settings-field-label">Scopes</span>
              <input
                className="input"
                value={draft.oidc.scopes}
                disabled={busy}
                placeholder="openid email profile"
                onChange={(event) =>
                  setDraft(updateOIDCDraft(draft, { scopes: event.target.value }))
                }
              />
            </label>
          </div>

          <details className="platform-sso-advanced">
            <summary>高级端点覆盖</summary>
            <div className="platform-sso-form-grid">
              {(
                [
                  ["Authorization Endpoint", "authorizationEndpoint"],
                  ["Token Endpoint", "tokenEndpoint"],
                  ["UserInfo Endpoint", "userInfoEndpoint"],
                ] as const
              ).map(([label, key]) => (
                <label
                  key={key}
                  className="platform-settings-field platform-sso-span-2"
                >
                  <span className="platform-settings-field-label">{label}</span>
                  <input
                    className="input"
                    type="url"
                    value={draft.oidc[key]}
                    disabled={busy}
                    placeholder="留空使用 Discovery"
                    onChange={(event) =>
                      setDraft(updateOIDCDraft(draft, { [key]: event.target.value }))
                    }
                  />
                </label>
              ))}
            </div>
          </details>

          {!draft.oidc.testedFingerprint &&
          (draft.oidc.testedAt || draft.oidc.enabled) ? (
            <p className="platform-sso-retest">配置已变化，请重新测试。</p>
          ) : null}
          <div className="platform-sso-provider-actions">
            {draft.oidc.hasClientSecret ? (
              <button
                type="button"
                className="btn btn-ghost"
                disabled={busy}
                onClick={() =>
                  setDraft(
                    updateOIDCDraft(draft, {
                      clearClientSecret: !draft.oidc.clearClientSecret,
                      clientSecret: "",
                    }),
                  )
                }
              >
                {draft.oidc.clearClientSecret ? "取消清除密钥" : "清除已存密钥"}
              </button>
            ) : <span />}
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy}
              onClick={() => void testProvider("oidc")}
            >
              {testing === "oidc" ? "测试中…" : "测试连接"}
            </button>
          </div>
        </section>

        <section className="platform-sso-section stack-md">
          <div className="platform-sso-provider-head">
            <div>
              <h2 className="platform-sso-title">LDAP</h2>
              <p className="platform-sso-description">使用服务账号绑定并搜索企业目录。</p>
            </div>
            <ProviderStatus
              enabled={draft.ldap.enabled}
              tested={ldapCanEnable}
            />
          </div>

          <SettingsSwitch
            label="启用 LDAP"
            description={
              !ldapCanEnable && !draft.ldap.enabled
                ? "请先完成连接测试"
                : undefined
            }
            checked={draft.ldap.enabled}
            disabled={busy || (!draft.ldap.enabled && !ldapCanEnable)}
            onChange={(enabled) =>
              setDraft((current) =>
                current ? updateLDAPDraft(current, { enabled }) : current,
              )
            }
          />

          <div className="platform-sso-form-grid">
            <label className="platform-settings-field platform-sso-span-2">
              <span className="platform-settings-field-label">LDAP URL</span>
              <input
                className="input"
                value={draft.ldap.url}
                disabled={busy}
                placeholder="ldaps://ldap.example.com:636"
                onChange={(event) =>
                  setDraft(updateLDAPDraft(draft, { url: event.target.value }))
                }
              />
            </label>
            <label className="platform-settings-field">
              <span className="platform-settings-field-label">Bind DN</span>
              <input
                className="input"
                value={draft.ldap.bindDn}
                disabled={busy}
                onChange={(event) =>
                  setDraft(updateLDAPDraft(draft, { bindDn: event.target.value }))
                }
              />
            </label>
            <label className="platform-settings-field">
              <span className="platform-settings-field-label">Bind Password</span>
              <input
                className="input"
                type="password"
                autoComplete="new-password"
                value={draft.ldap.bindPassword}
                disabled={busy || draft.ldap.clearBindPassword}
                placeholder={draft.ldap.hasBindPassword ? "已配置" : "输入密码"}
                onChange={(event) =>
                  setDraft(
                    updateLDAPDraft(draft, {
                      bindPassword: event.target.value,
                      clearBindPassword: false,
                    }),
                  )
                }
              />
              {draft.ldap.hasBindPassword ? (
                <span className="platform-sso-secret-hint">已配置；留空将保留</span>
              ) : null}
            </label>
            <label className="platform-settings-field platform-sso-span-2">
              <span className="platform-settings-field-label">User Search Base</span>
              <input
                className="input"
                value={draft.ldap.userSearchBase}
                disabled={busy}
                onChange={(event) =>
                  setDraft(
                    updateLDAPDraft(draft, { userSearchBase: event.target.value }),
                  )
                }
              />
            </label>
            <label className="platform-settings-field platform-sso-span-2">
              <span className="platform-settings-field-label">User Search Filter</span>
              <input
                className="input"
                value={draft.ldap.userSearchFilter}
                disabled={busy}
                placeholder="(uid=%s)"
                onChange={(event) =>
                  setDraft(
                    updateLDAPDraft(draft, { userSearchFilter: event.target.value }),
                  )
                }
              />
            </label>
            <label className="platform-settings-field">
              <span className="platform-settings-field-label">Email Attribute</span>
              <input
                className="input"
                value={draft.ldap.emailAttribute}
                disabled={busy}
                onChange={(event) =>
                  setDraft(
                    updateLDAPDraft(draft, { emailAttribute: event.target.value }),
                  )
                }
              />
            </label>
            <label className="platform-settings-field">
              <span className="platform-settings-field-label">Name Attribute</span>
              <input
                className="input"
                value={draft.ldap.nameAttribute}
                disabled={busy}
                onChange={(event) =>
                  setDraft(
                    updateLDAPDraft(draft, { nameAttribute: event.target.value }),
                  )
                }
              />
            </label>
          </div>

          {!draft.ldap.testedFingerprint &&
          (draft.ldap.testedAt || draft.ldap.enabled) ? (
            <p className="platform-sso-retest">配置已变化，请重新测试。</p>
          ) : null}
          <div className="platform-sso-provider-actions">
            {draft.ldap.hasBindPassword ? (
              <button
                type="button"
                className="btn btn-ghost"
                disabled={busy}
                onClick={() =>
                  setDraft(
                    updateLDAPDraft(draft, {
                      clearBindPassword: !draft.ldap.clearBindPassword,
                      bindPassword: "",
                    }),
                  )
                }
              >
                {draft.ldap.clearBindPassword ? "取消清除密码" : "清除已存密码"}
              </button>
            ) : <span />}
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy}
              onClick={() => void testProvider("ldap")}
            >
              {testing === "ldap" ? "测试中…" : "测试连接"}
            </button>
          </div>
        </section>
      </div>

      <div className="platform-sso-save">
        <button
          type="button"
          className="btn btn-primary"
          disabled={busy}
          onClick={() => void save()}
        >
          {saving ? "保存中…" : "保存 SSO 配置"}
        </button>
      </div>
    </div>
  );
}
