"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { product, workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { EnvironmentVM, ProductProjectVM } from "@sparkops/contracts/product";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { generateProjectTemplates } from "@/modules/workspace/components/project/project-template-render";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ProductDetailEmptyState } from "./ProductDetailEmptyState";
import { ProductMasterDetail } from "./ProductMasterDetail";
import { ProductProjectYamlWorkbench } from "./ProductProjectYamlWorkbench";
import {
  applyResourceToCluster,
  loadClusterResources,
  mergeProjectResources,
  templatesToResources,
  type ProductK8sResource,
} from "./product-k8s-yaml";

interface ProductProjectsPanelProps {
  workspaceId: string;
  productId: string;
  onChanged?: () => void;
}

function projectBindState(project: ProjectVM, productId: string): "available" | "bound" | "other" {
  if (!project.product_id) return "available";
  if (project.product_id === productId) return "bound";
  return "other";
}

async function loadProjectTemplateResources(workspaceId: string, projectId: string): Promise<{
  project: ProjectVM;
  resources: ProductK8sResource[];
  hint?: string;
}> {
  const project = await workspace.getProject(workspaceId, projectId);
  try {
    const artifacts = await workspace.listGeneratedTemplates(workspaceId, projectId);
    return { project, resources: templatesToResources(project, artifacts) };
  } catch (err) {
    try {
      return {
        project,
        resources: templatesToResources(project, generateProjectTemplates(project, [])),
        hint: err instanceof BffError ? `${err.message}（已使用本地模板）` : "后端生成失败，已使用本地模板",
      };
    } catch {
      return {
        project,
        resources: [],
        hint: err instanceof BffError ? err.message : "加载项目模板失败",
      };
    }
  }
}

async function loadProjectClusterResources(project: ProjectVM): Promise<{
  resources: ProductK8sResource[];
  hint?: string;
}> {
  try {
    return { resources: await loadClusterResources(project) };
  } catch (err) {
    return {
      resources: [],
      hint: err instanceof Error ? err.message : "读取集群资源失败，已保留项目模板 YAML",
    };
  }
}

export function ProductProjectsPanel({ workspaceId, productId, onChanged }: ProductProjectsPanelProps) {
  const [projects, setProjects] = useState<ProductProjectVM[]>([]);
  const [resources, setResources] = useState<ProductK8sResource[]>([]);
  const [yamlById, setYamlById] = useState<Record<string, string>>({});
  const [selectedResourceId, setSelectedResourceId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [bindOpen, setBindOpen] = useState(false);
  const [unbindTarget, setUnbindTarget] = useState<ProductProjectVM | null>(null);
  const [workspaceProjects, setWorkspaceProjects] = useState<ProjectVM[]>([]);
  const [bindSearch, setBindSearch] = useState("");
  const [selectedBindIds, setSelectedBindIds] = useState<string[]>([]);
  const [environments, setEnvironments] = useState<EnvironmentVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [hint, setHint] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [clusterLoading, setClusterLoading] = useState(false);
  const [bindLoading, setBindLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setClusterLoading(false);
    setError(null);
    setHint(null);
    try {
      const [items, envItems] = await Promise.all([
        product.listProductProjects(workspaceId, productId),
        product.listEnvironments(workspaceId, productId).catch(() => [] as EnvironmentVM[]),
      ]);
      setProjects(items);
      setEnvironments(envItems);

      // 第一阶段：只拉模板 YAML（后端/本地，通常很快），先解除「加载中」
      const templateLoaded = await Promise.all(
        items.map((row) => loadProjectTemplateResources(workspaceId, row.id)),
      );
      const templateResources = templateLoaded.flatMap((item) => item.resources);
      const templateHints = templateLoaded.map((item) => item.hint).filter(Boolean);
      setResources(templateResources);
      setYamlById(Object.fromEntries(templateResources.map((item) => [item.id, item.yaml])));
      setHint(templateHints[0] ?? null);
      setSelectedResourceId((prev) => {
        if (prev && templateResources.some((item) => item.id === prev)) return prev;
        return templateResources[0]?.id ?? null;
      });
      setLoading(false);

      // 第二阶段：后台补集群实时资源，不阻塞侧栏
      const projectsDetail = templateLoaded.map((item) => item.project);
      if (projectsDetail.some((p) => p.default_cluster_id && p.namespace)) {
        setClusterLoading(true);
        const clusterLoaded = await Promise.all(projectsDetail.map((p) => loadProjectClusterResources(p)));
        const clusterResources = clusterLoaded.flatMap((item) => item.resources);
        const clusterHints = clusterLoaded.map((item) => item.hint).filter(Boolean);
        if (clusterResources.length > 0 || clusterHints.length > 0) {
          setResources((prev) => {
            const byProject = new Map<string, ProductK8sResource[]>();
            for (const item of prev) {
              const list = byProject.get(item.projectId) ?? [];
              list.push(item);
              byProject.set(item.projectId, list);
            }
            for (const project of projectsDetail) {
              const templates = byProject.get(project.id) ?? [];
              const cluster = clusterResources.filter((item) => item.projectId === project.id);
              byProject.set(project.id, mergeProjectResources(cluster, templates));
            }
            return [...byProject.values()].flat();
          });
          setYamlById((prev) => {
            const next = { ...prev };
            for (const item of clusterResources) {
              if (!(item.id in next)) next[item.id] = item.yaml;
            }
            return next;
          });
          if (clusterHints[0]) setHint(clusterHints[0]);
          setSelectedResourceId((prev) => {
            if (prev) return prev;
            return clusterResources[0]?.id ?? null;
          });
        }
        setClusterLoading(false);
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载项目失败");
      setProjects([]);
      setResources([]);
      setLoading(false);
      setClusterLoading(false);
    }
  }, [workspaceId, productId]);

  useEffect(() => {
    void load();
  }, [load]);

  const openBindModal = useCallback(async () => {
    setBindOpen(true);
    setBindSearch("");
    setSelectedBindIds([]);
    setBindLoading(true);
    setError(null);
    try {
      const items = await workspace.listProjects(workspaceId);
      setWorkspaceProjects(items.filter((row) => row.status === "active"));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载工作区项目失败");
      setWorkspaceProjects([]);
    } finally {
      setBindLoading(false);
    }
  }, [workspaceId]);

  const selectedResource = useMemo(
    () => resources.find((item) => item.id === selectedResourceId) ?? null,
    [resources, selectedResourceId],
  );

  const selectedProject = useMemo(
    () => (selectedResource ? projects.find((row) => row.id === selectedResource.projectId) ?? null : null),
    [projects, selectedResource],
  );

  const listItems = useMemo(() => {
    const groups = new Map<string, ProductK8sResource[]>();
    for (const resource of resources) {
      const list = groups.get(resource.projectId) ?? [];
      list.push(resource);
      groups.set(resource.projectId, list);
    }
    return projects.flatMap((project) => {
      const groupItems = groups.get(project.id) ?? [];
      return [
        {
          id: `group:${project.id}`,
          title: project.display_name || project.name,
          isGroup: true,
        },
        ...groupItems.map((item) => ({
          id: item.id,
          title: item.name,
          subtitle: item.kind,
          indent: true,
          meta: item.source === "cluster" ? "集群" : "模板",
        })),
      ];
    });
  }, [projects, resources]);

  const currentYaml = selectedResource ? (yamlById[selectedResource.id] ?? selectedResource.yaml) : "";
  const dirty = Boolean(selectedResource && currentYaml !== selectedResource.yaml);

  function onSelectListItem(id: string) {
    if (id.startsWith("group:")) return;
    setSelectedResourceId(id);
  }

  const availableCount = useMemo(
    () => workspaceProjects.filter((row) => projectBindState(row, productId) === "available").length,
    [workspaceProjects, productId],
  );

  const filteredBindCandidates = useMemo(() => {
    const q = bindSearch.trim().toLowerCase();
    return workspaceProjects.filter((row) => {
      if (projectBindState(row, productId) === "bound") return false;
      if (!q) return true;
      const label = (row.display_name || row.name).toLowerCase();
      return label.includes(q) || row.name.toLowerCase().includes(q);
    });
  }, [workspaceProjects, bindSearch, productId]);

  function toggleBindSelection(projectId: string) {
    setSelectedBindIds((prev) =>
      prev.includes(projectId) ? prev.filter((id) => id !== projectId) : [...prev, projectId],
    );
  }

  async function onBindProjects() {
    if (selectedBindIds.length === 0) return;
    setBusy(true);
    setError(null);
    try {
      for (const projectId of selectedBindIds) {
        const row = workspaceProjects.find((item) => item.id === projectId);
        if (!row || projectBindState(row, productId) !== "available") continue;
        await workspace.updateProject(workspaceId, projectId, {
          product_id: productId,
          name: row.name,
        });
      }
      setBindOpen(false);
      setSelectedBindIds([]);
      await load();
      onChanged?.();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "绑定项目失败");
    } finally {
      setBusy(false);
    }
  }

  function openUnbindForGroup(groupId: string) {
    const projectId = groupId.slice("group:".length);
    setError(null);
    setUnbindTarget(projects.find((row) => row.id === projectId) ?? null);
  }

  async function onUnbindProject() {
    if (!unbindTarget) return;
    setBusy(true);
    setError(null);
    try {
      await workspace.updateProject(workspaceId, unbindTarget.id, {
        product_id: null,
        name: unbindTarget.name,
      });
      setUnbindTarget(null);
      setSelectedResourceId(null);
      await load();
      onChanged?.();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "解绑项目失败");
    } finally {
      setBusy(false);
    }
  }

  async function onSaveYaml() {
    if (!selectedResource) return;
    setSaving(true);
    setError(null);
    try {
      if (selectedResource.source === "template" && selectedResource.templateKind) {
        await workspace.updateGeneratedTemplate(workspaceId, selectedResource.projectId, selectedResource.templateKind, {
          content: currentYaml,
        });
      } else if (selectedResource.source === "cluster" && selectedResource.clusterId && selectedResource.namespace) {
        await applyResourceToCluster({ ...selectedResource, yaml: currentYaml }, selectedResource.clusterId, selectedResource.namespace);
      } else {
        throw new Error("当前资源无法保存");
      }
      setResources((prev) =>
        prev.map((item) => (item.id === selectedResource.id ? { ...item, yaml: currentYaml } : item)),
      );
    } catch (err) {
      setError(err instanceof BffError ? err.message : err instanceof Error ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      {error && !bindOpen && !unbindTarget ? (
        <div className="product-master-main-inner">
          <InlineAlert tone="error">{error}</InlineAlert>
        </div>
      ) : null}

      <ProductMasterDetail
        sidebarTitle="资源配置"
        searchPlaceholder="搜索项目或资源"
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        items={listItems}
        selectedId={selectedResourceId}
        onSelect={onSelectListItem}
        onAdd={() => void openBindModal()}
        addAriaLabel="绑定项目"
        groupActionLabel="解绑"
        onGroupAction={openUnbindForGroup}
        loading={loading}
        showMain={projects.length > 0}
        emptyMain={
          <ProductDetailEmptyState
            title="暂无项目，请绑定工作区项目"
            description="绑定后将自动加载项目模板与集群中的 Kubernetes YAML。"
            action={
              <button type="button" className="btn btn-primary text-sm" onClick={() => void openBindModal()}>
                绑定项目
              </button>
            }
          />
        }
      >
        {projects.length > 0 && resources.length === 0 && !loading ? (
          <ProductDetailEmptyState
            title="尚未找到 Kubernetes YAML"
            description="请确认项目已配置集群/命名空间，或已选择 Service、Deployment 模板。"
          />
        ) : (
          <ProductProjectYamlWorkbench
            resource={selectedResource}
            yaml={currentYaml}
            onYamlChange={(value) => {
              if (!selectedResource) return;
              setYamlById((prev) => ({ ...prev, [selectedResource.id]: value }));
            }}
            environments={environments}
            saving={saving}
            dirty={dirty}
            error={null}
            hint={clusterLoading ? "正在后台同步集群资源…" : hint}
            onSave={() => void onSaveYaml()}
            onUnbind={selectedProject ? () => setUnbindTarget(selectedProject) : undefined}
          />
        )}
      </ProductMasterDetail>

      <Modal
        open={bindOpen}
        onClose={() => setBindOpen(false)}
        title="绑定项目"
        description={`选择工作区内的项目关联到当前产品。可绑定 ${availableCount} 个独立项目。绑定后自动加载 Kubernetes YAML。`}
        size="lg"
        footer={
          <>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setBindOpen(false)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={busy || selectedBindIds.length === 0}
              onClick={() => void onBindProjects()}
            >
              {busy ? "绑定中…" : `绑定${selectedBindIds.length > 0 ? ` (${selectedBindIds.length})` : ""}`}
            </button>
          </>
        }
      >
        <div className="stack-sm">
          {error && bindOpen ? <InlineAlert tone="error">{error}</InlineAlert> : null}

          <label className="product-bind-search">
            <input
              className="input text-sm"
              value={bindSearch}
              onChange={(e) => setBindSearch(e.target.value)}
              placeholder="搜索项目名称或标识"
              aria-label="搜索项目名称或标识"
            />
          </label>

          {bindLoading ? (
            <div className="empty-state text-sm" role="status">
              加载工作区项目…
            </div>
          ) : filteredBindCandidates.length === 0 ? (
            <div className="empty-state text-sm" role="status">
              {availableCount === 0
                ? "工作区内暂无可绑定的独立项目，请先在项目管理中创建项目。"
                : "没有匹配的项目，请调整搜索条件。"}
            </div>
          ) : (
            <ul className="product-bind-list" role="listbox" aria-label="可绑定项目">
              {filteredBindCandidates.map((row) => {
                const state = projectBindState(row, productId);
                const disabled = state === "other";
                const checked = selectedBindIds.includes(row.id);
                return (
                  <li key={row.id}>
                    <label className={`product-bind-item${disabled ? " is-disabled" : ""}`}>
                      <input
                        type="checkbox"
                        checked={checked}
                        disabled={disabled || busy}
                        onChange={() => toggleBindSelection(row.id)}
                      />
                      <span className="product-bind-item-body">
                        <span className="product-bind-item-title">{row.display_name || row.name}</span>
                        <span className="product-bind-item-sub">{row.name}</span>
                        {disabled ? <span className="product-bind-item-tag">已属于其他产品</span> : null}
                      </span>
                    </label>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </Modal>

      <Modal
        open={unbindTarget !== null}
        onClose={() => {
          if (!busy) setUnbindTarget(null);
        }}
        title="解绑项目"
        description={
          unbindTarget
            ? `确定将「${unbindTarget.display_name || unbindTarget.name}」从当前产品解绑？解绑后项目恢复为独立项目。`
            : undefined
        }
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" disabled={busy} onClick={() => setUnbindTarget(null)}>
              取消
            </button>
            <button type="button" className="btn-danger-solid" disabled={busy} onClick={() => void onUnbindProject()}>
              {busy ? "解绑中…" : "确认解绑"}
            </button>
          </div>
        }
      >
        <div className="stack-sm">
          {error && unbindTarget ? <InlineAlert tone="error">{error}</InlineAlert> : null}
          <p className="text-sm text-muted">
            解绑不会删除项目，也不会影响集群中运行的服务，仅取消与当前产品的关联。解绑后该项目不再出现在产品的环境、版本与工作流中。
          </p>
        </div>
      </Modal>
    </>
  );
}
