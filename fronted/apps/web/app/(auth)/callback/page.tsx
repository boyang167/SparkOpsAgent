"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { identity } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";

export default function AuthCallbackPage() {
  const router = useRouter();
  const params = useSearchParams();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const code = params.get("code");
    const state = params.get("state");
    if (!code || !state) {
      setError("缺少 OAuth 回调参数");
      return;
    }
    void (async () => {
      try {
        await identity.completeOIDCCallback(code, state);
        router.replace("/dashboard");
        router.refresh();
      } catch (err) {
        setError(err instanceof BffError ? err.message : "SSO 登录失败");
      }
    })();
  }, [params, router]);

  return (
    <div className="auth-shell">
      <div className="auth-card text-center">
        {error ? <p className="error">{error}</p> : <p className="text-muted">正在完成 SSO 登录…</p>}
      </div>
    </div>
  );
}
