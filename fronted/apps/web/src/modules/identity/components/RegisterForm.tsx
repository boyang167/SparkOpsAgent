"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { identity } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";

export function RegisterForm() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await identity.register({ email, password, display_name: displayName });
      router.push("/dashboard/workplace");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "注册失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form className="auth-form-fields" onSubmit={onSubmit}>
      {error ? (
        <div className="auth-alert auth-alert-error" role="alert">
          {error}
        </div>
      ) : null}

      <label className="auth-field" htmlFor="display_name">
        <span className="auth-field-label">显示名称</span>
        <input
          id="display_name"
          className="auth-input auth-input-plain"
          placeholder="您的姓名或昵称"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          required
        />
      </label>

      <label className="auth-field" htmlFor="email">
        <span className="auth-field-label">工作邮箱</span>
        <input
          id="email"
          className="auth-input auth-input-plain"
          type="email"
          autoComplete="email"
          placeholder="name@company.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
      </label>

      <label className="auth-field" htmlFor="password">
        <span className="auth-field-label">密码</span>
        <input
          id="password"
          className="auth-input auth-input-plain"
          type="password"
          autoComplete="new-password"
          placeholder="至少 8 位"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          minLength={8}
          required
        />
      </label>

      <button className="auth-submit" type="submit" disabled={loading}>
        {loading ? (
          <>
            <span className="auth-spinner" aria-hidden />
            注册中…
          </>
        ) : (
          "创建账号"
        )}
      </button>
    </form>
  );
}
