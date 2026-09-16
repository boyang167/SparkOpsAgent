"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { EnvironmentStatusVM, EnvironmentVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductMasterDetail } from "./ProductMasterDetail";

interface EnvironmentsPanelProps {
  workspaceId: string;
  productId: string;
}

export function EnvironmentsPanel({ workspaceId, productId }: EnvironmentsPanelProps) {
  const [items, setItems] = useState<EnvironmentVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [status, setStatus] = useState<EnvironmentStatusVM | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<EnvironmentVM | null>(null);
  const [name, setName] = useState("");
  const [namespace, setNamespace] = useState("");
  const [clusterId, setClusterId] = useState("");
  const [envType, setEnvType] = useState<"test" | "production">("test");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await product.listEnvironments(workspaceId, productId);
      setItems(list);
      setSelectedId((prev) => {
        if (prev && list.some((item) => item.id === prev)) return prev;
        return list[0]?.id ?? null;
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载环境失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadStatus = useCallback(
    async (envId: string) => {
      try {
        setStatus(await product.getEnvironmentStatus(workspaceId, productId, envId));
      } catch (err) {
        setError(err instanceof BffError ? err.message : "加载部署矩阵失败");
        setStatus(null);
      }
    },
    [workspaceId, productId],
  );

  useEffect(() => {
    if (!selectedId) {
      setStatus(null);
      return;
    }
    void loadStatus(selectedId);
  }, [selectedId, loadStatus]);

  const selected = useMemo(() => items.find((item) => item.id === selectedId) ?? null, [items, selectedId]);

  const listItems = useMemo(
    () =>
      items.map((row) => ({
        id: row.id,
        title: row.name,
        subtitle: row.namespace,
        meta: row.env_type === "production" ? "生产" : "测试",
      })),
    [items],
  );

  async function onCreate() {
    if (!name.trim() || !namespace.trim() || !clusterId.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const created = await product.createEnvironment(workspaceId, productId, {
        name: name.trim(),
        namespace: namespace.trim(),
        cluster_id: clusterId.trim(),
        env_type: envType,
      });
      setCreateOpen(false);
      setName("");
      setNamespace("");
      setClusterId("");
      await load();
      setSelectedId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建环境失败");
    } finally {
      setBusy(false);
    }
  }

  async function onDelete() {
    if (!deleteTarget) return;
    setBusy(true);
    try {
      await product.deleteEnvironment(workspaceId, productId, deleteTarget.id);
      if (selectedId === deleteTarget.id) setSelectedId(null);
      setDeleteTarget(null);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      {error ? (
        <div className="product-master-main-inner">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      <ProductMasterDetail
        sidebarTitle="环境列表"
        searchPlaceholder="搜索环境"
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        items={listItems}
        selectedId={selectedId}
        onSelect={setSelectedId}
        onAdd={() => setCreateOpen(true)}
        addAriaLabel="新建环境"
        loading={loading}
        showMain={Boolean(selected)}
        emptyMain={
          <ProductDetailEmptyState
            title="暂无环境，请点击 + 新建环境"
            description="创建 dev / uat / prod 环境后，可在此查看各项目部署版本矩阵。"
            action={
              <button type="button" className="btn btn-primary text-sm" onClick={() => setCreateOpen(true)}>
                新建环境
              </button>
            }
          />
        }
      >
        {selected ? (
          <div className="product-master-main-inner stack-md">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="stack-xs">
                <h2 className="section-title">{selected.name}</h2>
                <p className="text-sm text-muted-foreground">
                  {selected.namespace} · {selected.env_type === "production" ? "生产环境" : "测试环境"}
                </p>
              </div>
              <button type="button" className="btn-ghost text-sm text-red-600" onClick={() => setDeleteTarget(selected)}>
                删除
              </button>
            </div>

            {status ? (
              <div className="stack-sm">
                <h3 className="text-sm font-medium">部署矩阵</h3>
                <div className="table-wrap">
                  <table className="data-table text-sm">
                    <thead>
                      <tr>
                        <th>项目</th>
                        <th>镜像 Tag</th>
                        <th>最近部署</th>
                      </tr>
                    </thead>
                    <tbody>
                      {status.projects.length === 0 ? (
                        <tr>
                          <td colSpan={3} className="text-muted-foreground">
                            暂无部署记录
                          </td>
                        </tr>
                      ) : (
                        status.projects.map((p) => (
                          <tr key={p.project_id}>
                            <td>{p.display_name || p.project_name}</td>
                            <td className="font-mono text-xs">{p.current_image_tag || "-"}</td>
                            <td>{p.last_deployed_at ? new Date(p.last_deployed_at).toLocaleString() : "-"}</td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : (
              <div className="empty-state text-sm">加载部署矩阵…</div>
            )}
          </div>
        ) : null}
      </ProductMasterDetail>

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="新建环境"
        description="为产品创建 dev / uat / prod 等运行环境。"
        footer={
          <>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onCreate()}>
              {busy ? "创建中…" : "创建"}
            </button>
          </>
        }
      >
        <div className="stack-sm text-sm">
          <label className="stack-xs">
            <span className="text-muted-foreground">名称</span>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="uat" />
          </label>
          <label className="stack-xs">
            <span className="text-muted-foreground">类型</span>
            <select className="input" value={envType} onChange={(e) => setEnvType(e.target.value as "test" | "production")}>
              <option value="test">测试</option>
              <option value="production">生产</option>
            </select>
          </label>
          <label className="stack-xs">
            <span className="text-muted-foreground">集群 ID</span>
            <input className="input font-mono text-xs" value={clusterId} onChange={(e) => setClusterId(e.target.value)} />
          </label>
          <label className="stack-xs">
            <span className="text-muted-foreground">Namespace</span>
            <input className="input" value={namespace} onChange={(e) => setNamespace(e.target.value)} placeholder="uat-ns" />
          </label>
        </div>
      </Modal>

      <Modal
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="删除环境"
        description={deleteTarget ? `确定删除环境「${deleteTarget.name}」？此操作不可撤销。` : undefined}
        footer={
          <>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setDeleteTarget(null)}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" disabled={busy} onClick={() => void onDelete()}>
              {busy ? "删除中…" : "确认删除"}
            </button>
          </>
        }
      >
        <p className="text-sm text-muted-foreground">删除后该环境下的部署状态记录将无法恢复。</p>
      </Modal>
    </>
  );
}
