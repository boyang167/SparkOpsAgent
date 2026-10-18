"use client";

import { useEffect, useMemo, useState } from "react";
import type { EnvironmentStatusVM, EnvironmentVM, ProductProjectVM, WorkflowJobVM } from "@sparkops/contracts/product";
import { BffError } from "@sparkops/contracts";
import { product } from "@sparkops/web-sdk";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";
import {
  environmentSyncJobSpecToRecord,
  environmentSyncProjectToApiRecord,
  parseEnvironmentSyncJobSpec,
  type EnvironmentSyncJobSpec,
  type EnvironmentSyncProjectSpec,
} from "./environment-sync-job-spec";
import {
  EnvironmentSyncResourceEditorModal,
  type EnvironmentSyncSourceResourceVM,
} from "./EnvironmentSyncResourceEditorModal";
import {
  isEnvironmentSyncJobValid,
  isEnvironmentSyncProjectContentValid,
  normalizeEnvironmentSyncProjects,
  validateEnvironmentSyncJob,
} from "./environment-sync-job-validation";
import {
  DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS,
  ENVIRONMENT_SYNC_RESOURCE_KINDS,
  toggleSyncResourceKind,
  type EnvironmentSyncResourceKind,
} from "./environment-sync-resource-kinds";
import {
  SYNC_OPTION_IMAGE_HINT,
  SYNC_OPTION_IMAGE_LABEL,
  SYNC_OPTION_YAML_HINT,
  SYNC_OPTION_YAML_LABEL,
  buildProjectSyncContentLines,
} from "./environment-sync-content-display";

interface EnvironmentSyncJobConfigDrawerProps {
  open: boolean;
  workspaceId: string;
  productId: string;
  job: WorkflowJobVM;
  environments: EnvironmentVM[];
  projects: ProductProjectVM[];
  onClose: () => void;
  onConfirm: (job: WorkflowJobVM) => void;
}

function defaultProjectsFromStatus(status: EnvironmentStatusVM | null): EnvironmentSyncProjectSpec[] {
  return (status?.projects ?? [])
    .filter((item) => item.enabled)
    .map((item) => ({
      projectId: item.project_id,
      syncImage: true,
      syncYaml: false,
      syncResourceKinds: [],
      resourceOverrides: {},
    }));
}

export function EnvironmentSyncJobConfigDrawer({
  open,
  workspaceId,
  productId,
  job,
  environments,
  onClose,
  onConfirm,
}: EnvironmentSyncJobConfigDrawerProps) {
  const [name, setName] = useState(job.name);
  const [draft, setDraft] = useState<EnvironmentSyncJobSpec>(() => parseEnvironmentSyncJobSpec(job.spec));
  const [sourceStatus, setSourceStatus] = useState<EnvironmentStatusVM | null>(null);
  const [targetStatus, setTargetStatus] = useState<EnvironmentStatusVM | null>(null);
  const [sourceLoading, setSourceLoading] = useState(false);
  const [targetLoading, setTargetLoading] = useState(false);
  const [sourceError, setSourceError] = useState<string | null>(null);
  const [targetError, setTargetError] = useState<string | null>(null);
  const [projectsInitialized, setProjectsInitialized] = useState(false);
  const [editorProjectId, setEditorProjectId] = useState<string | null>(null);
  const [editorResources, setEditorResources] = useState<EnvironmentSyncSourceResourceVM[]>([]);
  const [editorLoading, setEditorLoading] = useState(false);
  const [editorError, setEditorError] = useState<string | null>(null);

  const activeEnvironments = useMemo(
    () => environments.filter((item) => item.status !== "disabled"),
    [environments],
  );

  const targetEnvironmentChoices = useMemo(
    () => activeEnvironments.filter((item) => item.id !== draft.sourceEnvironmentId),
    [activeEnvironments, draft.sourceEnvironmentId],
  );

  const targetProjectIds = useMemo(
    () => new Set((targetStatus?.projects ?? []).map((item) => item.project_id)),
    [targetStatus],
  );

  const normalizedProjects = useMemo(
    () => normalizeEnvironmentSyncProjects(draft.projects, targetProjectIds),
    [draft.projects, targetProjectIds],
  );

  const validationErrors = useMemo(
    () => validateEnvironmentSyncJob({ ...draft, projects: normalizedProjects }),
    [draft, normalizedProjects],
  );

  const enabledSourceProjects = useMemo(
    () => (sourceStatus?.projects ?? []).filter((item) => item.enabled),
    [sourceStatus],
  );

  const selectedProjectIds = useMemo(
    () => new Set(normalizedProjects.map((item) => item.projectId)),
    [normalizedProjects],
  );

  useEffect(() => {
    if (!open) return;
    const parsed = parseEnvironmentSyncJobSpec(job.spec);
    setName(job.name);
    setDraft(parsed);
    setSourceStatus(null);
    setTargetStatus(null);
    setSourceError(null);
    setTargetError(null);
    setProjectsInitialized(parsed.projects.length > 0);
    setEditorProjectId(null);
    setEditorResources([]);
    setEditorLoading(false);
    setEditorError(null);
  }, [open, job.name, job.spec]);

  useEffect(() => {
    if (!open || !workspaceId || !productId || !draft.sourceEnvironmentId) {
      setSourceStatus(null);
      setSourceLoading(false);
      setSourceError(null);
      return;
    }
    let cancelled = false;
    setSourceLoading(true);
    setSourceError(null);
    void product
      .getEnvironmentStatus(workspaceId, productId, draft.sourceEnvironmentId)
      .then((status) => {
        if (cancelled) return;
        setSourceStatus(status);
        setSourceLoading(false);
      })
      .catch((err) => {
        if (cancelled) return;
        setSourceStatus(null);
        setSourceLoading(false);
        setSourceError(err instanceof BffError ? err.message : "加载源环境项目状态失败");
      });
    return () => {
      cancelled = true;
    };
  }, [open, workspaceId, productId, draft.sourceEnvironmentId]);

  useEffect(() => {
    if (!open || !workspaceId || !productId || !draft.targetEnvironmentId) {
      setTargetStatus(null);
      setTargetLoading(false);
      setTargetError(null);
      return;
    }
    let cancelled = false;
    setTargetLoading(true);
    setTargetError(null);
    void product
      .getEnvironmentStatus(workspaceId, productId, draft.targetEnvironmentId)
      .then((status) => {
        if (cancelled) return;
        setTargetStatus(status);
        setTargetLoading(false);
      })
      .catch((err) => {
        if (cancelled) return;
        setTargetStatus(null);
        setTargetLoading(false);
        setTargetError(err instanceof BffError ? err.message : "加载目标环境项目状态失败");
      });
    return () => {
      cancelled = true;
    };
  }, [open, workspaceId, productId, draft.targetEnvironmentId]);

  useEffect(() => {
    if (!open || projectsInitialized || sourceLoading || !sourceStatus) return;
    const defaults = defaultProjectsFromStatus(sourceStatus);
    if (defaults.length === 0) return;
    setDraft((current) => ({ ...current, projects: defaults }));
    setProjectsInitialized(true);
  }, [open, projectsInitialized, sourceLoading, sourceStatus]);

  useEffect(() => {
    if (!open || !draft.targetEnvironmentId) return;
    if (draft.targetEnvironmentId === draft.sourceEnvironmentId) {
      setDraft((current) => ({ ...current, targetEnvironmentId: "" }));
    }
  }, [open, draft.sourceEnvironmentId, draft.targetEnvironmentId]);

  const statusBusy = sourceLoading || targetLoading;
  const canConfirm =
    name.trim().length > 0 &&
    isEnvironmentSyncJobValid(validationErrors) &&
    !statusBusy &&
    normalizedProjects.every(isEnvironmentSyncProjectContentValid);

  function projectSpec(projectId: string): EnvironmentSyncProjectSpec | undefined {
    return normalizedProjects.find((item) => item.projectId === projectId);
  }

  function toggleProject(projectId: string, selected: boolean) {
    setProjectsInitialized(true);
    setDraft((current) => {
      if (selected) {
        if (current.projects.some((item) => item.projectId === projectId)) return current;
        return {
          ...current,
          projects: [{
            projectId,
            syncImage: true,
            syncYaml: false,
            syncResourceKinds: [],
            resourceOverrides: {},
          }],
        };
      }
      return {
        ...current,
        projects: current.projects.filter((item) => item.projectId !== projectId),
      };
    });
  }

  function updateProjectOption(
    projectId: string,
    patch: Partial<Pick<EnvironmentSyncProjectSpec, "syncImage" | "syncYaml" | "syncResourceKinds">>,
  ) {
    setProjectsInitialized(true);
    setDraft((current) => ({
      ...current,
      projects: current.projects.map((item) => {
        if (item.projectId !== projectId) return item;
        const next = { ...item, ...patch };
        if (patch.syncYaml === true && next.syncResourceKinds.length === 0) {
          next.syncResourceKinds = [...DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS];
        }
        if (patch.syncYaml === false) {
          next.syncResourceKinds = [];
        }
        return next;
      }),
    }));
  }

  function updateProjectResourceKind(
    projectId: string,
    kind: EnvironmentSyncResourceKind,
    selected: boolean,
    locked: boolean,
  ) {
    if (locked) return;
    setProjectsInitialized(true);
    setDraft((current) => ({
      ...current,
      projects: current.projects.map((item) =>
        item.projectId === projectId
          ? {
              ...item,
              syncResourceKinds: toggleSyncResourceKind(item.syncResourceKinds, kind, selected),
            }
          : item,
      ),
    }));
  }

  function updateProjectResourceOverrides(projectId: string, resourceOverrides: Record<string, string>) {
    setProjectsInitialized(true);
    setDraft((current) => ({
      ...current,
      projects: current.projects.map((item) =>
        item.projectId === projectId ? { ...item, resourceOverrides } : item,
      ),
    }));
    setEditorProjectId(null);
  }

  async function openResourceEditor(projectId: string) {
    const spec = projectSpec(projectId);
    if (!spec?.syncYaml || !draft.sourceEnvironmentId) return;
    const normalizedSpec = normalizedProjects.find((item) => item.projectId === projectId) ?? spec;
    setEditorProjectId(projectId);
    setEditorResources([]);
    setEditorLoading(true);
    setEditorError(null);
    try {
      const result = await product.listEnvironmentSyncSourceResources(workspaceId, productId, {
        source_environment_id: draft.sourceEnvironmentId,
        projects: [environmentSyncProjectToApiRecord(normalizedSpec) as never],
      });
      const resources =
        result.projects.find((item) => item.project_id === projectId)?.resources ?? [];
      setEditorResources(resources);
    } catch (err) {
      setEditorError(err instanceof BffError ? err.message : "加载源环境配置失败");
    } finally {
      setEditorLoading(false);
    }
  }

  function renderProjectSection() {
    if (!draft.sourceEnvironmentId) {
      return (
        <div className="pwf-env-sync-empty" role="status">
          <p>选择源环境后显示可同步的项目</p>
        </div>
      );
    }

    if (!draft.targetEnvironmentId) {
      return (
        <div className="pwf-env-sync-empty" role="status">
          <p>选择目标环境后配置同步内容</p>
        </div>
      );
    }

    if (sourceLoading || targetLoading) {
      return (
        <div className="pwf-env-sync-empty" role="status">
          <p>正在加载环境项目…</p>
        </div>
      );
    }

    if (sourceError) {
      return <InlineAlert tone="error">{sourceError}</InlineAlert>;
    }

    if (targetError) {
      return <InlineAlert tone="error">{targetError}</InlineAlert>;
    }

    if (enabledSourceProjects.length === 0) {
      return (
        <div className="pwf-env-sync-empty">
          <p>源环境没有已启用的项目</p>
        </div>
      );
    }

    return (
      <ul className="pwf-env-sync-projects">
        {enabledSourceProjects.map((item) => {
          const selected = selectedProjectIds.has(item.project_id);
          const spec = projectSpec(item.project_id);
          const firstSync = !targetProjectIds.has(item.project_id);
          const syncImage = firstSync ? true : (spec?.syncImage ?? true);
          const syncYaml = firstSync ? true : (spec?.syncYaml ?? false);
          const syncResourceKinds = firstSync
            ? [...DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS]
            : (spec?.syncResourceKinds ?? []);

          return (
            <li
              key={item.project_id}
              className={`pwf-env-sync-project-row${selected ? " pwf-env-sync-project-row--active" : ""}`}
            >
              <div className="pwf-env-sync-project-head">
                <label className="pwf-check pwf-env-sync-project-select">
                  <input
                    type="checkbox"
                    checked={selected}
                    onChange={(event) => toggleProject(item.project_id, event.target.checked)}
                  />
                  <span className="pwf-env-sync-project-name">
                    {item.display_name || item.project_name}
                  </span>
                </label>
                {firstSync ? <span className="pwf-env-sync-first-badge">首次同步</span> : null}
              </div>
              {selected ? (
                <>
                  <div className="pwf-env-sync-options">
                    <label
                      className={`pwf-env-sync-option${syncImage ? " pwf-env-sync-option--on" : ""}`}
                      title={SYNC_OPTION_IMAGE_HINT}
                    >
                      <input
                        type="checkbox"
                        checked={syncImage}
                        disabled={firstSync}
                        onChange={(event) =>
                          updateProjectOption(item.project_id, { syncImage: event.target.checked })
                        }
                      />
                      {SYNC_OPTION_IMAGE_LABEL}
                    </label>
                    <label
                      className={`pwf-env-sync-option${syncYaml ? " pwf-env-sync-option--on" : ""}`}
                      title={SYNC_OPTION_YAML_HINT}
                    >
                      <input
                        type="checkbox"
                        checked={syncYaml}
                        disabled={firstSync}
                        onChange={(event) =>
                          updateProjectOption(item.project_id, { syncYaml: event.target.checked })
                        }
                      />
                      {SYNC_OPTION_YAML_LABEL}
                    </label>
                  </div>
                  {syncYaml ? (
                    <div className="pwf-env-sync-kinds">
                      <span className="pwf-env-sync-kinds-label">资源类型</span>
                      <div className="pwf-env-sync-kinds-list">
                        {ENVIRONMENT_SYNC_RESOURCE_KINDS.map((kind) => {
                          const kindSelected = syncResourceKinds.includes(kind.id);
                          return (
                            <label
                              key={kind.id}
                              className={`pwf-env-sync-kind${kindSelected ? " pwf-env-sync-kind--on" : ""}`}
                              title={kind.hint}
                            >
                              <input
                                type="checkbox"
                                checked={kindSelected}
                                disabled={firstSync}
                                onChange={(event) =>
                                  updateProjectResourceKind(
                                    item.project_id,
                                    kind.id,
                                    event.target.checked,
                                    firstSync,
                                  )
                                }
                              />
                              {kind.label}
                            </label>
                          );
                        })}
                      </div>
                    </div>
                  ) : null}
                  <dl className="pwf-env-sync-content">
                    {buildProjectSyncContentLines({
                      projectId: item.project_id,
                      sourceProjects: sourceStatus?.projects,
                      targetProjects: targetStatus?.projects,
                      syncImage,
                      syncYaml,
                      syncResourceKinds,
                      firstSync,
                    }).map((line) => {
                      const editable = line.label === SYNC_OPTION_YAML_LABEL && syncYaml;
                      const overrideCount = Object.keys(spec?.resourceOverrides ?? {}).length;
                      return (
                        <div
                          key={line.label}
                          className={`pwf-env-sync-content-row${line.muted ? " pwf-env-sync-content-row--muted" : ""}${editable ? " pwf-env-sync-content-row--action" : ""}`}
                        >
                          <dt>{line.label}</dt>
                          <dd className={line.label === "容器镜像" ? "mono" : undefined}>
                            {editable ? (
                              <button
                                type="button"
                                className="pwf-env-sync-content-link"
                                onClick={() => void openResourceEditor(item.project_id)}
                              >
                                {line.value}
                                {overrideCount > 0 ? ` · ${overrideCount} 项已修改` : " · 查看并编辑"}
                              </button>
                            ) : (
                              line.value
                            )}
                          </dd>
                        </div>
                      );
                    })}
                  </dl>
                </>
              ) : null}
            </li>
          );
        })}
      </ul>
    );
  }

  return (
    <Drawer
      open={open}
      title="环境部署同步"
      description="将源环境中的项目部署状态同步到目标环境"
      onClose={onClose}
      panelClassName="drawer-panel--config pwf-env-sync-drawer"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn-primary"
            disabled={!canConfirm}
            onClick={() =>
              onConfirm({
                ...job,
                name: name.trim(),
                spec: environmentSyncJobSpecToRecord({
                  ...draft,
                  projects: normalizedProjects,
                }),
              })
            }
          >
            确定
          </button>
        </div>
      }
    >
      <div className="stack-md pwf-env-sync-form">
        <section className="pwf-env-sync-section">
          <label className="pwf-env-sync-field">
            <span className="pwf-env-sync-label">
              任务名称
              <span className="pwf-required" aria-hidden>
                *
              </span>
            </span>
            <input
              className="input"
              value={name}
              onChange={(event) => setName(event.target.value)}
              aria-label="任务名称"
            />
          </label>
        </section>

        <section className="pwf-env-sync-section">
          <h3 className="config-section-title">环境范围</h3>
          <div className="pwf-env-sync-env-flow">
            <label className="pwf-env-sync-env-field">
              <span className="pwf-env-sync-label">
                源环境
                <span className="pwf-required" aria-hidden>
                  *
                </span>
              </span>
              <select
                className="input"
                aria-label="源环境"
                value={draft.sourceEnvironmentId}
                onChange={(event) => {
                  const sourceEnvironmentId = event.target.value;
                  setProjectsInitialized(false);
                  setDraft((current) => ({
                    ...current,
                    sourceEnvironmentId,
                    targetEnvironmentId:
                      current.targetEnvironmentId === sourceEnvironmentId
                        ? ""
                        : current.targetEnvironmentId,
                    projects: [],
                  }));
                }}
              >
                <option value="">
                  {activeEnvironments.length === 0 ? "还没有可用环境" : "请选择源环境"}
                </option>
                {activeEnvironments.map((env) => (
                  <option key={env.id} value={env.id}>
                    {env.name}
                  </option>
                ))}
              </select>
              {validationErrors.sourceEnvironmentId ? (
                <span className="pwf-env-sync-field-error">{validationErrors.sourceEnvironmentId}</span>
              ) : null}
            </label>

            <span className="pwf-env-sync-flow-arrow" aria-hidden>
              →
            </span>

            <label className="pwf-env-sync-env-field">
              <span className="pwf-env-sync-label">
                目标环境
                <span className="pwf-required" aria-hidden>
                  *
                </span>
              </span>
              <select
                className="input"
                aria-label="目标环境"
                value={draft.targetEnvironmentId}
                disabled={!draft.sourceEnvironmentId}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, targetEnvironmentId: event.target.value }))
                }
              >
                <option value="">
                  {!draft.sourceEnvironmentId
                    ? "请先选择源环境"
                    : targetEnvironmentChoices.length === 0
                      ? "没有可选的目标环境"
                      : "请选择目标环境"}
                </option>
                {targetEnvironmentChoices.map((env) => (
                  <option key={env.id} value={env.id}>
                    {env.name}
                  </option>
                ))}
              </select>
              {validationErrors.targetEnvironmentId ? (
                <span className="pwf-env-sync-field-error">{validationErrors.targetEnvironmentId}</span>
              ) : null}
            </label>
          </div>
        </section>

        <section className="pwf-env-sync-section">
          <div className="pwf-env-sync-section-head">
            <h3 className="config-section-title">项目与同步内容</h3>
            {normalizedProjects.length > 0 ? (
              <span className="pwf-env-sync-count">{normalizedProjects.length} 已选</span>
            ) : null}
          </div>
          {validationErrors.projects ? (
            <span className="pwf-env-sync-field-error">{validationErrors.projects}</span>
          ) : null}
          {renderProjectSection()}
        </section>

        <p className="pwf-env-sync-footnote">
          「容器镜像」指同步源环境当前运行的镜像版本；「部署配置」指同步 K8s 资源 YAML，可点击查看并修改后再发布。首次同步强制两项全开。YAML
          同步仅创建或更新，不删除目标独有资源，也不复制 Secret 密文。完整差异请在启动工作流前通过「生成同步预览」确认。
        </p>
      </div>

      <EnvironmentSyncResourceEditorModal
        open={editorProjectId !== null}
        projectName={
          enabledSourceProjects.find((item) => item.project_id === editorProjectId)?.display_name
          ?? enabledSourceProjects.find((item) => item.project_id === editorProjectId)?.project_name
          ?? editorProjectId
          ?? ""
        }
        projectSpec={
          normalizedProjects.find((item) => item.projectId === editorProjectId)
          ?? { projectId: editorProjectId ?? "", syncImage: true, syncYaml: true, syncResourceKinds: [], resourceOverrides: {} }
        }
        resources={editorResources}
        loading={editorLoading}
        error={editorError}
        onClose={() => setEditorProjectId(null)}
        onSave={(overrides) => {
          if (editorProjectId) updateProjectResourceOverrides(editorProjectId, overrides);
        }}
      />
    </Drawer>
  );
}
