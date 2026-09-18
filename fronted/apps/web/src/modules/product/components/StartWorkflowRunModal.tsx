"use client";

import { useMemo, useState } from "react";
import type { EnvironmentVM, ProductWorkflowVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { jobTypeLabel } from "./workflow-editor-utils";

interface StartWorkflowRunModalProps {
  open: boolean;
  workflow: ProductWorkflowVM | null;
  environments: EnvironmentVM[];
  busy?: boolean;
  onClose: () => void;
  onSubmit: (input: {
    branch?: string;
    environment_overrides?: Record<string, string>;
  }) => void;
}

export function StartWorkflowRunModal({
  open,
  workflow,
  environments,
  busy,
  onClose,
  onSubmit,
}: StartWorkflowRunModalProps) {
  const deployJobs = useMemo(() => {
    const stages = workflow?.stages ?? [];
    return stages.flatMap((stage) => stage.jobs.filter((job) => job.type === "sparkops-deploy"));
  }, [workflow]);

  const [branch, setBranch] = useState("main");
  const [envOverrides, setEnvOverrides] = useState<Record<string, string>>({});

  if (!open || !workflow) return null;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="执行工作流"
      description={`${workflow.name} · 可覆盖运行时分支与部署环境`}
      footer={
        <>
          <button type="button" className="btn btn-ghost text-sm" onClick={onClose} disabled={busy}>
            取消
          </button>
          <button
            type="button"
            className="btn btn-primary text-sm"
            disabled={busy}
            onClick={() =>
              onSubmit({
                branch: branch.trim() || undefined,
                environment_overrides: Object.keys(envOverrides).length ? envOverrides : undefined,
              })
            }
          >
            开始执行
          </button>
        </>
      }
    >
      <div className="stack-md">
        <label className="field">
          <span className="text-xs text-muted">构建分支（覆盖全部构建任务）</span>
          <input className="input" value={branch} onChange={(e) => setBranch(e.target.value)} />
        </label>

        {deployJobs.length > 0 ? (
          <div className="stack-sm">
            <p className="text-sm font-medium">部署环境覆盖</p>
            {deployJobs.map((job) => (
              <label key={job.name} className="field">
                <span className="text-xs text-muted">
                  {jobTypeLabel(job.type)} · {job.name}
                </span>
                <select
                  className="input"
                  value={envOverrides[job.name] ?? String(job.spec?.environment_id ?? "")}
                  onChange={(e) =>
                    setEnvOverrides((prev) => ({ ...prev, [job.name]: e.target.value }))
                  }
                >
                  <option value="">使用定义中的环境</option>
                  {environments.map((env) => (
                    <option key={env.id} value={env.id}>
                      {env.name}
                    </option>
                  ))}
                </select>
              </label>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted">当前工作流无部署任务，将直接按定义执行。</p>
        )}
      </div>
    </Modal>
  );
}
