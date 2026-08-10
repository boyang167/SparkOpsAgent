"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { ToolVM } from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";

export function ToolsPanel() {
  const [items, setItems] = useState<ToolVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await platform.listTools());
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败（需 Super Admin）");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) return <div className="empty-state">加载中…</div>;

  return (
    <div className="stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {items.map((tool) => (
          <div key={tool.id} className="rounded-lg border border-[var(--border)] p-4">
            <h3 className="font-medium">{tool.name}</h3>
            <p className="text-sm text-muted mt-2">{tool.description ?? "-"}</p>
            {tool.path ? <p className="text-xs font-mono text-muted mt-2">{tool.path}</p> : null}
          </div>
        ))}
      </div>
    </div>
  );
}
