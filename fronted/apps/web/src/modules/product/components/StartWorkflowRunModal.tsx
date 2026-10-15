"use client";

import { useEffect, useMemo, useState } from "react";
import type {
  EnvironmentVM,
  ProductProjectVM,
  ProductWorkflowVM,
  StartProductWorkflowRunCommand,
} from "@sparkops/contracts/product";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { runtimeVersionTag } from "./create-version-job-spec";
import { DeployYamlCompareModal } from "./DeployYamlCompareModal";
import { deployImageRef, imageTagFromRef } from "./deploy-job-spec";
import { RunBranchField } from "./RunBranchField";
import { runModalExtraInputs } from "./release-plan-runtime-inputs";
import {
  buildWorkflowRunForm,
  defaultDeployEnvironmentId,
  type RunFormDeployRow,
} from "./workflow-run-form";
import { peekWorkspaceRunAssets, prefetchWorkspaceRunAssets } from "./workflow-run-prefetch";
import { WorkflowRunFormSkeleton } from "./WorkflowRunFormSkeleton";

interface StartWorkflowRunModalProps {
  open?: boolean;
  presentation?: "modal" | "page" | "embedded";
  workspaceId: string;
  workflow: ProductWorkflowVM | null;
  environments: EnvironmentVM[];
  projects?: ProductProjectVM[];
  busy?: boolean;
  loading?: boolean;
  workflowName?: string;
  submitError?: string | null;
  onClose: () => void;
  onSubmit: (input: {
    branch?: string;
    branch_overrides?: Record<string, string>;
    ref_type_overrides?: Record<string, string>;
    environment_overrides?: Record<string, string>;
    version_tag_overrides?: Record<string, string>;
    variables?: Record<string, string>;
    remark?: string;
    stage_indices?: number[];
  }) => void;
}

export function StartWorkflowRunModal({
  open = false,
  presentation = "modal",
  workspaceId,
  workflow,
  environments,
  projects = [],
  busy,
  loading = false,
  workflowName,
  submitError,
  onClose,
  onSubmit,
}: StartWorkflowRunModalProps) {
  const active =
    presentation === "page" || presentation === "embedded" ? Boolean(workflow) : open;
  const initialAssets = peekWorkspaceRunAssets(workspaceId);
  const [gitProjects, setGitProjects] = useState<ProjectVM[]>(() => initialAssets?.gitProjects ?? []);
  const [registries, setRegistries] = useState<SecretVM[]>(() => initialAssets?.registries ?? []);
  const form = useMemo(
    () => buildWorkflowRunForm(workflow?.stages, projects, gitProjects),
    [workflow?.stages, projects, gitProjects],
  );
  const runtimeVariables = useMemo(() => {
    const declared = (workflow?.variables ?? []).filter((item) => item.runtime_input && item.key.trim());
    const extra = runModalExtraInputs(workflow?.runtime_inputs)
      .filter((input) => !declared.some((item) => item.key === input.key))
      .map((input) => ({
        id: `runtime-input-${input.key}`,
        key: input.key,
        value: "",
        description: input.label,
        runtime_input: true,
        secret: false,
        vault_path: "",
      }));
    return [...declared, ...extra];
  }, [workflow]);
  const [branches, setBranches] = useState<Record<string, string>>({});
  const [refTypes, setRefTypes] = useState<Record<string, "branch" | "tag" | "pr">>({});
  const [environmentId, setEnvironmentId] = useState("");
  const [variables, setVariables] = useState<Record<string, string>>({});
  const [yamlCompare, setYamlCompare] = useState<RunFormDeployRow | null>(null);
  const [visibleStageIds, setVisibleStageIds] = useState<Set<string>>(new Set());
  const [collapsedStageIds, setCollapsedStageIds] = useState<Set<string>>(new Set());
  const [remark, setRemark] = useState("");
  const [versionTags, setVersionTags] = useState<Record<string, string>>({});

  const stageSelectors = useMemo(() => {
    const items = form.pills.map((pill) => ({ id: pill.id, name: pill.name }));
    if (runtimeVariables.length > 0) {
      items.push({ id: "wf-run-variables", name: "运行时变量" });
    }
    return items;
  }, [form.pills, runtimeVariables.length]);

  const selectedStages = useMemo(
    () => form.stages.filter((stage) => visibleStageIds.has(stage.id)),
    [form.stages, visibleStageIds],
  );
  const selectedDeployJobs = useMemo(
    () => selectedStages.flatMap((stage) => stage.deployJobs),
    [selectedStages],
  );
  const selectedHasBuild = selectedStages.some((stage) => stage.buildRows.length > 0);

  useEffect(() => {
    if (!active || !workspaceId) return;
    let cancelled = false;
    void prefetchWorkspaceRunAssets(workspaceId).then(({ gitProjects: projectItems, registries: registryItems }) => {
      if (cancelled) return;
      setGitProjects(projectItems);
      setRegistries(registryItems);
    });
    return () => {
      cancelled = true;
    };
  }, [active, workspaceId]);

  useEffect(() => {
    if (!active || !workflow) return;
    const nextBranches: Record<string, string> = {};
    const nextTypes: Record<string, "branch" | "tag"> = {};
    for (const stage of form.stages) {
      for (const row of stage.buildRows) {
        nextBranches[row.key] = row.branch || "main";
        nextTypes[row.key] = row.refType;
      }
    }
    setBranches(nextBranches);
    setRefTypes(nextTypes);
    const next: Record<string, string> = {};
    for (const item of runtimeVariables) next[item.key] = item.value ?? "";
    setVariables(next);
    // 只在打开或切换工作流、补全仓库信息后重置。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, workflow?.id, gitProjects]);

  useEffect(() => {
    if (!active || !workflow) return;
    setEnvironmentId(defaultDeployEnvironmentId(workflow.stages) || environments[0]?.id || "");
  }, [active, workflow, environments]);

  useEffect(() => {
    if (!active || !workflow) return;
    setVisibleStageIds(new Set(stageSelectors.map((item) => item.id)));
    setCollapsedStageIds(new Set());
  }, [active, workflow?.id, stageSelectors]);

  const modalReady = Boolean(workflow) && !loading;

  if (presentation === "page") {
    if (!active || !workflow) return null;
  } else if (presentation === "embedded") {
    if (!active) return loading ? <WorkflowRunFormSkeleton /> : null;
  } else if (!open) {
    return null;
  }

  const registryOptions = registries.map((item) => ({ id: item.id, serviceUrl: item.service_url }));
  const projectOptions = gitProjects.map((item) => ({
    id: item.id,
    name: item.name,
    namespace: item.namespace,
    displayName: item.display_name,
  }));

  function buildJobSpec(jobName: string): Record<string, unknown> | undefined {
    for (const stage of workflow?.stages ?? []) {
      const job = stage.jobs.find((item) => item.name === jobName && item.type === "sparkops-build");
      if (job?.spec) return job.spec;
    }
    return undefined;
  }

  function resolveImageRef(row: RunFormDeployRow): string {
    const branch = (branches[row.buildKey] ?? "main").trim();
    return deployImageRef(buildJobSpec(row.buildJobName), row.projectId, branch, registryOptions, projectOptions);
  }

  const yamlCompareTag = yamlCompare
    ? imageTagFromRef((branches[yamlCompare.buildKey] ?? "main").trim())
    : "";

  const missingVariable = runtimeVariables.some((item) => !(variables[item.key] ?? "").trim());
  const missingBranch = selectedStages.some((stage) =>
    stage.buildRows.some((row) => !(branches[row.key] ?? row.branch).trim()),
  );
  const missingEnv = selectedDeployJobs.length > 0 && !environmentId;
  const missingVersionTag = selectedStages.some((stage) =>
    stage.versionJobs.some((job) => !runtimeVersionTag(versionTags[job.jobName] ?? job.versionTag)),
  );

  function toggleStageVisibility(stageId: string) {
    setVisibleStageIds((prev) => {
      const next = new Set(prev);
      if (next.has(stageId)) {
        if (next.size <= 1) return prev;
        next.delete(stageId);
        return next;
      }
      next.add(stageId);
      requestAnimationFrame(() => {
        document.getElementById(stageId)?.scrollIntoView({ behavior: "smooth", block: "start" });
      });
      return next;
    });
  }

  function toggleStageCollapsed(stageId: string) {
    setCollapsedStageIds((prev) => {
      const next = new Set(prev);
      if (next.has(stageId)) next.delete(stageId);
      else next.add(stageId);
      return next;
    });
  }

  function parseStageIndex(stageId: string): number | null {
    const match = /^wf-run-stage-(\d+)$/.exec(stageId);
    return match ? Number(match[1]) : null;
  }

  function renderStageBar(className?: string) {
    if (stageSelectors.length < 2) return null;
    const skipped = stageSelectors.filter((item) => !visibleStageIds.has(item.id));
    const summary =
      skipped.length === 0
        ? `全部 ${stageSelectors.length} 个阶段将执行，点击可取消选择`
        : `将跳过 ${skipped.length} 个阶段：${skipped.map((item) => item.name).join("、")}`;
    return (
      <div className={`wf-run-stagebar${className ? ` ${className}` : ""}`}>
        <div className="wf-run-stagebar-copy">
          <span className="wf-run-stagebar-title">执行阶段</span>
          <span className={`wf-run-stagebar-desc${skipped.length > 0 ? " is-skipping" : ""}`} aria-live="polite">
            {summary}
          </span>
        </div>
        <nav className="wf-run-pills" aria-label="选择执行阶段">
          {stageSelectors.map((item) => {
            const active = visibleStageIds.has(item.id);
            const locked = active && visibleStageIds.size <= 1;
            return (
              <button
                key={item.id}
                type="button"
                className={`wf-run-pill${active ? " is-active" : ""}`}
                aria-pressed={active}
                title={locked ? "至少保留一个阶段" : active ? "点击跳过该阶段" : "点击执行该阶段"}
                onClick={() => toggleStageVisibility(item.id)}
              >
                <span className="wf-run-pill-check" aria-hidden />
                {item.name}
              </button>
            );
          })}
        </nav>
      </div>
    );
  }

  function submit() {
    const environment_overrides =
      selectedDeployJobs.length > 0 && environmentId
        ? Object.fromEntries(selectedDeployJobs.map((job) => [job.jobName, environmentId]))
        : undefined;
    const branch_overrides: Record<string, string> = {};
    const ref_type_overrides: Record<string, string> = {};
    for (const stage of selectedStages) {
      for (const row of stage.buildRows) {
        const value = (branches[row.key] ?? row.branch).trim();
        if (!value) continue;
        const key = row.projectId ? `${row.jobName}:${row.projectId}` : row.jobName;
        branch_overrides[key] = value;
        ref_type_overrides[key] = refTypes[row.key] ?? row.refType;
      }
    }
    const firstBranch = Object.values(branch_overrides)[0];
    const trimmedRemark = remark.trim();
    const version_tag_overrides: Record<string, string> = {};
    for (const stage of selectedStages) {
      for (const job of stage.versionJobs) {
        const value = (versionTags[job.jobName] ?? job.versionTag).trim();
        if (value) version_tag_overrides[job.jobName] = value;
      }
    }
    const stage_indices = selectedStages
      .map((stage) => parseStageIndex(stage.id))
      .filter((index): index is number => index != null)
      .sort((a, b) => a - b);
    const runAllStages = stage_indices.length === form.stages.length;
    onSubmit({
      branch: selectedHasBuild ? firstBranch : undefined,
      branch_overrides: Object.keys(branch_overrides).length ? branch_overrides : undefined,
      ref_type_overrides: Object.keys(ref_type_overrides).length ? ref_type_overrides : undefined,
      environment_overrides,
      version_tag_overrides: Object.keys(version_tag_overrides).length ? version_tag_overrides : undefined,
      variables: runtimeVariables.length ? variables : undefined,
      remark: trimmedRemark || undefined,
      stage_indices: runAllStages ? undefined : stage_indices,
    });
  }

  const footerActions = modalReady ? (
    <>
      <button
        type="button"
        className="wf-run-submit"
        disabled={busy || missingVariable || missingEnv || missingBranch || missingVersionTag}
        title={
          missingEnv
            ? "请选择部署环境"
            : missingBranch
              ? "请选择分支"
              : missingVersionTag
                ? "请填写交付版本号（不能包含 {{…}} 占位符）"
                : missingVariable
                  ? "请填写运行时变量"
                  : undefined
        }
        onClick={submit}
      >
        {busy ? "启动中…" : "执行"}
      </button>
      <button type="button" className="btn btn-ghost text-sm" onClick={onClose} disabled={busy}>
        取消
      </button>
    </>
  ) : (
    <>
      <button type="button" className="wf-run-submit" disabled>
        执行
      </button>
      <button type="button" className="btn btn-ghost text-sm" onClick={onClose}>
        取消
      </button>
    </>
  );

  const formContent = (
      <div className="wf-run-body">
        {presentation === "modal" ? renderStageBar("is-sticky") : null}
        {submitError ? (
          <div className="wf-run-block">
            <InlineAlert tone="error">{submitError}</InlineAlert>
          </div>
        ) : null}
        {form.stages.length === 0 ? (
          <p className="wf-run-hint wf-run-empty">
            当前工作流尚未配置阶段，将按定义直接执行。
          </p>
        ) : null}

        {form.stages.map((stage) => {
          if (!visibleStageIds.has(stage.id)) return null;
          const collapsed = collapsedStageIds.has(stage.id);
          return (
          <section key={stage.id} id={stage.id} className={`wf-run-stage${collapsed ? " is-collapsed" : ""}`}>
            <button
              type="button"
              className="wf-run-stage-head"
              aria-expanded={!collapsed}
              aria-controls={`${stage.id}-panel`}
              onClick={() => toggleStageCollapsed(stage.id)}
            >
              <span className="wf-run-stage-chevron" aria-hidden />
              <span className="wf-run-stage-name">{stage.name}</span>
            </button>

            {!collapsed ? (
            <div id={`${stage.id}-panel`} className="wf-run-stage-panel">
            {stage.buildRows.length > 0 ? (
              <div className="wf-run-block">
                <table className="wf-run-table">
                  <thead>
                    <tr>
                      <th>项目名称</th>
                      <th>代码库</th>
                      <th>分支</th>
                    </tr>
                  </thead>
                  <tbody>
                    {stage.buildRows.map((row) => (
                      <tr key={row.key}>
                        <td>{row.projectName}</td>
                        <td className="wf-run-muted">{row.repoLabel}</td>
                        <td>
                          <RunBranchField
                            workspaceId={workspaceId}
                            gitUrl={row.gitUrl}
                            secretId={row.secretId}
                            value={branches[row.key] ?? row.branch}
                            refType={refTypes[row.key] ?? row.refType}
                            label={`${row.projectName} 分支`}
                            onChange={(next, nextType) => {
                              setBranches((prev) => ({ ...prev, [row.key]: next }));
                              setRefTypes((prev) => ({ ...prev, [row.key]: nextType }));
                            }}
                          />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}

            {stage.versionJobs.length > 0 ? (
              <div className="wf-run-block">
                <table className="wf-run-table">
                  <thead>
                    <tr>
                      <th>任务</th>
                      <th>版本号</th>
                    </tr>
                  </thead>
                  <tbody>
                    {stage.versionJobs.map((job) => (
                      <tr key={job.jobName}>
                        <td>
                          {job.jobName}
                          {job.fromDeploy ? <span className="text-muted text-xs">（交付版本部署）</span> : null}
                        </td>
                        <td>
                          <input
                            className="input"
                            value={versionTags[job.jobName] ?? job.versionTag}
                            placeholder="例如 v1.2.0"
                            onChange={(event) =>
                              setVersionTags((prev) => ({ ...prev, [job.jobName]: event.target.value }))
                            }
                          />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}

            {stage.deployJobs.length > 0 ? (
              <div className="wf-run-block">
                <label className="wf-run-field">
                  <span className="wf-run-field-label">环境</span>
                  <select
                    className="input wf-run-select"
                    value={environmentId}
                    onChange={(e) => setEnvironmentId(e.target.value)}
                  >
                    <option value="">请选择环境</option>
                    {environments.map((env) => (
                      <option key={env.id} value={env.id}>
                        {env.name}
                      </option>
                    ))}
                  </select>
                </label>
                <table className="wf-run-table wf-run-deploy-table">
                  <thead>
                    <tr>
                      <th className="wf-run-col-project">项目名称</th>
                      <th className="wf-run-col-image">镜像</th>
                      <th className="wf-run-col-action">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {stage.deployJobs.flatMap((job) =>
                      job.rows.map((row, index) => {
                        const imageRef = resolveImageRef(row);
                        const canCompare = Boolean(environmentId && row.projectId);
                        return (
                          <tr key={`${job.jobName}-${row.projectId || row.projectName}-${index}`}>
                            <td className="wf-run-col-project">{row.projectName}</td>
                            <td className="wf-run-col-image">
                              <span className="wf-run-image-ref mono" title={imageRef}>
                                {imageRef}
                              </span>
                            </td>
                            <td className="wf-run-col-action">
                              {canCompare ? (
                                <button
                                  type="button"
                                  className="wf-run-yaml-link"
                                  onClick={() => setYamlCompare(row)}
                                >
                                  YAML 比对
                                </button>
                              ) : (
                                <span className="wf-run-action-hint">请选择环境</span>
                              )}
                            </td>
                          </tr>
                        );
                      }),
                    )}
                  </tbody>
                </table>
              </div>
            ) : null}

            {stage.otherJobs.length > 0 ? (
              <div className="wf-run-block">
                <table className="wf-run-table">
                  <thead>
                    <tr>
                      <th>任务</th>
                      <th>类型</th>
                      <th>说明</th>
                    </tr>
                  </thead>
                  <tbody>
                    {stage.otherJobs.map((job) => (
                      <tr key={job.jobName}>
                        <td>{job.jobName}</td>
                        <td>{job.typeLabel}</td>
                        <td className="wf-run-muted">按编排定义执行</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}
            </div>
            ) : null}
          </section>
        );
        })}

        {runtimeVariables.length > 0 && visibleStageIds.has("wf-run-variables") ? (
          <section
            className={`wf-run-stage${collapsedStageIds.has("wf-run-variables") ? " is-collapsed" : ""}`}
            id="wf-run-variables"
          >
            <button
              type="button"
              className="wf-run-stage-head"
              aria-expanded={!collapsedStageIds.has("wf-run-variables")}
              aria-controls="wf-run-variables-panel"
              onClick={() => toggleStageCollapsed("wf-run-variables")}
            >
              <span className="wf-run-stage-chevron" aria-hidden />
              <span className="wf-run-stage-name">运行时变量</span>
            </button>
            {!collapsedStageIds.has("wf-run-variables") ? (
              <div id="wf-run-variables-panel" className="wf-run-stage-panel">
                <div className="wf-run-block stack-sm">
                  {runtimeVariables.map((item) => (
                    <label key={item.id} className="wf-run-field">
                      <span className="wf-run-field-label">{item.key}</span>
                      <input
                        className="input wf-run-input"
                        value={variables[item.key] ?? ""}
                        onChange={(e) => setVariables((prev) => ({ ...prev, [item.key]: e.target.value }))}
                        placeholder={item.description || item.key}
                      />
                    </label>
                  ))}
                </div>
              </div>
            ) : null}
          </section>
        ) : null}

        <section className="wf-run-remark">
          <label className="wf-run-field" htmlFor="wf-run-remark-input">
            <span className="wf-run-field-label">备注</span>
            <textarea
              id="wf-run-remark-input"
              className="input wf-run-remark-input"
              value={remark}
              onChange={(e) => setRemark(e.target.value)}
              placeholder="记录本次执行说明，便于后续追溯"
              rows={3}
              maxLength={500}
            />
          </label>
        </section>

      </div>
  );

  if (presentation === "embedded") {
    return (
      <>
        <div className="wf-run-embedded">
          {modalReady ? renderStageBar("wf-run-embedded-stagebar") : null}
          <div className="wf-run-embedded-body">
            {modalReady ? formContent : <WorkflowRunFormSkeleton />}
          </div>
          <footer className="wf-run-embedded-footer">{footerActions}</footer>
        </div>
        <DeployYamlCompareModal
          open={yamlCompare !== null}
          workspaceId={workspaceId}
          projectId={yamlCompare?.projectId ?? ""}
          projectName={yamlCompare?.projectName ?? ""}
          environmentId={environmentId}
          imageTag={yamlCompareTag}
          imageRef={yamlCompare ? resolveImageRef(yamlCompare) : ""}
          onClose={() => setYamlCompare(null)}
        />
      </>
    );
  }

  if (presentation === "page") {
    return (
      <>
        <div className="wf-run-page">
          <header className="wf-run-page-head">
            <div className="wf-run-page-head-main">
              <h1 className="wf-run-page-title">执行工作流</h1>
              <p className="wf-run-page-desc">{workflow.name}</p>
            </div>
          </header>
          <section className="wf-run-page-card">
            {renderStageBar()}
            {formContent}
          </section>
          <footer className="wf-run-page-footer">{footerActions}</footer>
        </div>
        <DeployYamlCompareModal
          open={yamlCompare !== null}
          workspaceId={workspaceId}
          projectId={yamlCompare?.projectId ?? ""}
          projectName={yamlCompare?.projectName ?? ""}
          environmentId={environmentId}
          imageTag={yamlCompareTag}
          imageRef={yamlCompare ? resolveImageRef(yamlCompare) : ""}
          onClose={() => setYamlCompare(null)}
        />
      </>
    );
  }

  return (
    <>
    <Modal
      open={open}
      onClose={onClose}
      title="执行工作流"
      description={workflow?.name ?? workflowName}
      size="full"
      footer={footerActions}
    >
      {modalReady ? formContent : <WorkflowRunFormSkeleton />}
    </Modal>
    <DeployYamlCompareModal
      open={yamlCompare !== null}
      workspaceId={workspaceId}
      projectId={yamlCompare?.projectId ?? ""}
      projectName={yamlCompare?.projectName ?? ""}
      environmentId={environmentId}
      imageTag={yamlCompareTag}
      imageRef={yamlCompare ? resolveImageRef(yamlCompare) : ""}
      onClose={() => setYamlCompare(null)}
    />
    </>
  );
}
