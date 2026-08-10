"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { identity } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";

export function LoginForm() {
  const router = useRouter();
  const [email, setEmail] = useState("demo@sparkops.local");
  const [password, setPassword] = useState("Demo1234!");
  const [ldapUser, setLdapUser] = useState("");
  const [ldapPassword, setLdapPassword] = useState("");
  const [providers, setProviders] = useState({ oidc_enabled: false, ldap_enabled: false });
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
      router.push("/dashboard");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "登录失败");
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
      router.push("/dashboard");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "LDAP 登录失败");
    } finally {
      setLdapLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      <form className="space-y-4" onSubmit={onSubmit}>
        <div>
          <label className="label" htmlFor="email">
            邮箱
          </label>
          <input
            id="email"
            className="input"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div>
          <label className="label" htmlFor="password">
            密码
          </label>
          <input
            id="password"
            className="input"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        <button className="btn w-full" type="submit" disabled={loading}>
          {loading ? "登录中..." : "登录"}
        </button>
      </form>

      {providers.oidc_enabled || providers.ldap_enabled ? (
        <div className="space-y-4 border-t border-[var(--border)] pt-4">
          <p className="text-sm text-muted text-center">或使用企业身份</p>
          {providers.oidc_enabled ? (
            <button type="button" className="btn-secondary w-full" onClick={() => void onOIDCLogin()}>
              SSO / OIDC 登录
            </button>
          ) : null}
          {providers.ldap_enabled ? (
            <form className="space-y-3" onSubmit={onLDAPLogin}>
              <input
                className="input"
                placeholder="LDAP 用户名"
                value={ldapUser}
                onChange={(e) => setLdapUser(e.target.value)}
                required
              />
              <input
                className="input"
                type="password"
                placeholder="LDAP 密码"
                value={ldapPassword}
                onChange={(e) => setLdapPassword(e.target.value)}
                required
              />
              <button className="btn-secondary w-full" type="submit" disabled={ldapLoading}>
                {ldapLoading ? "LDAP 登录中..." : "LDAP 登录"}
              </button>
            </form>
          ) : null}
        </div>
      ) : null}

      {error ? <p className="error">{error}</p> : null}
    </div>
  );
}
