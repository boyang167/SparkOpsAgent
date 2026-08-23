"use client";

import type { AuthProvidersVM } from "@sparkops/contracts/identity";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { identity } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";

function FieldIcon({ children }: { children: React.ReactNode }) {
  return <span className="auth-field-icon">{children}</span>;
}

export function LoginForm() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [ldapUser, setLdapUser] = useState("");
  const [ldapPassword, setLdapPassword] = useState("");
  const [providers, setProviders] = useState<AuthProvidersVM>({
    oidc_enabled: false,
    ldap_enabled: false,
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [ldapLoading, setLdapLoading] = useState(false);

  useEffect(() => {
    void identity.getAuthProviders().then(setProviders).catch(() => {});
  }, []);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await identity.login({ email, password });
      router.push("/dashboard/workplace");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "登录失败，请检查邮箱与密码");
    } finally {
      setLoading(false);
    }
  }

  async function onOIDCLogin() {
    setError(null);
    try {
      const { authorization_url } = await identity.startOIDCLogin();
      window.location.href = authorization_url;
    } catch (err) {
      setError(err instanceof BffError ? err.message : "SSO 启动失败");
    }
  }

  async function onLDAPLogin(e: React.FormEvent) {
    e.preventDefault();
    setLdapLoading(true);
    setError(null);
    try {
      await identity.ldapLogin({ username: ldapUser, password: ldapPassword });
      router.push("/dashboard/workplace");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "LDAP 登录失败");
    } finally {
      setLdapLoading(false);
    }
  }

  const showPassword = providers.sso_ui?.show_password_login !== false;
  const showEnterprise =
    providers.sso_ui?.enabled !== false &&
    (providers.oidc_enabled || providers.ldap_enabled);

  return (
    <div className="auth-form stack-md">
      {error ? (
        <div className="auth-alert auth-alert-error" role="alert">
          {error}
        </div>
      ) : null}

      {showPassword ? (
        <form className="auth-form-fields" onSubmit={onSubmit}>
          <label className="auth-field" htmlFor="email">
            <span className="auth-field-label">工作邮箱</span>
            <div className="auth-field-control">
              <FieldIcon>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                  <path d="M4 6h16v12H4z" strokeLinejoin="round" />
                  <path d="M4 7l8 6 8-6" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </FieldIcon>
              <input
                id="email"
                className="auth-input"
                type="email"
                autoComplete="email"
                placeholder="name@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
          </label>

          <label className="auth-field" htmlFor="password">
            <span className="auth-field-label">密码</span>
            <div className="auth-field-control">
              <FieldIcon>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                  <rect x="5" y="11" width="14" height="10" rx="2" />
                  <path d="M8 11V8a4 4 0 118 0v3" strokeLinecap="round" />
                </svg>
              </FieldIcon>
              <input
                id="password"
                className="auth-input"
                type="password"
                autoComplete="current-password"
                placeholder="请输入密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
          </label>

          <button className="auth-submit" type="submit" disabled={loading}>
            {loading ? (
              <>
                <span className="auth-spinner" aria-hidden />
                登录中…
              </>
            ) : (
              "进入控制台"
            )}
          </button>
        </form>
      ) : null}

      {showEnterprise ? (
        <div className="auth-sso-block">
          {showPassword ? (
            <div className="auth-divider">
              <span>或使用企业身份</span>
            </div>
          ) : null}

          {providers.oidc_enabled &&
          (providers.sso_ui?.provider !== "ldap" || !providers.ldap_enabled) ? (
            <button type="button" className="auth-sso-btn" onClick={() => void onOIDCLogin()}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
                <path d="M12 3l7 4v5c0 4.4-3 8.5-7 9-4-.5-7-4.6-7-9V7l7-4z" strokeLinejoin="round" />
              </svg>
              SSO / OIDC 登录
            </button>
          ) : null}

          {providers.ldap_enabled &&
          (providers.sso_ui?.provider === "ldap" || !providers.oidc_enabled) ? (
            <form className="auth-form-fields auth-ldap-form" onSubmit={onLDAPLogin}>
              <label className="auth-field">
                <span className="auth-field-label">LDAP 用户名</span>
                <input
                  className="auth-input auth-input-plain"
                  placeholder="域账号"
                  value={ldapUser}
                  onChange={(e) => setLdapUser(e.target.value)}
                  required
                />
              </label>
              <label className="auth-field">
                <span className="auth-field-label">LDAP 密码</span>
                <input
                  className="auth-input auth-input-plain"
                  type="password"
                  placeholder="密码"
                  value={ldapPassword}
                  onChange={(e) => setLdapPassword(e.target.value)}
                  required
                />
              </label>
              <button className="auth-sso-btn" type="submit" disabled={ldapLoading}>
                {ldapLoading ? "LDAP 登录中…" : "LDAP 登录"}
              </button>
            </form>
          ) : null}
        </div>
      ) : null}

      <p className="auth-form-note">
        登录即表示您同意平台安全策略与会话管理规范
      </p>
    </div>
  );
}
