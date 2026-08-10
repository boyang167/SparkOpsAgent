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
      router.push("/dashboard");
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "注册失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <div>
        <label className="label" htmlFor="display_name">
          显示名称
        </label>
        <input
          id="display_name"
          className="input"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          required
        />
      </div>
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
          minLength={8}
          required
        />
      </div>
      {error ? <p className="error">{error}</p> : null}
      <button className="btn w-full" type="submit" disabled={loading}>
        {loading ? "注册中..." : "注册"}
      </button>
    </form>
  );
}
