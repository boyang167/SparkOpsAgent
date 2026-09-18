"use client";

import { useEffect, useMemo, useState } from "react";
import type { ProductProjectVM, TemplateVM } from "@sparkops/contracts/product";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { product, secret, workspace } from "@sparkops/web-sdk";
import { Drawer } from "@/components/ui/Drawer";
import { InlineAlert } from "@/components/ui/InlineAlert";
import {
  buildJobSpecToRecord,
  parseBuildJobSpec,
  type BuildJobSpec,
  type BuildServiceRow,
} from "./build-job-spec";
import { BuildCodeRepoFields } from "./BuildCodeRepoFields";
import { BuildNotifyList } from "./BuildNotifyList";

type BuildTab = "basic" | "strategy" | "notify";
type RowEditor = "code" | "vars" | "script";

interface BuildJobConfigDrawerProps {
  open: boolean;
  workspaceId: string;
  jobName: string;
  spec: Record<string, unknown> | undefined;
  projects: ProductProjectVM[];
  onClose: () => void;
  onConfirm: (name: string, spec: Record<string, unknown>) => void;
}

function projectLabel(project: { display_name?: string; name?: string; id: string }): string {
  return project.display_name || project.name || project.id;
}

function isGitSecret(item: SecretVM): boolean {
  const type = item.service_type.toLowerCase();
  return type === "gitlab" || type === "github";
}

function isRegistry(item: SecretVM): boolean {
  const type = item.service_type.toLowerCase();
  return type.includes("harbor") || type.includes("registry") || type.includes("docker");
}

function projectGitSecretId(project: ProjectVM | ProductProjectVM | undefined): string {
  if (!project || !("secret_id" in project)) return "";
  return project.secret_id?.trim() ?? "";
}

function secretName(item: SecretVM): string {
  return item.display_name?.trim() || item.name;
}

function emptyRow(project: ProjectVM | ProductProjectVM, templates: TemplateVM[]): BuildServiceRow {
  const full = "git_url" in project ? project : null;
  const templateId = full?.resources?.jenkinsfile_template_id;
  const matched = templates.find((item) => item.id === templateId);
  return {
    project_id: project.id,
    build_name: matched?.name || `${project.name || "service"}-build`,
    branch: full?.git_ref_value || "main",
    git_url: full?.git_url || "",
    secret_id: projectGitSecretId(project),
    git_ref_type: full?.git_ref_type === "tag" ? "tag" : "branch",
    dockerfile_path: full?.dockerfile_path || "./Dockerfile",
    variables: [],
    script: "",
  };
}

export function BuildJobConfigDrawer({
  open,
  workspaceId,
  jobName,
  spec,
  projects,
  onClose,
  onConfirm,
}: BuildJobConfigDrawerProps) {
  const [tab, setTab] = useState<BuildTab>("basic");
  const [name, setName] = useState(jobName);
  const [draft, setDraft] = useState<BuildJobSpec>(() => parseBuildJobSpec(spec));
  const [registries, setRegistries] = useState<SecretVM[]>([]);
  const [gitSecrets, setGitSecrets] = useState<SecretVM[]>([]);
  const [templates, setTemplates] = useState<TemplateVM[]>([]);
  const [details, setDetails] = useState<ProjectVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [pendingProjectId, setPendingProjectId] = useState("");
  const [editing, setEditing] = useState<{ index: number; kind: RowEditor } | null>(null);

  useEffect(() => {
    if (!open) return;
    setTab("basic");
    setName(jobName);
    setDraft(parseBuildJobSpec(spec));
    setPickerOpen(false);
    setPendingProjectId("");
    setEditing(null);
    setLoadError(null);
    if (!workspaceId) {
      setLoadError("缺少工作空间，无法加载镜像仓库");
      return;
    }
    let cancelled = false;
    setLoading(true);
    void Promise.all([
      secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
      product.listTemplates(workspaceId).catch(() => [] as TemplateVM[]),
      workspace.listProjects(workspaceId).catch(() => [] as ProjectVM[]),
    ])
      .then(([secretItems, templateItems, projectItems]) => {
        if (cancelled) return;
        const allowed = new Set(projects.map((item) => item.id));
        setRegistries(secretItems.filter(isRegistry));
        setGitSecrets(secretItems.filter(isGitSecret));
        setTemplates(templateItems.filter((item) => (item.kind ?? "").toLowerCase() === "jenkinsfile"));
        const scoped = projectItems.filter((item) => allowed.has(item.id));
        setDetails(scoped);
        const bound = new Map(scoped.map((item) => [item.id, item.secret_id?.trim() ?? ""]));
        setDraft((current) => ({
          ...current,
          project_builds: current.project_builds.map((row) =>
            row.secret_id ? row : { ...row, secret_id: bound.get(row.project_id) ?? "" },
          ),
        }));
      })
      .catch(() => {
        if (!cancelled) setLoadError("加载构建配置失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, workspaceId, jobName, spec, projects]);

  const jenkinsTemplates = templates;
  const projectById = useMemo(() => {
    const map = new Map<string, ProjectVM | ProductProjectVM>();
    for (const project of projects) map.set(project.id, project);
    for (const project of details) map.set(project.id, project);
    return map;
  }, [projects, details]);

  const selectedIds = new Set(draft.project_builds.map((row) => row.project_id));
  const canConfirm = name.trim().length > 0 && draft.registry_id.trim().length > 0;
  const editingRow = editing ? draft.project_builds[editing.index] : null;
  const boundGitSecretId = editingRow ? projectGitSecretId(projectById.get(editingRow.project_id)) : "";
  const selectedGitSecretId = editingRow ? editingRow.secret_id || boundGitSecretId : "";

  function patchRow(index: number, patch: Partial<BuildServiceRow>) {
    setDraft((current) => ({
      ...current,
      project_builds: current.project_builds.map((row, i) => (i === index ? { ...row, ...patch } : row)),
    }));
  }

  function addProject(projectId: string) {
    if (!projectId || selectedIds.has(projectId)) return;
    const project = projectById.get(projectId);
    if (!project) return;
    setDraft((current) => ({
      ...current,
      project_builds: [...current.project_builds, emptyRow(project, jenkinsTemplates)],
    }));
  }

  function removeProject(projectId: string) {
    setDraft((current) => ({
      ...current,
      project_builds: current.project_builds.filter((row) => row.project_id !== projectId),
    }));
  }

  return (
    <>
      <Drawer
        open={open}
        onClose={onClose}
        title="构建"
        panelClassName="drawer-panel-wide pwf-build-drawer"
        tabs={
          <>
            {(
              [
                ["basic", "基础配置"],
                ["strategy", "策略配置"],
                ["notify", "通知配置"],
              ] as const
            ).map(([id, label]) => (
              <button
                key={id}
                type="button"
                role="tab"
                aria-selected={tab === id}
                className={`drawer-tab${tab === id ? " drawer-tab-active" : ""}`}
                onClick={() => setTab(id)}
              >
                {label}
              </button>
            ))}
            <span className="pwf-build-tab-spacer" />
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={!canConfirm}
              onClick={() => onConfirm(name.trim(), buildJobSpecToRecord({ ...draft, registry_id: draft.registry_id.trim() }))}
            >
              确定
            </button>
            <button type="button" className="btn btn-secondary text-sm" onClick={onClose}>
              取消
            </button>
          </>
        }
      >
        {loading ? (
          <p className="text-sm text-muted" role="status">
            加载构建配置…
          </p>
        ) : null}
        {loadError ? <InlineAlert tone="error">{loadError}</InlineAlert> : null}

        {tab === "basic" ? (
          <div className="pwf-build-form">
            <label className="pwf-build-field">
              <span>任务名称</span>
              <span className="pwf-build-control">
                <span className="pwf-required" aria-hidden>
                  *
                </span>
                <input className="input" value={name} aria-label="任务名称" onChange={(e) => setName(e.target.value)} />
              </span>
            </label>
            <label className="pwf-build-field">
              <span>镜像仓库</span>
              <span className="pwf-build-control">
                <span className="pwf-required" aria-hidden>
                  *
                </span>
                <select
                  className="input"
                  aria-label="镜像仓库"
                  value={draft.registry_id}
                  onChange={(e) => setDraft((current) => ({ ...current, registry_id: e.target.value }))}
                >
                  <option value="">请选择镜像仓库</option>
                  {registries.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.service_url || item.display_name || item.name}
                    </option>
                  ))}
                </select>
              </span>
            </label>
            {registries.length === 0 && !loading ? (
              <p className="text-xs text-muted">工作空间还没有 Harbor 或 Docker Registry 密钥。</p>
            ) : null}

            <div className="pwf-build-field">
              <span>项目组件</span>
              <div className="pwf-chip-select">
                {draft.project_builds.map((row) => {
                  const project = projectById.get(row.project_id);
                  return (
                    <span key={row.project_id} className="pwf-chip">
                      {project ? projectLabel(project) : row.project_id}
                      <button type="button" aria-label="移除项目" onClick={() => removeProject(row.project_id)}>
                        ×
                      </button>
                    </span>
                  );
                })}
                <button type="button" className="pwf-chip-add" onClick={() => setPickerOpen((value) => !value)}>
                  选择项目
                </button>
              </div>
              {pickerOpen ? (
                <div className="pwf-chip-menu">
                  {projects.length === 0 ? <p className="text-xs text-muted">暂无关联项目</p> : null}
                  {projects.map((project) => (
                    <label key={project.id} className="pwf-check">
                      <input
                        type="checkbox"
                        checked={selectedIds.has(project.id)}
                        onChange={(e) => (e.target.checked ? addProject(project.id) : removeProject(project.id))}
                      />
                      {projectLabel(project)}
                    </label>
                  ))}
                </div>
              ) : null}
            </div>

            <div className="pwf-build-table-wrap">
              <table className="pwf-build-table">
                <thead>
                  <tr>
                    <th>项目组件</th>
                    <th>Jenkinsfile 模板</th>
                    <th>构建配置</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {draft.project_builds.map((row, index) => {
                    const project = projectById.get(row.project_id);
                    const names = new Set(jenkinsTemplates.map((item) => item.name));
                    if (row.build_name) names.add(row.build_name);
                    return (
                      <tr key={row.project_id}>
                        <td>{project ? projectLabel(project) : row.project_id}</td>
                        <td>
                          <select
                            className="input"
                            aria-label="Jenkinsfile 模板"
                            value={row.build_name}
                            onChange={(e) => patchRow(index, { build_name: e.target.value })}
                          >
                            <option value="">请选择</option>
                            {[...names].map((item) => (
                              <option key={item} value={item}>
                                {item}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td>
                          <div className="pwf-build-icons">
                            <button type="button" aria-label="代码配置" onClick={() => setEditing({ index, kind: "code" })}>
                              {"</>"}
                            </button>
                            <button type="button" aria-label="变量配置" onClick={() => setEditing({ index, kind: "vars" })}>
                              节点
                            </button>
                            <button type="button" aria-label="脚本配置" onClick={() => setEditing({ index, kind: "script" })}>
                              脚本
                            </button>
                          </div>
                        </td>
                        <td>
                          <button type="button" className="pwf-build-delete" onClick={() => removeProject(row.project_id)}>
                            删除
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                  <tr>
                    <td colSpan={4}>
                      <div className="pwf-build-add-row">
                        <select
                          className="input"
                          aria-label="添加项目组件"
                          value={pendingProjectId}
                          onChange={(e) => setPendingProjectId(e.target.value)}
                        >
                          <option value="">请选择</option>
                          {projects
                            .filter((project) => !selectedIds.has(project.id))
                            .map((project) => (
                              <option key={project.id} value={project.id}>
                                {projectLabel(project)}
                              </option>
                            ))}
                        </select>
                        <button
                          type="button"
                          className="btn btn-secondary text-sm"
                          disabled={!pendingProjectId}
                          onClick={() => {
                            addProject(pendingProjectId);
                            setPendingProjectId("");
                          }}
                        >
                          + 添加
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            {!canConfirm ? <p className="text-xs text-muted">任务名称和镜像仓库为必填。</p> : null}
            {editing && editingRow ? (
              <div className="pwf-build-inline">
                <div className="pwf-build-inline-head">
                  <strong>
                    {editing.kind === "code" ? "代码配置" : editing.kind === "vars" ? "变量配置" : "脚本配置"}
                  </strong>
                  <button type="button" className="btn btn-secondary text-sm" onClick={() => setEditing(null)}>
                    完成
                  </button>
                </div>
                {editing.kind === "code" ? (
                  <div className="stack-sm">
                    <label className="field">
                      <span className="text-xs text-muted">代码库</span>
                      <input
                        className="input"
                        value={editingRow.git_url}
                        onChange={(e) => patchRow(editing.index, { git_url: e.target.value })}
                      />
                    </label>
                    <label className="field">
                      <span className="text-xs text-muted">仓库密钥</span>
                      <select
                        className="input"
                        aria-label="仓库密钥"
                        value={selectedGitSecretId}
                        onChange={(e) => patchRow(editing.index, { secret_id: e.target.value })}
                      >
                        {!selectedGitSecretId ? <option value="">请选择</option> : null}
                        {gitSecrets.map((item) => (
                          <option key={item.id} value={item.id}>
                            {secretName(item)}
                          </option>
                        ))}
                        {selectedGitSecretId && !gitSecrets.some((item) => item.id === selectedGitSecretId) ? (
                          <option value={selectedGitSecretId}>{selectedGitSecretId}</option>
                        ) : null}
                      </select>
                    </label>
                    <BuildCodeRepoFields
                      workspaceId={workspaceId}
                      row={{ ...editingRow, secret_id: selectedGitSecretId }}
                      gitSecrets={gitSecrets}
                      selectedSecretId={selectedGitSecretId}
                      onPatch={(patch) => patchRow(editing.index, patch)}
                    />
                  </div>
                ) : null}
                {editing.kind === "vars" ? (
                  <div className="stack-sm">
                    {editingRow.variables.map((variable, varIndex) => (
                      <div key={`${variable.key}-${varIndex}`} className="pwf-build-var-row">
                        <input
                          className="input"
                          aria-label="变量名"
                          placeholder="变量名"
                          value={variable.key}
                          onChange={(e) => {
                            const variables = editingRow.variables.map((item, i) =>
                              i === varIndex ? { ...item, key: e.target.value } : item,
                            );
                            patchRow(editing.index, { variables });
                          }}
                        />
                        <input
                          className="input"
                          aria-label="变量值"
                          placeholder="变量值"
                          value={variable.value}
                          onChange={(e) => {
                            const variables = editingRow.variables.map((item, i) =>
                              i === varIndex ? { ...item, value: e.target.value } : item,
                            );
                            patchRow(editing.index, { variables });
                          }}
                        />
                        <button
                          type="button"
                          className="btn btn-ghost text-xs"
                          onClick={() =>
                            patchRow(editing.index, {
                              variables: editingRow.variables.filter((_, i) => i !== varIndex),
                            })
                          }
                        >
                          删除
                        </button>
                      </div>
                    ))}
                    <button
                      type="button"
                      className="btn btn-secondary text-sm"
                      onClick={() =>
                        patchRow(editing.index, { variables: [...editingRow.variables, { key: "", value: "" }] })
                      }
                    >
                      + 添加变量
                    </button>
                  </div>
                ) : null}
                {editing.kind === "script" ? (
                  <label className="field">
                    <span className="text-xs text-muted">构建脚本</span>
                    <textarea
                      className="input"
                      rows={8}
                      value={editingRow.script}
                      onChange={(e) => patchRow(editing.index, { script: e.target.value })}
                    />
                  </label>
                ) : null}
              </div>
            ) : null}
          </div>
        ) : null}

        {tab === "strategy" ? (
          <div className="pwf-build-form">
            <label className="pwf-build-field">
              <span>超时（分钟）</span>
              <input
                className="input"
                type="number"
                min={1}
                value={draft.strategy.timeout_minutes}
                onChange={(e) =>
                  setDraft((current) => ({
                    ...current,
                    strategy: { ...current.strategy, timeout_minutes: Math.max(1, Number(e.target.value) || 60) },
                  }))
                }
              />
            </label>
            <div className="pwf-build-field">
              <span>构建缓存</span>
              <button
                type="button"
                className={`menu-switch${draft.strategy.cache ? " menu-switch-on" : ""}`}
                aria-pressed={draft.strategy.cache}
                aria-label="构建缓存"
                onClick={() =>
                  setDraft((current) => ({
                    ...current,
                    strategy: { ...current.strategy, cache: !current.strategy.cache },
                  }))
                }
              >
                <span className="menu-switch-thumb" />
              </button>
            </div>
            <p className="text-xs text-muted">超时和缓存会写入任务规格，构建仍由项目的 Jenkins 流水线执行。</p>
          </div>
        ) : null}

        {tab === "notify" ? (
          <BuildNotifyList
            items={draft.notify}
            onChange={(notify) => setDraft((current) => ({ ...current, notify }))}
          />
        ) : null}
      </Drawer>
    </>
  );
}
