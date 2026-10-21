"use client";

import { useCallback, useEffect, useState } from "react";
import { BffError } from "@sparkops/contracts";
import { plugin } from "@sparkops/web-sdk";
import { useShellContext } from "@/components/layout/shell-context";
import { InlineAlert } from "@/components/ui/InlineAlert";

export default function ExamplePluginPage() {
  const { workspaceId } = useShellContext();
  const [loading, setLoading] = useState(true);
  const [ok, setOk] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const ping = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await plugin.getExamplePing(workspaceId);
      setOk(result.ok);
    } catch (err) {
      setOk(false);
      setError(err instanceof BffError ? err.message : "Example 插件不可用");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void ping();
  }, [ping]);

  return (
    <div className="page stack-lg">
      <section className="card stack-md">
        <div>
          <h1 className="text-xl font-semibold">Example 插件</h1>
          <p className="text-sm text-muted mt-1">
            通过 BFF 调用插件后端的 ping 接口。
          </p>
        </div>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        <div className="flex items-center gap-3">
          <span className={ok ? "status-badge status-success" : "status-badge"}>
            {loading ? "检测中…" : ok ? "Ping 成功" : "Ping 失败"}
          </span>
          <button type="button" className="btn-secondary" disabled={loading} onClick={() => void ping()}>
            重新检测
          </button>
        </div>
      </section>
    </div>
  );
}
