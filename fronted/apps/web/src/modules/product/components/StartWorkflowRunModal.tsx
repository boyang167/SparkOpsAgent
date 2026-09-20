"use client";

import { useEffect, useMemo, useState } from "react";
import type { EnvironmentVM, ProductProjectVM, ProductWorkflowVM } from "@sparkops/contracts/product";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { secret, workspace } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import { DeployYamlCompareModal } from "./DeployYamlCompareModal";
import { deployImageRef, imageTagFromRef } from "./deploy-job-spec";
import { RunBranchField } from "./RunBranchField";
import {
  buildWorkflowRunForm,
  defaultDeployEnvironmentId,
  type RunFormDeployRow,
} from "./workflow-run-form";

interface StartWorkflowRunModalProps {
  open: boolean;
  workspaceId: string;
  workflow: ProductWorkflowVM | null;
  environments: EnvironmentVM[];
  projects?: ProductProjectVM[];
  busy?: boolean;
  onClose: () => void;
  onSubmit: (input: {
    branch?: string;
    branch_overrides?: Record<string, string>;
    ref_type_overrides?: Record<string, string>;
    environment_overrides?: Record<string, string>;
    variables?: Record<string, string>;
  }) => void;
}

export function StartWorkflowRunModal({
  open,
  workspaceId,
  workflow,
  environments,
  projects = [],
  busy,
  onClose,
  onSubmit,
}: StartWorkflowRunModalProps) {
  const [gitProjects, setGitProjects] = useState<ProjectVM[]>([]);
  const [registries, setRegistries] = useState<SecretVM[]>([]);
  const form = useMemo(
    () => buildWorkflowRunForm(workflow?.stages, projects, gitProjects),
    [workflow?.stages, projects, gitProjects],
  );
  const runtimeVariables = useMemo(
    () => (workflow?.variables ?? []).filter((item) => item.runtime_input && item.key.trim()),
    [workflow],
  );
  const deployJobs = useMemo(() => form.stages.flatMap((stage) => stage.deployJobs), [form.stages]);
  const hasBuild = form.stages.some((stage) => stage.buildRows.length > 0);

  const [branches, setBranches] = useState<Record<string, string>>({});
  const [refTypes, setRefTypes] = useState<Record<string, "branch" | "tag" | "pr">>({});
  const [environmentId, setEnvironmentId] = useState("");
  const [variables, setVariables] = useState<Record<string, string>>({});
  const [yamlCompare, setYamlCompare] = useState<RunFormDeployRow | null>(null);

  useEffect(() => {
    if (!open || !workspaceId) return;
    let cancelled = false;
    void Promise.all([
      workspace.listProjects(workspaceId),
      secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
    ]).then(([projectItems, secretItems]) => {
      if (cancelled) return;
      setGitProjects(projectItems);
      setRegistries(
        secretItems.filter((item) => {
          const type = item.service_type.toLowerCase();
          return type.includes("harbor") || type.includes("registry") || type.includes("docker");
        }),
      );
    });
    return () => {
      cancelled = true;
    };
  }, [open, workspaceId]);

  useEffect(() => {
    if (!open || !workflow) return;
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
  }, [open, workflow?.id, gitProjects]);

  useEffect(() => {
    if (!open || !workflow) return;
    setEnvironmentId(defaultDeployEnvironmentId(workflow.stages) || environments[0]?.id || "");
  }, [open, workflow, environments]);

  if (!open || !workflow) return null;

  const registryOptions = registries.map((item) => ({ id: item.id, serviceUrl: item.service_url }));
  const projectOptions = gitProjects.map((item) => ({
    id: item.id,
    name: item.name,
    namespace: item.namespace,
    displayName: item.display_name,
  }));

  function buildJobSpec(jobName: string): Record<string, unknown> | undefined {
    for (const stage of workflow.stages ?? []) {
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
  const missingBranch = form.stages.some((stage) =>
    stage.buildRows.some((row) => !(branches[row.key] ?? row.branch).trim()),
  );
  const missingEnv = deployJobs.length > 0 && !environmentId;

  function submit() {
    const environment_overrides =
      deployJobs.length > 0 && environmentId
        ? Object.fromEntries(deployJobs.map((job) => [job.jobName, environmentId]))
        : undefined;
    const branch_overrides: Record<string, string> = {};
    const ref_type_overrides: Record<string, string> = {};
    for (const stage of form.stages) {
      for (const row of stage.buildRows) {
        const value = (branches[row.key] ?? row.branch).trim();
        if (!value) continue;
        const key = row.projectId ? `${row.jobName}:${row.projectId}` : row.jobName;
        branch_overrides[key] = value;
        ref_type_overrides[key] = refTypes[row.key] ?? row.refType;
      }
    }
    const firstBranch = Object.values(branch_overrides)[0];
    onSubmit({
      branch: hasBuild ? firstBranch : undefined,
      branch_overrides: Object.keys(branch_overrides).length ? branch_overrides : undefined,
      ref_type_overrides: Object.keys(ref_type_overrides).length ? ref_type_overrides : undefined,
      environment_overrides,
      variables: runtimeVariables.length ? variables : undefined,
    });
  }

  return (
    <>
    <Modal
      open={open}
      onClose={onClose}
      title="执行工作流"
      description={workflow.name}
      size="full"
      headerExtra={
        form.pills.length > 0 ? (
          <nav className="wf-run-pills" aria-label="阶段">
            {form.pills.map((pill) => (
              <button
                key={pill.id}
                type="button"
                className="wf-run-pill"
                onClick={() => document.getElementById(pill.id)?.scrollIntoView({ behavior: "smooth", block: "start" })}
              >
                {pill.name}
              </button>
            ))}
          </nav>
        ) : null
      }
      footer={
        <>
          <button
            type="button"
            className="wf-run-submit"
            disabled={busy || missingVariable || missingEnv || missingBranch}
            title={
              missingEnv ? "请选择部署环境" : missingBranch ? "请选择分支" : missingVariable ? "请填写运行时变量" : undefined
            }
            onClick={submit}
          >
            {busy ? "启动中…" : "执行"}
          </button>
          <button type="button" className="btn btn-ghost text-sm" onClick={onClose} disabled={busy}>
            取消
          </button>
        </>
      }
    >
      <div className="wf-run-body">
        {form.stages.length === 0 ? (
          <p className="wf-run-hint wf-run-empty">
            当前工作流尚未配置阶段，将按定义直接执行。
          </p>
        ) : null}

        {form.stages.map((stage) => (
          <section key={stage.id} id={stage.id} className="wf-run-stage">
            <h3 className="wf-run-stage-title">{stage.name}</h3>

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
          </section>
        ))}

        {runtimeVariables.length > 0 ? (
          <section className="wf-run-stage" id="wf-run-variables">
            <h3 className="wf-run-stage-title">运行时变量</h3>
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
          </section>
        ) : null}
      </div>
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
