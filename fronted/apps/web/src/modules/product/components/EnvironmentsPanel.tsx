"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { platform, product, workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import type { EnvironmentStatusVM, EnvironmentVM, ProductEnvironmentType, ProductProjectVM } from "@sparkops/contracts/product";
import { environmentRequiresApproval, environmentTypeBadgeClass, environmentTypeLabel } from "@sparkops/contracts/product";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { generateProjectTemplates } from "@/modules/workspace/components/project/project-template-render";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import {
  deployableAuthorizedClusters,
  suggestEnvironmentsFromProjects,
  type EnvironmentSuggestCandidate,
} from "./environment-suggest";
import { CreateEnvironmentModal } from "./CreateEnvironmentModal";
import { EnvironmentResourceView } from "./EnvironmentResourceView";
import { UpdateEnvironmentVariablesModal } from "./UpdateEnvironmentVariablesModal";
import {
  applyResourceToCluster,
  templatesToResources,
  type ProductK8sResource,
} from "./product-k8s-yaml";

interface EnvironmentsPanelProps {
  workspaceId: string;
  productId: string;
}

type ProjectViewMode = "service" | "resource";

function formatEnvTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function EnvironmentsPanel({ workspaceId, productId }: EnvironmentsPanelProps) {
  const [items, setItems] = useState<EnvironmentVM[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [status, setStatus] = useState<EnvironmentStatusVM | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const [projectSearch, setProjectSearch] = useState("");
  const [viewMode, setViewMode] = useState<ProjectViewMode>("service");
  const [moreOpen, setMoreOpen] = useState(false);
  const [variablesOpen, setVariablesOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [generateOpen, setGenerateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<EnvironmentVM | null>(null);
  const [clusters, setClusters] = useState<AuthorizedClusterVM[]>([]);
  const [productProjects, setProductProjects] = useState<ProductProjectVM[]>([]);
  const [workspaceProjects, setWorkspaceProjects] = useState<ProjectVM[]>([]);
  const [candidates, setCandidates] = useState<EnvironmentSuggestCandidate[]>([]);
  const [selectedCandidateKeys, setSelectedCandidateKeys] = useState<string[]>([]);
  const [productSlug, setProductSlug] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [syncingProjectId, setSyncingProjectId] = useState<string | null>(null);

  const deployClusters = useMemo(() => deployableAuthorizedClusters(clusters), [clusters]);
  const clusterNameById = useMemo(
    () => new Map(deployClusters.map((item) => [item.id, item.cluster_name])),
    [deployClusters],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [list, envClusters, boundProjects, productDetail] = await Promise.all([
        product.listEnvironments(workspaceId, productId),
        platform.listAuthorizedWorkspaceClusters(workspaceId).catch(() => [] as AuthorizedClusterVM[]),
        product.listProductProjects(workspaceId, productId).catch(() => [] as ProductProjectVM[]),
        product.getProduct(workspaceId, productId).catch(() => null),
      ]);
      setItems(list);
      setClusters(envClusters);
      setProductProjects(boundProjects);
      if (productDetail?.name) setProductSlug(productDetail.name);
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
      setStatusLoading(true);
      try {
        setStatus(await product.getEnvironmentStatus(workspaceId, productId, envId));
      } catch (err) {
        setError(err instanceof BffError ? err.message : "加载项目状态失败");
        setStatus(null);
      } finally {
        setStatusLoading(false);
      }
    },
    [workspaceId, productId],
  );

  useEffect(() => {
    setProjectSearch("");
    setMoreOpen(false);
    setVariablesOpen(false);
    if (!selectedId) {
      setStatus(null);
      return;
    }
    void loadStatus(selectedId);
  }, [selectedId, loadStatus]);

  const selected = useMemo(() => items.find((item) => item.id === selectedId) ?? null, [items, selectedId]);

  const filteredProjects = useMemo(() => {
    const projects = status?.projects ?? [];
    const q = projectSearch.trim().toLowerCase();
    if (!q) return projects;
    return projects.filter((p) => {
      const title = (p.display_name || p.project_name).toLowerCase();
      return title.includes(q) || p.project_name.toLowerCase().includes(q);
    });
  }, [status, projectSearch]);

  const deployedCount = useMemo(
    () => (status?.projects ?? []).filter((p) => p.enabled && (p.current_image_tag || p.last_deployed_at)).length,
    [status],
  );

  const envVariableCount = Object.keys(selected?.variables ?? {}).length;
  const envRunning = Boolean(selected && selected.status !== "disabled");

  async function ensureWorkspaceProjects() {
    if (workspaceProjects.length > 0) return workspaceProjects;
    const next = await workspace.listProjects(workspaceId);
    setWorkspaceProjects(next);
    return next;
  }

  async function openCreateModal() {
    setError(null);
    setCreateOpen(true);
    try {
      const [envClusters, , bound, productDetail] = await Promise.all([
        clusters.length > 0
          ? Promise.resolve(clusters)
          : platform.listAuthorizedWorkspaceClusters(workspaceId),
        ensureWorkspaceProjects(),
        productProjects.length > 0
          ? Promise.resolve(productProjects)
          : product.listProductProjects(workspaceId, productId),
        productSlug
          ? Promise.resolve(null)
          : product.getProduct(workspaceId, productId).catch(() => null),
      ]);
      if (clusters.length === 0) setClusters(envClusters);
      if (productProjects.length === 0) setProductProjects(bound);
      if (productDetail?.name) setProductSlug(productDetail.name);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载集群/项目失败");
    }
  }

  async function openGenerateModal() {
    setError(null);
    setGenerateOpen(true);
    try {
      const [envClusters, wsProjects, bound] = await Promise.all([
        clusters.length > 0
          ? Promise.resolve(clusters)
          : platform.listAuthorizedWorkspaceClusters(workspaceId),
        ensureWorkspaceProjects(),
        productProjects.length > 0
          ? Promise.resolve(productProjects)
          : product.listProductProjects(workspaceId, productId),
      ]);
      if (clusters.length === 0) setClusters(envClusters);
      if (productProjects.length === 0) setProductProjects(bound);
      const next = suggestEnvironmentsFromProjects(bound, wsProjects, envClusters);
      setCandidates(next);
      const existingKeys = new Set(items.map((item) => `${item.cluster_id}::${item.namespace}`));
      setSelectedCandidateKeys(next.filter((item) => !existingKeys.has(item.key)).map((item) => item.key));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "分析项目环境失败");
      setCandidates([]);
    }
  }

  async function onCreate(values: {
    name: string;
    env_type: ProductEnvironmentType;
    cluster_id: string;
    namespace: string;
    project_ids: string[];
  }) {
    setBusy(true);
    setError(null);
    try {
      const created = await product.createEnvironment(workspaceId, productId, values);
      setCreateOpen(false);
      await load();
      setSelectedId(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建环境失败");
    } finally {
      setBusy(false);
    }
  }

  async function onGenerate() {
    const picked = candidates.filter((item) => selectedCandidateKeys.includes(item.key));
    if (picked.length === 0) return;
    setBusy(true);
    setError(null);
    try {
      const existingKeys = new Set(items.map((item) => `${item.cluster_id}::${item.namespace}`));
      let lastId: string | null = null;
      for (const candidate of picked) {
        if (existingKeys.has(candidate.key)) continue;
        const created = await product.createEnvironment(workspaceId, productId, {
          name: candidate.name,
          namespace: candidate.namespace,
          cluster_id: candidate.cluster_id,
          env_type: candidate.env_type,
          project_ids: candidate.project_ids,
        });
        lastId = created.id;
      }
      setGenerateOpen(false);
      await load();
      if (lastId) setSelectedId(lastId);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "从项目生成环境失败");
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
      setMoreOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setBusy(false);
    }
  }

  async function syncProjectToEnvironment(projectId: string) {
    if (!selected) return;
    setSyncingProjectId(projectId);
    setError(null);
    try {
      const detail = await workspace.getProject(workspaceId, projectId);
      let resources: ProductK8sResource[] = [];
      try {
        const artifacts = await workspace.listGeneratedTemplates(workspaceId, projectId);
        resources = templatesToResources(detail, artifacts);
      } catch {
        resources = templatesToResources(detail, generateProjectTemplates(detail, []));
      }
      const applyList = resources.filter((item) => item.kind === "Deployment" || item.kind === "Service");
      if (applyList.length === 0) {
        throw new Error("该项目暂无 Deployment/Service YAML，请先在项目配置中完成模板");
      }
      for (const resource of applyList) {
        await applyResourceToCluster(resource, selected.cluster_id, selected.namespace);
      }
      await loadStatus(selected.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : err instanceof Error ? err.message : "更新到环境失败");
    } finally {
      setSyncingProjectId(null);
    }
  }

  function toggleCandidate(key: string) {
    setSelectedCandidateKeys((prev) =>
      prev.includes(key) ? prev.filter((id) => id !== key) : [...prev, key],
    );
  }

  if (loading) {
    return (
      <div className="product-env-page">
        <div className="empty-state text-sm" role="status">
          加载环境…
        </div>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <>
        {error && !createOpen && !generateOpen ? (
          <div className="product-env-page-pad">
            <InlineAlert tone="error">{error}</InlineAlert>
          </div>
        ) : null}
        <ProductDetailEmptyState
          title="暂无产品环境"
          description="可从已绑定项目的集群/命名空间生成，或手动新建 uat / prod 运行面。"
          action={
            <div className="flex flex-wrap gap-2 justify-center">
              <button type="button" className="btn btn-primary text-sm" onClick={() => void openGenerateModal()}>
                从项目生成
              </button>
              <button type="button" className="btn btn-secondary text-sm" onClick={() => void openCreateModal()}>
                新建环境
              </button>
            </div>
          }
        />
        <CreateEnvironmentModal
          open={createOpen}
          busy={busy}
          error={createOpen ? error : null}
          productSlug={productSlug}
          clusters={clusters}
          productProjects={productProjects}
          workspaceProjects={workspaceProjects}
          existingEnvironments={items}
          onClose={() => setCreateOpen(false)}
          onSubmit={onCreate}
        />
        <GenerateEnvironmentsModal
          open={generateOpen}
          busy={busy}
          error={generateOpen ? error : null}
          candidates={candidates}
          selectedKeys={selectedCandidateKeys}
          existing={items}
          onClose={() => setGenerateOpen(false)}
          onToggle={toggleCandidate}
          onSubmit={() => void onGenerate()}
        />
      </>
    );
  }

  return (
    <>
      <div className="product-env-page">
        {error && !createOpen && !generateOpen ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        <div className="product-env-tabs" role="tablist" aria-label="产品环境">
          <div className="product-env-tabs-scroll">
            {items.map((env) => {
              const active = env.id === selectedId;
              return (
                <button
                  key={env.id}
                  type="button"
                  role="tab"
                  aria-selected={active}
                  className={`product-env-tab${active ? " is-active" : ""}`}
                  onClick={() => setSelectedId(env.id)}
                >
                  <span className="product-env-tab-name">{env.name}</span>
                  {environmentRequiresApproval(env.env_type) ? (
                    <span className="product-env-tab-tag">{environmentTypeLabel(env.env_type)}</span>
                  ) : null}
                </button>
              );
            })}
          </div>
          <div className="product-env-tabs-actions">
            <button type="button" className="btn-ghost text-sm" onClick={() => void openGenerateModal()}>
              从项目生成
            </button>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => void openCreateModal()}>
              新建环境
            </button>
          </div>
        </div>

        {selected ? (
          <>
            <section className="product-env-summary" aria-label="环境信息">
              <header className="product-env-summary-top">
                <div className="product-env-summary-identity">
                  <h2 className="product-env-summary-name">{selected.name}</h2>
                  <span className={`badge ${environmentTypeBadgeClass(selected.env_type)}`}>
                    {environmentTypeLabel(selected.env_type)}
                  </span>
                  <span className={`product-env-status${envRunning ? " is-running" : " is-stopped"}`}>
                    <span className="product-env-status-dot" aria-hidden />
                    {envRunning ? "正在运行" : "已停用"}
                  </span>
                </div>
                <div className="product-env-ops">
                  <button
                    type="button"
                    className="btn btn-secondary text-sm"
                    onClick={() => setVariablesOpen(true)}
                  >
                    更新全局变量
                  </button>
                  <button
                    type="button"
                    className="btn btn-secondary text-sm"
                    onClick={() => selected && void loadStatus(selected.id)}
                    disabled={statusLoading}
                  >
                    刷新状态
                  </button>
                  <div className="product-env-more">
                    <button
                      type="button"
                      className="btn btn-secondary text-sm"
                      aria-expanded={moreOpen}
                      onClick={() => setMoreOpen((v) => !v)}
                    >
                      更多
                    </button>
                    {moreOpen ? (
                      <div className="product-env-more-menu" role="menu">
                        <button
                          type="button"
                          role="menuitem"
                          className="product-env-more-item is-danger"
                          onClick={() => {
                            setMoreOpen(false);
                            setDeleteTarget(selected);
                          }}
                        >
                          删除环境
                        </button>
                      </div>
                    ) : null}
                  </div>
                </div>
              </header>
              <div className="product-env-summary-grid">
                <dl className="product-env-summary-col">
                  <div className="product-env-kv">
                    <dt>K8s 集群</dt>
                    <dd>{clusterNameById.get(selected.cluster_id) || selected.cluster_id}</dd>
                  </div>
                  <div className="product-env-kv">
                    <dt>K8s 命名空间</dt>
                    <dd className="font-mono">{selected.namespace}</dd>
                  </div>
                  <div className="product-env-kv">
                    <dt>全局变量</dt>
                    <dd>
                      {envVariableCount > 0 ? (
                        <button type="button" className="product-env-var-chip is-set" onClick={() => setVariablesOpen(true)}>
                          {envVariableCount} 项
                        </button>
                      ) : (
                        <button type="button" className="product-env-var-chip" onClick={() => setVariablesOpen(true)}>
                          未配置
                        </button>
                      )}
                    </dd>
                  </div>
                </dl>
                <dl className="product-env-summary-col">
                  <div className="product-env-kv">
                    <dt>更新时间</dt>
                    <dd>{formatEnvTime(selected.created_at)}</dd>
                  </div>
                  <div className="product-env-kv">
                    <dt>纳入项目</dt>
                    <dd>
                      {status
                        ? `${status.projects.filter((p) => p.enabled).length} / ${status.projects.length}`
                        : "—"}
                    </dd>
                  </div>
                </dl>
              </div>
            </section>

            <section className="product-env-services" aria-label="项目列表">
              <div className="product-env-services-bar">
                {viewMode === "service" ? (
                  <div className="product-env-services-left">
                    <label className="product-env-search">
                      <span className="sr-only">搜索项目</span>
                      <input
                        className="input"
                        value={projectSearch}
                        onChange={(e) => setProjectSearch(e.target.value)}
                        placeholder="搜索项目"
                      />
                    </label>
                    <span className="product-env-services-count">
                      项目总数 {(status?.projects ?? []).length} 个
                      {status ? ` · 已部署 ${deployedCount}` : null}
                    </span>
                  </div>
                ) : (
                  <div />
                )}
                <div className="product-env-view-toggle" role="group" aria-label="视图切换">
                  <button
                    type="button"
                    className={viewMode === "service" ? "is-active" : undefined}
                    onClick={() => setViewMode("service")}
                  >
                    项目视图
                  </button>
                  <button
                    type="button"
                    className={viewMode === "resource" ? "is-active" : undefined}
                    onClick={() => setViewMode("resource")}
                  >
                    资源视图
                  </button>
                </div>
              </div>

              {viewMode === "resource" ? (
                selected ? (
                  <EnvironmentResourceView clusterId={selected.cluster_id} namespace={selected.namespace} />
                ) : null
              ) : statusLoading && !status ? (
                <div className="product-env-panel-state" role="status">
                  加载项目状态…
                </div>
              ) : (
                <div className="table-wrap product-env-services-table">
                  <table className="data-table text-sm">
                    <thead>
                      <tr>
                        <th>项目名称</th>
                        <th>
                          状态
                          {status ? ` (${deployedCount}/${status.projects.length})` : ""}
                        </th>
                        <th>镜像</th>
                        <th>外网访问</th>
                        <th>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredProjects.length === 0 ? (
                        <tr>
                          <td colSpan={5}>
                            <div className="product-env-panel-state" role="status">
                              {projectSearch.trim()
                                ? "无匹配项目"
                                : "暂无关联项目，请先在「项目配置」绑定"}
                            </div>
                          </td>
                        </tr>
                      ) : (
                        filteredProjects.map((p) => {
                          const deployed = Boolean(p.current_image_tag || p.last_deployed_at);
                          const title = p.display_name?.trim() || p.project_name;
                          return (
                            <tr key={p.project_id}>
                              <td>
                                <Link href={`/projects/${p.project_id}`} className="product-env-service-name">
                                  <span className="product-env-service-icon" aria-hidden>
                                    ▦
                                  </span>
                                  {title}
                                </Link>
                              </td>
                              <td>
                                {!p.enabled ? (
                                  <span className="badge badge-neutral">未纳入</span>
                                ) : deployed ? (
                                  <span className="badge badge-success">Running</span>
                                ) : (
                                  <span className="badge badge-neutral">未部署</span>
                                )}
                              </td>
                              <td className="font-mono text-xs product-env-image-cell">
                                {p.current_image_tag || "—"}
                              </td>
                              <td className="product-env-muted-cell">—</td>
                              <td>
                                <div className="product-env-row-actions">
                                  <button
                                    type="button"
                                    className="link-primary text-xs"
                                    disabled={Boolean(syncingProjectId) || !p.enabled}
                                    onClick={() => void syncProjectToEnvironment(p.project_id)}
                                  >
                                    {syncingProjectId === p.project_id ? "更新中…" : "更新"}
                                  </button>
                                  <Link href={`/projects/${p.project_id}`} className="link-primary text-xs">
                                    进入项目
                                  </Link>
                                </div>
                              </td>
                            </tr>
                          );
                        })
                      )}
                    </tbody>
                  </table>
                </div>
              )}
            </section>
          </>
        ) : null}
      </div>

      <CreateEnvironmentModal
        open={createOpen}
        busy={busy}
        error={createOpen ? error : null}
        productSlug={productSlug}
        clusters={clusters}
        productProjects={productProjects}
        workspaceProjects={workspaceProjects}
        existingEnvironments={items}
        onClose={() => setCreateOpen(false)}
        onSubmit={onCreate}
      />

      <UpdateEnvironmentVariablesModal
        open={variablesOpen}
        workspaceId={workspaceId}
        productId={productId}
        environment={selected}
        onClose={() => setVariablesOpen(false)}
        onSaved={(variables) => {
          setItems((prev) =>
            prev.map((item) => (item.id === selected?.id ? { ...item, variables } : item)),
          );
          setVariablesOpen(false);
        }}
      />

      <GenerateEnvironmentsModal
        open={generateOpen}
        busy={busy}
        error={generateOpen ? error : null}
        candidates={candidates}
        selectedKeys={selectedCandidateKeys}
        existing={items}
        onClose={() => setGenerateOpen(false)}
        onToggle={toggleCandidate}
        onSubmit={() => void onGenerate()}
      />

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

function GenerateEnvironmentsModal({
  open,
  busy,
  error,
  candidates,
  selectedKeys,
  existing,
  onClose,
  onToggle,
  onSubmit,
}: {
  open: boolean;
  busy: boolean;
  error: string | null;
  candidates: EnvironmentSuggestCandidate[];
  selectedKeys: string[];
  existing: EnvironmentVM[];
  onClose: () => void;
  onToggle: (key: string) => void;
  onSubmit: () => void;
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="从项目生成环境"
      description="按绑定项目的集群与 Namespace 分组，一键创建产品环境并纳入对应项目。"
      size="lg"
      footer={
        <>
          <button type="button" className="btn btn-secondary text-sm" onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn btn-primary text-sm"
            disabled={busy || selectedKeys.length === 0}
            onClick={onSubmit}
          >
            {busy ? "生成中…" : `生成${selectedKeys.length > 0 ? ` (${selectedKeys.length})` : ""}`}
          </button>
        </>
      }
    >
      <div className="stack-sm text-sm">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {candidates.length === 0 ? (
          <div className="empty-state text-sm" role="status">
            未从绑定项目归纳出候选环境。请确认项目已配置默认集群与 Namespace。
          </div>
        ) : (
          <ul className="product-bind-list">
            {candidates.map((item) => {
              const exists = existing.some(
                (env) => env.cluster_id === item.cluster_id && env.namespace === item.namespace,
              );
              return (
                <li key={item.key}>
                  <label className={`product-bind-item${exists ? " is-disabled" : ""}`}>
                    <input
                      type="checkbox"
                      checked={selectedKeys.includes(item.key)}
                      disabled={exists || busy}
                      onChange={() => onToggle(item.key)}
                    />
                    <span className="product-bind-item-body">
                      <span className="product-bind-item-title">
                        {item.name} · {item.cluster_name}/{item.namespace}
                      </span>
                      <span className="product-bind-item-sub">
                        {item.project_labels.join("、")} · {environmentTypeLabel(item.env_type)}
                      </span>
                      {exists ? <span className="product-bind-item-tag">已存在相同集群/Namespace</span> : null}
                    </span>
                  </label>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </Modal>
  );
}
