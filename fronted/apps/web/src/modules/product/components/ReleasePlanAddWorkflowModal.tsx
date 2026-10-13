"use client";

import { useEffect, useMemo, useState } from "react";
import type { EnvironmentVM, ProductWorkflowVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";
import {
  defaultBranchForWorkflow,
  defaultEnvironmentIdForWorkflow,
  workflowHasBuildJobs,
} from "./release-plan-workflow-params";

interface ReleasePlanAddWorkflowModalProps {
  open: boolean;
  workflows: ProductWorkflowVM[];
  environments: EnvironmentVM[];
  busy?: boolean;
  onClose: () => void;
  onSubmit: (workflowId: string, options: { environmentId?: string; branch?: string }) => void | Promise<void>;
}

function workflowHasDeployJobs(workflow: ProductWorkflowVM | null) {
  return (workflow?.stages ?? []).some((stage) =>
    (stage.jobs ?? []).some((job) => job.type === "sparkops-deploy" || job.type === "sparkops-deploy-version"),
  );
}

export function ReleasePlanAddWorkflowModal({
  open,
  workflows,
  environments,
  busy,
  onClose,
  onSubmit,
}: ReleasePlanAddWorkflowModalProps) {
  const [workflowId, setWorkflowId] = useState("");
  const [environmentId, setEnvironmentId] = useState("");
  const [branch, setBranch] = useState("main");

  const workflow = useMemo(() => workflows.find((item) => item.id === workflowId) ?? null, [workflows, workflowId]);
  const needsEnvironment = workflowHasDeployJobs(workflow);
  const needsBranch = workflowHasBuildJobs(workflow);

  useEffect(() => {
    if (open) setWorkflowId((current) => current || workflows[0]?.id || "");
  }, [open, workflows]);

  useEffect(() => {
    setEnvironmentId(defaultEnvironmentIdForWorkflow(workflow, environments));
    setBranch(defaultBranchForWorkflow(workflow));
  }, [workflow, environments]);

  const canSubmit = Boolean(workflowId) && (!needsEnvironment || Boolean(environmentId));

  return (
    <Modal
      open={open}
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
            })
          }
        />
      }
    >
      <div className="stack-sm">
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
            <span className="form-label">构建分支</span>
            <input className="input" value={branch} placeholder="main" onChange={(e) => setBranch(e.target.value)} />
          </label>
        ) : null}
      </div>
    </Modal>
  );
}
