"use client";

import { useEffect, useMemo, useState } from "react";
import type { EnvironmentStatusVM, EnvironmentVM, ProductProjectVM } from "@sparkops/contracts/product";
import { BffError } from "@sparkops/contracts";
import { product } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import {
  normalizeEnvironmentSyncProjects,
  validateEnvironmentSyncJob,
} from "./environment-sync-job-validation";
import {
  clearEnvironmentSyncPreview,
  confirmEnvironmentSyncPreview,
  groupDiffsByAction,
  setEnvironmentSyncPendingPreview,
  toggleEnvironmentSyncProject,
  updateEnvironmentSyncProject,
  updateEnvironmentSyncSource,
  updateEnvironmentSyncTarget,
  type EnvironmentSyncPreviewProject,
  type EnvironmentSyncPreviewVM,
  type EnvironmentSyncResourceDiff,
  type EnvironmentSyncRunJobState,
} from "./environment-sync-run-state";
import { environmentSyncProjectToApiRecord } from "./environment-sync-job-spec";
import {
  EnvironmentSyncResourceEditorModal,
  type EnvironmentSyncSourceResourceVM,
} from "./EnvironmentSyncResourceEditorModal";
import { environmentSyncResourceKey } from "./environment-sync-resource-overrides";
import {
  SYNC_OPTION_IMAGE_HINT,
  SYNC_OPTION_IMAGE_LABEL,
  SYNC_OPTION_YAML_HINT,
  SYNC_OPTION_YAML_LABEL,
  buildProjectSyncContentLines,
} from "./environment-sync-content-display";
import {
  DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS,
  ENVIRONMENT_SYNC_RESOURCE_KINDS,
  toggleSyncResourceKind,
  type EnvironmentSyncResourceKind,
} from "./environment-sync-resource-kinds";

interface EnvironmentSyncRunFieldsProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
  jobName: string;
  environments: EnvironmentVM[];
  projects?: ProductProjectVM[];
  state: EnvironmentSyncRunJobState;
  onChange: (state: EnvironmentSyncRunJobState) => void;
  compact?: boolean;
}

function projectLabel(projects: ProductProjectVM[], projectId: string): string {
  const item = projects.find((project) => project.id === projectId);
  return item?.display_name?.trim() || item?.name || projectId;
}

function environmentName(environments: EnvironmentVM[], environmentId: string): string {
  return environments.find((item) => item.id === environmentId)?.name ?? environmentId;
}

function resourcesFromPreviewProject(project: EnvironmentSyncPreviewProject): EnvironmentSyncSourceResourceVM[] {
  return project.diffs
    .filter(
      (item) =>
        (item.action === "create" || item.action === "update") && Boolean(item.after?.trim() || item.before?.trim()),
    )
    .map((item) => ({
      kind: item.kind,
      name: item.name,
      yaml: item.after ?? item.before ?? "",
      note: item.note,
    }));
}

function diffActionLabel(action: string): string {
  if (action === "create") return "新增";
  if (action === "update") return "更新";
  if (action === "unchanged") return "无变化";
  if (action === "excluded") return "已排除";
  return action;
}

export function EnvironmentSyncRunFields({
  workspaceId,
  productId,
  workflowId,
  jobName,
  environments,
  projects = [],
  state,
  onChange,
  compact = false,
}: EnvironmentSyncRunFieldsProps) {
  const [sourceStatus, setSourceStatus] = useState<EnvironmentStatusVM | null>(null);
  const [targetStatus, setTargetStatus] = useState<EnvironmentStatusVM | null>(null);
  const [sourceLoading, setSourceLoading] = useState(false);
  const [targetLoading, setTargetLoading] = useState(false);
  const [sourceError, setSourceError] = useState<string | null>(null);
  const [targetError, setTargetError] = useState<string | null>(null);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [editorProjectId, setEditorProjectId] = useState<string | null>(null);
  const [editorResources, setEditorResources] = useState<EnvironmentSyncSourceResourceVM[]>([]);
  const [editorLoading, setEditorLoading] = useState(false);
  const [editorError, setEditorError] = useState<string | null>(null);
  const [editorFromPreview, setEditorFromPreview] = useState(false);

  const activeEnvironments = useMemo(
    () => environments.filter((item) => item.status !== "disabled"),
    [environments],
  );
  const targetChoices = useMemo(
    () => activeEnvironments.filter((item) => item.id !== state.sourceEnvironmentId),
    [activeEnvironments, state.sourceEnvironmentId],
  );
  const targetProjectIds = useMemo(
    () => new Set((targetStatus?.projects ?? []).map((item) => item.project_id)),
    [targetStatus],
  );
  const normalizedProjects = useMemo(
    () => normalizeEnvironmentSyncProjects(state.projects, targetProjectIds),
    [state.projects, targetProjectIds],
  );
  const validationErrors = useMemo(
    () =>
      validateEnvironmentSyncJob({
        sourceEnvironmentId: state.sourceEnvironmentId,
        targetEnvironmentId: state.targetEnvironmentId,
        projects: normalizedProjects,
      }),
    [normalizedProjects, state.sourceEnvironmentId, state.targetEnvironmentId],
  );
  const enabledSourceProjects = useMemo(
    () => (sourceStatus?.projects ?? []).filter((item) => item.enabled),
    [sourceStatus],
  );
  const selectedProjectIds = useMemo(
    () => new Set(normalizedProjects.map((item) => item.projectId)),
    [normalizedProjects],
  );
  const preview = state.pendingPreview;
  const confirmed = Boolean(state.previewId.trim());

  useEffect(() => {
    if (!workspaceId || !productId || !state.sourceEnvironmentId) {
      setSourceStatus(null);
      setSourceLoading(false);
      setSourceError(null);
      return;
    }
    let cancelled = false;
    setSourceLoading(true);
    setSourceError(null);
    void product
      .getEnvironmentStatus(workspaceId, productId, state.sourceEnvironmentId)
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
  }, [workspaceId, productId, state.sourceEnvironmentId]);

  useEffect(() => {
    if (!workspaceId || !productId || !state.targetEnvironmentId) {
      setTargetStatus(null);
      setTargetLoading(false);
      setTargetError(null);
      return;
    }
    let cancelled = false;
    setTargetLoading(true);
    setTargetError(null);
    void product
      .getEnvironmentStatus(workspaceId, productId, state.targetEnvironmentId)
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
  }, [workspaceId, productId, state.targetEnvironmentId]);

  async function generatePreview() {
    if (Object.keys(validationErrors).length > 0) return;
    setPreviewBusy(true);
    setPreviewError(null);
    try {
      const result = await product.createEnvironmentSyncPreview(workspaceId, productId, {
        workflow_id: workflowId,
        job_name: jobName,
        source_environment_id: state.sourceEnvironmentId,
        target_environment_id: state.targetEnvironmentId,
        projects: normalizedProjects.map((project) => environmentSyncProjectToApiRecord(project) as never),
      });
      onChange(setEnvironmentSyncPendingPreview(state, result));
    } catch (err) {
      setPreviewError(err instanceof BffError ? err.message : "生成同步预览失败");
    } finally {
      setPreviewBusy(false);
    }
  }

  async function openResourceEditor(projectId: string) {
    const spec = normalizedProjects.find((project) => project.projectId === projectId);
    if (!spec?.syncYaml || !state.sourceEnvironmentId) return;
    setEditorProjectId(projectId);
    setEditorFromPreview(false);
    setEditorResources([]);
    setEditorLoading(true);
    setEditorError(null);
    try {
      const result = await product.listEnvironmentSyncSourceResources(workspaceId, productId, {
        source_environment_id: state.sourceEnvironmentId,
        projects: [environmentSyncProjectToApiRecord(spec) as never],
      });
      setEditorResources(result.projects.find((item) => item.project_id === projectId)?.resources ?? []);
    } catch (err) {
      setEditorError(err instanceof BffError ? err.message : "加载源环境配置失败");
    } finally {
      setEditorLoading(false);
    }
  }

  function openPreviewResourceEditor(project: EnvironmentSyncPreviewProject) {
    setEditorProjectId(project.project_id);
    setEditorFromPreview(true);
    setEditorResources(resourcesFromPreviewProject(project));
    setEditorLoading(false);
    setEditorError(null);
  }

  function saveResourceOverrides(projectId: string, overrides: Record<string, string>) {
    onChange(
      updateEnvironmentSyncProject(state, projectId, { resourceOverrides: overrides }),
    );
    setEditorProjectId(null);
    setEditorFromPreview(false);
  }

  function renderProjectSection() {
    if (!state.sourceEnvironmentId) {
      return (
        <div className="wf-env-sync-run-state" role="status">
          <p className="text-xs text-muted">选择源环境后显示可同步的项目</p>
        </div>
      );
    }
    if (sourceLoading) {
      return (
        <div className="wf-env-sync-run-state" role="status">
          <p className="text-xs text-muted">正在加载源环境项目…</p>
        </div>
      );
    }
    if (sourceError) {
      return (
        <div className="wf-env-sync-run-state">
          <InlineAlert tone="error">{sourceError}</InlineAlert>
        </div>
      );
    }
    if (enabledSourceProjects.length === 0) {
      return (
        <div className="wf-env-sync-run-state">
          <p className="text-xs text-muted">源环境没有已启用的项目</p>
        </div>
      );
    }

    return (
      <ul className="pwf-env-sync-projects">
        {enabledSourceProjects.map((item) => {
          const selected = selectedProjectIds.has(item.project_id);
          const spec = normalizedProjects.find((project) => project.projectId === item.project_id);
          const firstSync = !targetProjectIds.has(item.project_id);
          const syncImage = firstSync ? true : (spec?.syncImage ?? true);
          const syncYaml = firstSync ? true : (spec?.syncYaml ?? false);
          const syncResourceKinds = firstSync
            ? [...DEFAULT_ENVIRONMENT_SYNC_RESOURCE_KINDS]
            : (spec?.syncResourceKinds ?? []);

          return (
            <li key={item.project_id} className="pwf-env-sync-project-row">
              <label className="pwf-check pwf-env-sync-project-select">
                <input
                  type="checkbox"
                  checked={selected}
                  disabled={confirmed}
                  onChange={(event) =>
                    onChange(toggleEnvironmentSyncProject(state, item.project_id, event.target.checked))
                  }
                />
                <span>{item.display_name || item.project_name}</span>
              </label>
              {firstSync ? <span className="badge badge-primary pwf-env-sync-first-badge">首次同步</span> : null}
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
                        disabled={firstSync || confirmed}
                        onChange={(event) =>
                          onChange(
                            updateEnvironmentSyncProject(state, item.project_id, {
                              syncImage: event.target.checked,
                            }),
                          )
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
                        disabled={firstSync || confirmed}
                        onChange={(event) =>
                          onChange(
                            updateEnvironmentSyncProject(state, item.project_id, { syncYaml: event.target.checked }),
                          )
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
                                disabled={firstSync || confirmed}
                                onChange={(event) =>
                                  onChange(
                                    updateEnvironmentSyncProject(state, item.project_id, {
                                      syncResourceKinds: toggleSyncResourceKind(
                                        syncResourceKinds,
                                        kind.id,
                                        event.target.checked,
                                      ),
                                    }),
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
                  {!confirmed ? (
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
                  ) : null}
                </>
              ) : null}
            </li>
          );
        })}
      </ul>
    );
  }

  function renderPreviewDiffItem(project: EnvironmentSyncPreviewProject, item: EnvironmentSyncResourceDiff) {
    const editable = (item.action === "create" || item.action === "update") && !confirmed;
    if (!editable) {
      return (
        <li key={`${item.kind}/${item.name}`}>
          <span className="mono">{item.kind}/{item.name}</span>
          {item.note ? <span className="wf-env-sync-run-diff-note">{item.note}</span> : null}
        </li>
      );
    }
    const key = environmentSyncResourceKey(item.kind, item.name);
    const edited = Boolean(normalizedProjects.find((p) => p.projectId === project.project_id)?.resourceOverrides[key]?.trim());
    return (
      <li key={`${item.kind}/${item.name}`}>
        <button
          type="button"
          className="wf-env-sync-run-diff-link"
          onClick={() => openPreviewResourceEditor(project)}
        >
          <span className="mono">{item.kind}/{item.name}</span>
          {edited ? <span className="pwf-env-sync-editor-edited">已修改</span> : null}
        </button>
        {item.note ? <span className="wf-env-sync-run-diff-note">{item.note}</span> : null}
      </li>
    );
  }

  function renderPreviewProject(project: EnvironmentSyncPreviewVM["projects"][number]) {
    const groups = groupDiffsByAction(project.diffs);
    const editableDiffCount = project.diffs.filter(
      (item) => item.action === "create" || item.action === "update",
    ).length;
    return (
      <article key={project.project_id} className="wf-env-sync-run-preview-project">
        <header className="wf-env-sync-run-preview-project-head">
          <strong>{projectLabel(projects, project.project_id)}</strong>
          {project.first_sync ? <span className="badge badge-primary">首次同步</span> : null}
          {!confirmed && editableDiffCount > 0 ? (
            <button
              type="button"
              className="btn-ghost text-xs"
              onClick={() => openPreviewResourceEditor(project)}
            >
              查看并编辑配置
            </button>
          ) : null}
        </header>
        <div className="wf-env-sync-run-image-row">
          <div>
            <span className="wf-env-sync-run-label">源镜像</span>
            <span className="mono">{project.source_image || "—"}</span>
          </div>
          <div>
            <span className="wf-env-sync-run-label">目标镜像</span>
            <span className="mono">{project.target_image || "—"}</span>
          </div>
        </div>
        {project.first_sync ? (
          <p className="wf-env-sync-run-note">首次同步将强制同步镜像与全部受支持的运行中 YAML，成功后自动建立目标环境关联。</p>
        ) : null}
        <div className="wf-env-sync-run-diff-groups">
          {(["create", "update", "unchanged", "excluded"] as const).map((action) => {
            const items = groups[action];
            if (items.length === 0) return null;
            return (
              <div key={action} className="wf-env-sync-run-diff-group">
                <span className="wf-env-sync-run-diff-title">{diffActionLabel(action)}</span>
                <ul className="wf-env-sync-run-diff-list">
                  {items.map((item) => renderPreviewDiffItem(project, item))}
                </ul>
              </div>
            );
          })}
        </div>
      </article>
    );
  }

  function renderPreviewPanel() {
    if (!preview) return null;
    return (
      <div className="wf-env-sync-run-preview">
        <div className="wf-env-sync-run-preview-head">
          <span className="wf-env-sync-run-preview-title">同步预览</span>
          {!confirmed ? (
            <button
              type="button"
              className="btn btn-secondary text-xs"
              disabled={previewBusy}
              onClick={() => onChange(confirmEnvironmentSyncPreview(state, preview))}
            >
              确认预览
            </button>
          ) : (
            <button
              type="button"
              className="btn-ghost text-xs"
              onClick={() => onChange(clearEnvironmentSyncPreview(state))}
            >
              重新预览
            </button>
          )}
        </div>
        {(preview.warnings ?? []).map((warning) => (
          <InlineAlert key={warning} tone="warning">
            {warning}
          </InlineAlert>
        ))}
        <p className="wf-env-sync-run-note">不会删除目标环境中多余的资源；Kubernetes Secret 密文不会被复制。</p>
        {preview.projects.map((project) => renderPreviewProject(project))}
        {confirmed ? (
          <p className="wf-env-sync-run-confirmed" role="status">
            已确认预览 {state.previewId.slice(0, 8)}…，启动后将使用此快照。
          </p>
        ) : null}
      </div>
    );
  }

  return (
    <div className={`wf-env-sync-run${compact ? " is-compact" : ""}`}>
      <div className="wf-env-sync-run-env-row">
        <label className="wf-run-field">
          <span className="wf-run-field-label">源环境</span>
          <select
            className="input wf-run-select"
            value={state.sourceEnvironmentId}
            disabled={confirmed}
            onChange={(event) => onChange(updateEnvironmentSyncSource(state, event.target.value))}
          >
            <option value="">请选择源环境</option>
            {activeEnvironments.map((env) => (
              <option key={env.id} value={env.id}>
                {env.name}
              </option>
            ))}
          </select>
        </label>
        <label className="wf-run-field">
          <span className="wf-run-field-label">目标环境</span>
          <select
            className="input wf-run-select"
            value={state.targetEnvironmentId}
            disabled={confirmed}
            onChange={(event) => onChange(updateEnvironmentSyncTarget(state, event.target.value))}
          >
            <option value="">请选择目标环境</option>
            {targetChoices.map((env) => (
              <option key={env.id} value={env.id}>
                {env.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      {targetLoading ? <p className="text-xs text-muted">正在加载目标环境项目…</p> : null}
      {targetError ? <InlineAlert tone="error">{targetError}</InlineAlert> : null}

      {renderProjectSection()}

      {validationErrors.sourceEnvironmentId ? (
        <InlineAlert tone="error">{validationErrors.sourceEnvironmentId}</InlineAlert>
      ) : null}
      {validationErrors.targetEnvironmentId ? (
        <InlineAlert tone="error">{validationErrors.targetEnvironmentId}</InlineAlert>
      ) : null}
      {validationErrors.projects ? <InlineAlert tone="error">{validationErrors.projects}</InlineAlert> : null}

      <div className="wf-env-sync-run-actions">
        <button
          type="button"
          className="btn btn-secondary text-sm"
          disabled={previewBusy || confirmed || Object.keys(validationErrors).length > 0}
          onClick={() => void generatePreview()}
        >
          {previewBusy ? "生成中…" : "生成同步预览"}
        </button>
        {confirmed ? (
          <span className="wf-env-sync-run-confirmed-badge" role="status">
            已确认 · {environmentName(environments, state.sourceEnvironmentId)} →{" "}
            {environmentName(environments, state.targetEnvironmentId)}
          </span>
        ) : null}
      </div>

      {previewError ? <InlineAlert tone="error">{previewError}</InlineAlert> : null}
      {renderPreviewPanel()}

      <EnvironmentSyncResourceEditorModal
        open={editorProjectId !== null}
        projectName={projectLabel(projects, editorProjectId ?? "")}
        projectSpec={
          normalizedProjects.find((item) => item.projectId === editorProjectId)
          ?? { projectId: editorProjectId ?? "", syncImage: true, syncYaml: true, syncResourceKinds: [], resourceOverrides: {} }
        }
        resources={editorResources}
        loading={editorLoading}
        error={editorError}
        readOnly={confirmed}
        onClose={() => {
          setEditorProjectId(null);
          setEditorFromPreview(false);
        }}
        onSave={(overrides) => {
          if (editorProjectId) saveResourceOverrides(editorProjectId, overrides);
          if (editorFromPreview) {
            setPreviewError(null);
          }
        }}
      />
    </div>
  );
}
