"use client";

import { useEffect, useMemo, useState } from "react";
import type { EnvironmentVM, ProductProjectVM, ProductWorkflowVM } from "@sparkops/contracts/product";
import { product } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
import { ReleasePlanProjectBranches } from "./ReleasePlanProjectBranches";
import {
  defaultBranchForWorkflow,
  defaultEnvironmentIdForWorkflow,
  resolveWorkflowDetails,
  workflowHasBuildJobs,
  workflowProjectIds,
  workflowProjectSources,
  type ProjectRefOverrides,
  type WorkflowJobOptions,
} from "./release-plan-workflow-params";

interface ReleasePlanAddWorkflowModalProps {
  workspaceId: string;
  productId: string;
  open: boolean;
  workflows: ProductWorkflowVM[];
  environments: EnvironmentVM[];
  projects: ProductProjectVM[];
  busy?: boolean;
  onClose: () => void;
  onSubmit: (
    workflowId: string,
    options: WorkflowJobOptions,
  ) => void | Promise<void>;
}

function workflowHasDeployJobs(workflow: ProductWorkflowVM | null) {
  return (workflow?.stages ?? []).some((stage) =>
    (stage.jobs ?? []).some((job) => job.type === "sparkops-deploy" || job.type === "sparkops-deploy-version"),
  );
}

export function ReleasePlanAddWorkflowModal({
  workspaceId,
  productId,
  open,
  workflows,
  environments,
  projects,
  busy,
  onClose,
  onSubmit,
}: ReleasePlanAddWorkflowModalProps) {
  const [workflowId, setWorkflowId] = useState("");
  const [environmentId, setEnvironmentId] = useState("");
  const [branch, setBranch] = useState("main");
  const [projectIds, setProjectIds] = useState<string[]>([]);
  const [refOverrides, setRefOverrides] = useState<ProjectRefOverrides>({});
  const [workflowDetail, setWorkflowDetail] = useState<ProductWorkflowVM | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  const workflowSummary = useMemo(
    () => workflows.find((item) => item.id === workflowId) ?? null,
    [workflows, workflowId],
  );
  const workflow = workflowDetail?.id === workflowId ? workflowDetail : workflowSummary;
  const needsEnvironment = workflowHasDeployJobs(workflow);
  const needsBranch = workflowHasBuildJobs(workflow);
  const candidateProjectIds = useMemo(() => workflowProjectIds(workflow), [workflow]);

  useEffect(() => {
    if (open) setWorkflowId((current) => current || workflows[0]?.id || "");
  }, [open, workflows]);

  useEffect(() => {
    if (!open || !workflowSummary) return;
    let cancelled = false;
    setWorkflowDetail(null);
    setDetailError(null);
    setDetailLoading(true);
    void resolveWorkflowDetails(workflowSummary, (id) => product.getProductWorkflow(workspaceId, productId, id))
      .then((detail) => {
        if (!cancelled) setWorkflowDetail(detail);
      })
      .catch(() => {
        if (!cancelled) setDetailError("工作流详情加载失败，请重试。");
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, workflowSummary, workspaceId, productId]);

  useEffect(() => {
    const defaultBranch = defaultBranchForWorkflow(workflow);
    setEnvironmentId(defaultEnvironmentIdForWorkflow(workflow, environments));
    setBranch(defaultBranch);
    setProjectIds(candidateProjectIds);
    // 默认分支会覆盖所有项目，工作流里与之不同的项目分支 / Tag 需保留为单独指定
    const overrides: ProjectRefOverrides = {};
    for (const [projectId, source] of Object.entries(workflowProjectSources(workflow))) {
      if (source.ref && (source.ref.ref !== defaultBranch || source.ref.type !== "branch")) {
        overrides[projectId] = source.ref;
      }
    }
    setRefOverrides(overrides);
  }, [workflow, environments, candidateProjectIds]);

  const projectSources = useMemo(() => workflowProjectSources(workflow), [workflow]);

  const canSubmit =
    Boolean(workflowId) &&
    !detailLoading &&
    !detailError &&
    (!needsEnvironment || Boolean(environmentId)) &&
    (projectIds.length > 0 || (!needsBranch && candidateProjectIds.length === 0));

  return (
    <Modal
      open={open}
      size={needsBranch ? "lg" : "md"}
      onClose={onClose}
      title="添加工作流"
      description="选择要在此步骤执行的工作流，并指定本次发布的参数。"
      footer={
        <ReleasePlanModalFooter
          onCancel={onClose}
          submitLabel="添加"
          submittingLabel="添加中…"
          busy={busy}
          disabled={!canSubmit}
          onSubmit={() =>
            void onSubmit(workflowId, {
              ...(needsEnvironment ? { environmentId } : {}),
              ...(needsBranch ? { branch: branch.trim() || "main" } : {}),
              ...(projectIds.length > 0 ? { projectIds } : {}),
              ...(needsBranch ? { refOverrides } : {}),
            })
          }
        />
      }
    >
      <div className="stack-sm">
        {detailError ? <InlineAlert tone="error">{detailError}</InlineAlert> : null}
        <label className="form-field stack-xs">
          <span className="form-label">工作流</span>
          <select className="input" value={workflowId} onChange={(e) => setWorkflowId(e.target.value)}>
            {workflows.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </label>
        {detailLoading ? <span className="text-sm text-muted-foreground" role="status">正在加载工作流配置…</span> : null}
        {needsEnvironment ? (
          <label className="form-field stack-xs">
            <span className="form-label">部署环境</span>
            {environments.length === 0 ? (
              <span className="text-sm text-muted-foreground">请先在「环境」Tab 创建环境</span>
            ) : (
              <select className="input" value={environmentId} onChange={(e) => setEnvironmentId(e.target.value)}>
                {environments.map((env) => (
                  <option key={env.id} value={env.id}>
                    {env.name}
                  </option>
                ))}
              </select>
            )}
          </label>
        ) : null}
        {needsBranch ? (
          <label className="form-field stack-xs">
            <span className="form-label">默认分支</span>
            <input
              className="input font-mono"
              value={branch}
              placeholder="main"
              spellCheck={false}
              onChange={(e) => setBranch(e.target.value)}
            />
            <span className="text-xs text-muted-foreground">未单独指定分支的项目都使用此分支。</span>
          </label>
        ) : null}
        {projectIds.length > 0 || (needsBranch && projects.length > 0) ? (
          <div className="form-field stack-xs">
            <span className="form-label">执行项目</span>
            <ReleasePlanProjectBranches
              nested
              workspaceId={workspaceId}
              projects={projects}
              selectedIds={projectIds}
              refOverrides={refOverrides}
              defaultRefFor={() => ({ ref: branch.trim() || "main", type: "branch" })}
              workflowSourceFor={(projectId) => projectSources[projectId]}
              buildable={needsBranch}
              editable={!busy}
              onChange={(value) => {
                setProjectIds(value.projectIds);
                setRefOverrides(value.refOverrides);
              }}
            />
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
