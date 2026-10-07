"use client";

import { useCallback, useEffect, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { PluginRecordVM } from "@sparkops/contracts/plugin";
import { plugin } from "@sparkops/web-sdk";
import { notifyEntitlementsUpdated } from "@/components/layout/shell-context";
import { InlineAlert } from "@/components/ui/InlineAlert";

export function WorkspacePluginsPanel({ workspaceId }: { workspaceId: string }) {
  const [items, setItems] = useState<PluginRecordVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await plugin.listWorkspacePlugins(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载工作区插件失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function toggle(item: PluginRecordVM) {
    const nextEnabled = !item.enabled;
    setPendingId(item.id);
    setError(null);
    try {
      if (nextEnabled) {
        await plugin.enableWorkspacePlugin(workspaceId, item.id);
      } else {
        await plugin.disableWorkspacePlugin(workspaceId, item.id);
      }
      setItems((current) =>
        current.map((candidate) =>
          candidate.id === item.id ? { ...candidate, enabled: nextEnabled } : candidate,
        ),
      );
      notifyEntitlementsUpdated();
      window.dispatchEvent(new Event("sparkops:menus-updated"));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新插件开关失败");
    } finally {
      setPendingId(null);
    }
  }

  return (
    <div className="workspace-plugin-list">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载插件中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">暂无可启用的已部署插件。</div>
      ) : (
        items.map((item) => (
          <div key={item.id} className="workspace-plugin-row">
            <div className="workspace-plugin-copy">
              <p className="workspace-plugin-name">{item.display_name}</p>
              <p className="workspace-plugin-meta">
                {item.id} · {item.version}
              </p>
            </div>
            <button
              type="button"
              className={item.enabled ? "menu-switch menu-switch-on" : "menu-switch"}
              aria-label={`${item.enabled ? "停用" : "启用"} ${item.display_name}`}
              aria-pressed={item.enabled === true}
              disabled={pendingId === item.id}
              onClick={() => void toggle(item)}
            >
              <span className="menu-switch-thumb" />
            </button>
          </div>
        ))
      )}
    </div>
  );
}
