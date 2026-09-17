"use client";

import { useEffect, useId, useMemo, useState } from "react";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import type { EnvironmentVM, ProductProjectVM } from "@sparkops/contracts/product";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import {
  deployableAuthorizedClusters,
  suggestEnvironmentNamespace,
} from "./environment-suggest";

const FORM_ID = "create-environment-form";

export type CreateEnvironmentFormValues = {
  name: string;
  env_type: "test" | "production";
  cluster_id: string;
  namespace: string;
  project_ids: string[];
};

interface CreateEnvironmentModalProps {
  open: boolean;
  busy?: boolean;
  error?: string | null;
  productSlug: string;
  clusters: AuthorizedClusterVM[];
  productProjects: ProductProjectVM[];
  workspaceProjects: ProjectVM[];
  existingEnvironments: EnvironmentVM[];
  onClose: () => void;
  onSubmit: (values: CreateEnvironmentFormValues) => void | Promise<void>;
}

export function CreateEnvironmentModal({
  open,
  busy = false,
  error = null,
  productSlug,
  clusters,
  productProjects,
  workspaceProjects,
  existingEnvironments,
  onClose,
  onSubmit,
}: CreateEnvironmentModalProps) {
  const nameId = useId();
  const typeId = useId();
  const clusterIdField = useId();
  const namespaceId = useId();
  const searchId = useId();

  const [name, setName] = useState("dev");
  const [envType, setEnvType] = useState<"test" | "production">("test");
  const [clusterId, setClusterId] = useState("");
  const [namespace, setNamespace] = useState("");
  const [namespaceTouched, setNamespaceTouched] = useState(false);
  const [selectedProjectIds, setSelectedProjectIds] = useState<string[]>([]);
  const [projectSearch, setProjectSearch] = useState("");

  const deployClusters = useMemo(() => deployableAuthorizedClusters(clusters), [clusters]);
  const clusterNameById = useMemo(
    () => new Map(deployClusters.map((item) => [item.id, item.cluster_name])),
    [deployClusters],
  );

  const projectById = useMemo(
    () => new Map(workspaceProjects.map((item) => [item.id, item])),
    [workspaceProjects],
  );

  const filteredProjects = useMemo(() => {
    const q = projectSearch.trim().toLowerCase();
    return productProjects.filter((row) => {
      if (!q) return true;
      const label = (row.display_name || row.name).toLowerCase();
      return label.includes(q) || row.name.toLowerCase().includes(q);
    });
  }, [productProjects, projectSearch]);

  const namespaceConflict = useMemo(() => {
    const ns = namespace.trim();
    if (!ns || !clusterId) return false;
    return existingEnvironments.some(
      (env) => env.cluster_id === clusterId && env.namespace === ns,
    );
  }, [existingEnvironments, clusterId, namespace]);

  useEffect(() => {
    if (!open) return;
    const preferredCluster =
      deployableAuthorizedClusters(clusters).find((item) =>
        workspaceProjects.some(
          (p) =>
            productProjects.some((b) => b.id === p.id) &&
            p.status === "active" &&
            p.default_cluster_id === item.id,
        ),
      )?.id ??
      deployableAuthorizedClusters(clusters)[0]?.id ??
      "";
    const initialName = "dev";
    setName(initialName);
    setEnvType("test");
    setClusterId(preferredCluster);
    setNamespace(suggestEnvironmentNamespace(productSlug, initialName));
    setNamespaceTouched(false);
    setSelectedProjectIds(productProjects.map((item) => item.id));
    setProjectSearch("");
  }, [open, productSlug, clusters, productProjects, workspaceProjects]);

  function onNameChange(value: string) {
    setName(value);
    if (!namespaceTouched) {
      setNamespace(suggestEnvironmentNamespace(productSlug, value));
    }
  }

  function toggleProject(projectId: string) {
    setSelectedProjectIds((prev) =>
      prev.includes(projectId) ? prev.filter((id) => id !== projectId) : [...prev, projectId],
    );
  }

  function selectAllProjects() {
    setSelectedProjectIds(productProjects.map((item) => item.id));
  }

  function clearProjects() {
    setSelectedProjectIds([]);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || !namespace.trim() || !clusterId.trim()) return;
    await onSubmit({
      name: name.trim(),
      env_type: envType,
      cluster_id: clusterId.trim(),
      namespace: namespace.trim(),
      project_ids: selectedProjectIds,
    });
  }

  const canSubmit = Boolean(name.trim() && namespace.trim() && clusterId) && !busy;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="新建环境"
      description="配置环境基本信息与运行面，并选择纳入本环境的产品项目。"
      size="xl"
      panelClassName="product-env-create-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" disabled={busy} onClick={onClose}>
            取消
          </button>
          <button type="submit" form={FORM_ID} className="btn btn-primary" disabled={!canSubmit}>
            {busy ? "创建中…" : "创建"}
          </button>
        </div>
      }
    >
      <form id={FORM_ID} className="product-env-create-form" onSubmit={(e) => void handleSubmit(e)} noValidate>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        <section className="product-env-form-section">
          <header className="product-env-form-section-head">基本信息</header>
          <div className="product-env-form-section-body product-env-form-grid">
            <div className="form-field">
              <label className="label" htmlFor={nameId}>
                环境名称 <span className="label-required">*</span>
              </label>
              <input
                id={nameId}
                className="input"
                value={name}
                onChange={(e) => onNameChange(e.target.value)}
                placeholder="dev / uat / prod"
                autoComplete="off"
                autoFocus
                disabled={busy}
              />
              <p className="field-hint">用于列表展示与变量渲染，建议使用短小写标识。</p>
            </div>
            <div className="form-field">
              <label className="label" htmlFor={typeId}>
                环境类型
              </label>
              <select
                id={typeId}
                className="input"
                value={envType}
                onChange={(e) => setEnvType(e.target.value as "test" | "production")}
                disabled={busy}
              >
                <option value="test">测试</option>
                <option value="production">生产</option>
              </select>
            </div>
          </div>
        </section>

        <section className="product-env-form-section">
          <header className="product-env-form-section-head">运行面</header>
          <div className="product-env-form-section-body product-env-form-grid">
            <div className="form-field">
              <label className="label" htmlFor={clusterIdField}>
                K8s 集群 <span className="label-required">*</span>
              </label>
              <select
                id={clusterIdField}
                className="input"
                value={clusterId}
                onChange={(e) => setClusterId(e.target.value)}
                disabled={busy || deployClusters.length === 0}
                required
              >
                <option value="">请选择授权集群</option>
                {deployClusters.map((cluster) => (
                  <option key={cluster.id} value={cluster.id}>
                    {cluster.cluster_name}
                  </option>
                ))}
              </select>
              {deployClusters.length === 0 ? (
                <p className="field-hint">当前工作区暂无可用部署集群，请先完成集群授权。</p>
              ) : (
                <p className="field-hint">仅展示工作区已授权且非构建用途的集群。</p>
              )}
            </div>
            <div className="form-field">
              <label className="label" htmlFor={namespaceId}>
                K8s Namespace <span className="label-required">*</span>
              </label>
              <input
                id={namespaceId}
                className="input"
                value={namespace}
                onChange={(e) => {
                  setNamespaceTouched(true);
                  setNamespace(e.target.value);
                }}
                placeholder={suggestEnvironmentNamespace(productSlug || "product", name || "dev")}
                autoComplete="off"
                disabled={busy}
                required
              />
              <p className="field-hint">
                默认规则：产品标识-env-环境名。不同环境建议使用独立 Namespace。
              </p>
              {namespaceConflict ? (
                <InlineAlert tone="warning">
                  当前集群下已存在相同 Namespace 的环境，请更换名称或 Namespace。
                </InlineAlert>
              ) : null}
            </div>
          </div>
        </section>

        <section className="product-env-form-section">
          <header className="product-env-form-section-head">
            <span>纳入项目</span>
            <span className="product-env-form-section-meta">
              已选 {selectedProjectIds.length}/{productProjects.length}
            </span>
          </header>
          <div className="product-env-form-section-body stack-sm">
            {productProjects.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                暂无绑定项目。可先创建环境，之后在「项目配置」中绑定。
              </p>
            ) : (
              <>
                <div className="product-env-project-toolbar">
                  <input
                    id={searchId}
                    className="input product-bind-search"
                    value={projectSearch}
                    onChange={(e) => setProjectSearch(e.target.value)}
                    placeholder="搜索项目名称"
                    disabled={busy}
                    aria-label="搜索纳入项目"
                  />
                  <div className="product-env-project-actions">
                    <button type="button" className="btn-ghost text-sm" disabled={busy} onClick={selectAllProjects}>
                      全选
                    </button>
                    <button type="button" className="btn-ghost text-sm" disabled={busy} onClick={clearProjects}>
                      清空
                    </button>
                  </div>
                </div>
                <ul className="product-bind-list product-env-bind-list">
                  {filteredProjects.length === 0 ? (
                    <li className="empty-state text-sm" role="status">
                      无匹配项目
                    </li>
                  ) : (
                    filteredProjects.map((row) => {
                      const detail = projectById.get(row.id);
                      const clusterLabel =
                        (detail?.default_cluster_id &&
                          clusterNameById.get(detail.default_cluster_id)) ||
                        detail?.default_cluster_id ||
                        "未配置集群";
                      const nsLabel = detail?.namespace?.trim() || "未配置 Namespace";
                      return (
                        <li key={row.id}>
                          <label className="product-bind-item">
                            <input
                              type="checkbox"
                              checked={selectedProjectIds.includes(row.id)}
                              onChange={() => toggleProject(row.id)}
                              disabled={busy}
                            />
                            <span className="product-bind-item-body">
                              <span className="product-bind-item-title">
                                {row.display_name || row.name}
                              </span>
                              <span className="product-bind-item-sub">
                                {row.name} · {clusterLabel}/{nsLabel}
                              </span>
                            </span>
                          </label>
                        </li>
                      );
                    })
                  )}
                </ul>
              </>
            )}
          </div>
        </section>
      </form>
    </Modal>
  );
}
