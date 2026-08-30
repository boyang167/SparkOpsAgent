"use client";

import { useCallback, useEffect, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { PluginRecordVM } from "@sparkops/contracts/plugin";
import { plugin } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";

const STATUS_LABELS: Record<string, string> = {
  uploaded: "已上传",
  pr_open: "PR 已创建",
  merged: "已合并",
  deployed: "已部署",
  pr_failed: "PR 失败",
  ci_failed: "CI 失败",
  uninstalling: "卸载 PR 已创建",
  uninstall_merged: "卸载 PR 已合并",
};

export function PlatformPluginsPanel() {
  const [items, setItems] = useState<PluginRecordVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await plugin.listPlugins());
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载插件失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function upload(file: File) {
    setUploading(true);
    setError(null);
    try {
      await plugin.installPlugin(file);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "上传插件失败");
    } finally {
      setUploading(false);
    }
  }

  async function runAction(
    pluginId: string,
    action: (id: string) => Promise<unknown>,
    fallbackMessage: string,
  ) {
    setPendingId(pluginId);
    setError(null);
    try {
      await action(pluginId);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : fallbackMessage);
    } finally {
      setPendingId(null);
    }
  }

  const uploadAction = (
    <label className="btn platform-add-btn">
      {uploading ? "上传中…" : "上传 ZIP"}
      <input
        className="sr-only"
        type="file"
        accept=".zip,application/zip"
        disabled={uploading}
        onChange={(event) => {
          const file = event.target.files?.[0];
          event.target.value = "";
          if (file) void upload(file);
        }}
      />
    </label>
  );

  return (
    <div className="platform-orgs-page">
      <section className="platform-table-card platform-orgs-card">
        <PlatformPanelHead
          title="插件管理"
          description="上传插件包并推进合并、部署和卸载流程。"
          action={uploadAction}
        />
        <div className="platform-orgs-card-body stack-md">
          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
          {loading ? (
            <div className="platform-table-empty">加载插件中…</div>
          ) : items.length === 0 ? (
            <div className="platform-table-empty">暂无插件，请上传 zip 安装包。</div>
          ) : (
            <div className="platform-table-wrap">
              <table className="platform-table">
                <thead>
                  <tr>
                    <th>插件</th>
                    <th>版本</th>
                    <th>状态</th>
                    <th>PR</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((item) => {
                    const pending = pendingId === item.id;
                    return (
                      <tr key={item.id}>
                        <td>
                          <div className="font-medium">{item.display_name}</div>
                          <div className="text-xs text-muted font-mono">{item.id}</div>
                        </td>
                        <td>{item.version}</td>
                        <td>{STATUS_LABELS[item.status] ?? item.status}</td>
                        <td>
                          {item.pr_url ? (
                            <a href={item.pr_url} target="_blank" rel="noreferrer">
                              查看 PR
                            </a>
                          ) : (
                            "-"
                          )}
                        </td>
                        <td>
                          <div className="flex flex-wrap gap-2">
                            <button
                              type="button"
                              className="btn-secondary"
                              disabled={
                                pending ||
                                (item.status !== "pr_open" && item.status !== "uninstalling")
                              }
                              onClick={() =>
                                void runAction(
                                  item.id,
                                  plugin.markPluginMerged,
                                  "标记合并失败",
                                )
                              }
                            >
                              标记已合并
                            </button>
                            <button
                              type="button"
                              className="btn-secondary"
                              disabled={
                                pending ||
                                (item.status !== "merged" &&
                                  item.status !== "uninstall_merged")
                              }
                              onClick={() =>
                                void runAction(
                                  item.id,
                                  plugin.markPluginDeployed,
                                  "标记部署失败",
                                )
                              }
                            >
                              标记已部署
                            </button>
                            <button
                              type="button"
                              className="btn-secondary"
                              disabled={pending || item.status !== "deployed"}
                              onClick={() =>
                                void runAction(
                                  item.id,
                                  plugin.uninstallPlugin,
                                  "卸载插件失败",
                                )
                              }
                            >
                              卸载
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
