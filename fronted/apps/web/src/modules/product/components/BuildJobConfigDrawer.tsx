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

function repoLabel(gitUrl: string): string {
  const trimmed = gitUrl.trim().replace(/\.git$/, "").replace(/\/+$/, "");
  if (!trimmed) return "";
  const parts = trimmed.split(/[/:]/).filter(Boolean);
  return parts.slice(-2).join("/");
}

const EDITORS: Array<{ kind: RowEditor; label: string }> = [
  { kind: "code", label: "代码" },
  { kind: "vars", label: "变量" },
  { kind: "script", label: "脚本" },
];

function EditorIcon({ kind }: { kind: RowEditor }) {
  if (kind === "code") {
    return (
      <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden>
        <path d="m8 7-5 5 5 5M16 7l5 5-5 5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    );
  }
  if (kind === "vars") {
    return (
      <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden>
        <path
          d="M7 4C5 4 5 6 5 8s-1 4-2 4c1 0 2 2 2 4s0 4 2 4M17 4c2 0 2 2 2 4s1 4 2 4c-1 0-2 2-2 4s0 4-2 4"
          strokeLinecap="round"
        />
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden>
      <path d="m5 8 4 4-4 4M12 17h7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function IconTrash() {
  return (
    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden>
      <path
        d="M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
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
  const [editing, setEditing] = useState<{ index: number; kind: RowEditor } | null>(null);

  useEffect(() => {
    if (!open) return;
    setTab("basic");
    setName(jobName);
    setDraft(parseBuildJobSpec(spec));
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
  const availableProjects = projects.filter((project) => !selectedIds.has(project.id));
  const canConfirm = name.trim().length > 0 && draft.registry_id.trim().length > 0;

  function toggleEditor(index: number, kind: RowEditor) {
    setEditing((current) => (current?.index === index && current.kind === kind ? null : { index, kind }));
  }

  function confirm() {
    onConfirm(name.trim(), buildJobSpecToRecord({ ...draft, registry_id: draft.registry_id.trim() }));
  }

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
    setEditing(null);
    setDraft((current) => ({
      ...current,
      project_builds: current.project_builds.filter((row) => row.project_id !== projectId),
    }));
  }

  function renderEditor(index: number, kind: RowEditor, row: BuildServiceRow) {
    const selectedGitSecretId = row.secret_id || projectGitSecretId(projectById.get(row.project_id));
    if (kind === "code") {
      return (
        <div className="build-cfg-editor-grid">
          <label className="build-cfg-editor-field">
            <span>代码库</span>
            <input
              className="input"
              value={row.git_url}
              placeholder="https://git.example.com/group/repo.git"
              onChange={(e) => patchRow(index, { git_url: e.target.value })}
            />
          </label>
          <label className="build-cfg-editor-field">
            <span>仓库密钥</span>
            <select
              className="input"
              aria-label="仓库密钥"
              value={selectedGitSecretId}
              onChange={(e) => patchRow(index, { secret_id: e.target.value })}
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
          <div className="build-cfg-editor-wide">
            <BuildCodeRepoFields
              workspaceId={workspaceId}
              row={{ ...row, secret_id: selectedGitSecretId }}
              gitSecrets={gitSecrets}
              selectedSecretId={selectedGitSecretId}
              onPatch={(patch) => patchRow(index, patch)}
            />
          </div>
        </div>
      );
    }
    if (kind === "vars") {
      return (
        <div className="build-cfg-vars">
          {row.variables.length === 0 ? (
            <p className="build-cfg-editor-empty">尚未添加构建变量。</p>
          ) : null}
          {row.variables.map((variable, varIndex) => (
            <div key={varIndex} className="build-cfg-var-row">
              <input
                className="input"
                aria-label="变量名"
                placeholder="变量名"
                value={variable.key}
                onChange={(e) => {
                  const variables = row.variables.map((item, i) =>
                    i === varIndex ? { ...item, key: e.target.value } : item,
                  );
                  patchRow(index, { variables });
                }}
              />
              <input
                className="input"
                aria-label="变量值"
                placeholder="变量值"
                value={variable.value}
                onChange={(e) => {
                  const variables = row.variables.map((item, i) =>
                    i === varIndex ? { ...item, value: e.target.value } : item,
                  );
                  patchRow(index, { variables });
                }}
              />
              <button
                type="button"
                className="build-cfg-icon-btn is-danger"
                aria-label="删除变量"
                title="删除变量"
                onClick={() => patchRow(index, { variables: row.variables.filter((_, i) => i !== varIndex) })}
              >
                <IconTrash />
              </button>
            </div>
          ))}
          <button
            type="button"
            className="build-cfg-text-btn"
            onClick={() => patchRow(index, { variables: [...row.variables, { key: "", value: "" }] })}
          >
            + 添加变量
          </button>
        </div>
      );
    }
    return (
      <label className="build-cfg-editor-field">
        <span>构建脚本</span>
        <textarea
          className="input build-cfg-script"
          rows={8}
          value={row.script}
          placeholder="可选，自定义构建脚本"
          spellCheck={false}
          onChange={(e) => patchRow(index, { script: e.target.value })}
        />
      </label>
    );
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
          </>
        }
        footer={
          <div className="build-cfg-footer">
            <span className="build-cfg-footer-hint">{!canConfirm ? "请填写任务名称并选择镜像仓库" : null}</span>
            <button type="button" className="btn-secondary" onClick={onClose}>
              取消
            </button>
            <button type="button" className="btn btn-primary text-sm" disabled={!canConfirm} onClick={confirm}>
              确定
            </button>
          </div>
        }
      >
        {loadError ? <InlineAlert tone="error">{loadError}</InlineAlert> : null}

        {tab === "basic" ? (
          <div className="build-cfg">
            <section className="build-cfg-section">
              <label className="build-cfg-field">
                <span className="build-cfg-label">
                  任务名称<span className="build-cfg-required">*</span>
                </span>
                <input className="input" value={name} aria-label="任务名称" onChange={(e) => setName(e.target.value)} />
              </label>
              <label className="build-cfg-field">
                <span className="build-cfg-label">
                  镜像仓库<span className="build-cfg-required">*</span>
                </span>
                <span className="build-cfg-control">
                  <select
                    className="input"
                    aria-label="镜像仓库"
                    value={draft.registry_id}
                    disabled={loading}
                    onChange={(e) => setDraft((current) => ({ ...current, registry_id: e.target.value }))}
                  >
                    <option value="">{loading ? "加载中…" : "请选择镜像仓库"}</option>
                    {registries.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.service_url || item.display_name || item.name}
                      </option>
                    ))}
                  </select>
                  {registries.length === 0 && !loading ? (
                    <span className="build-cfg-help">工作空间还没有 Harbor 或 Docker Registry 密钥，请先在密钥管理中添加。</span>
                  ) : null}
                </span>
              </label>
            </section>

            <section className="build-cfg-section">
              <header className="build-cfg-section-head">
                <h3 className="build-cfg-section-title">
                  项目组件
                  {draft.project_builds.length > 0 ? (
                    <span className="build-cfg-count">{draft.project_builds.length}</span>
                  ) : null}
                </h3>
                {availableProjects.length > 0 ? (
                  <select
                    className="input build-cfg-add"
                    aria-label="添加项目组件"
                    value=""
                    onChange={(e) => addProject(e.target.value)}
                  >
                    <option value="">+ 添加项目</option>
                    {availableProjects.map((project) => (
                      <option key={project.id} value={project.id}>
                        {projectLabel(project)}
                      </option>
                    ))}
                  </select>
                ) : null}
              </header>

              {draft.project_builds.length === 0 ? (
                <div className="build-cfg-empty">
                  {projects.length === 0
                    ? "产品还没有关联项目，请先在「项目配置」中绑定项目。"
                    : "选择要构建的项目，每个项目会使用各自的 Jenkinsfile 模板构建镜像。"}
                </div>
              ) : (
                <ul className="build-cfg-list">
                  {draft.project_builds.map((row, index) => {
                    const project = projectById.get(row.project_id);
                    const names = new Set(jenkinsTemplates.map((item) => item.name));
                    if (row.build_name) names.add(row.build_name);
                    const repo = repoLabel(row.git_url);
                    const expanded = editing?.index === index ? editing.kind : null;
                    return (
                      <li key={row.project_id} className={`build-cfg-item${expanded ? " is-expanded" : ""}`}>
                        <div className="build-cfg-item-row">
                          <div className="build-cfg-item-name">
                            <span className="build-cfg-item-title">
                              {project ? projectLabel(project) : row.project_id}
                            </span>
                            <span className="build-cfg-item-sub" title={row.git_url || undefined}>
                              {repo ? `${repo} · ${row.branch || "main"}` : "未配置代码库"}
                            </span>
                          </div>
                          <select
                            className="input build-cfg-template"
                            aria-label="Jenkinsfile 模板"
                            title="Jenkinsfile 模板"
                            value={row.build_name}
                            onChange={(e) => patchRow(index, { build_name: e.target.value })}
                          >
                            <option value="">选择 Jenkinsfile 模板</option>
                            {[...names].map((item) => (
                              <option key={item} value={item}>
                                {item}
                              </option>
                            ))}
                          </select>
                          <div className="build-cfg-segment" role="group" aria-label="构建配置">
                            {EDITORS.map((item) => (
                              <button
                                key={item.kind}
                                type="button"
                                className={`build-cfg-segment-btn${expanded === item.kind ? " is-active" : ""}`}
                                aria-pressed={expanded === item.kind}
                                aria-label={`${item.label}配置`}
                                onClick={() => toggleEditor(index, item.kind)}
                              >
                                <EditorIcon kind={item.kind} />
                                {item.label}
                                {item.kind === "vars" && row.variables.length > 0 ? (
                                  <span className="build-cfg-segment-badge">{row.variables.length}</span>
                                ) : null}
                              </button>
                            ))}
                          </div>
                          <button
                            type="button"
                            className="build-cfg-icon-btn is-danger"
                            aria-label="移除项目"
                            title="移除项目"
                            onClick={() => removeProject(row.project_id)}
                          >
                            <IconTrash />
                          </button>
                        </div>
                        {expanded ? <div className="build-cfg-editor">{renderEditor(index, expanded, row)}</div> : null}
                      </li>
                    );
                  })}
                </ul>
              )}
            </section>
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
