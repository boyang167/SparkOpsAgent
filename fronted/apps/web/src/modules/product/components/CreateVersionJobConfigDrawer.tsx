"use client";

import { useEffect, useState } from "react";
import { Drawer } from "@/components/ui/Drawer";
import {
  createVersionJobSpecToRecord,
  parseCreateVersionJobSpec,
  type CreateVersionJobSpec,
} from "./create-version-job-spec";
import type { EditorJob } from "./workflow-editor-utils";

interface CreateVersionJobConfigDrawerProps {
  open: boolean;
  jobName: string;
  spec: Record<string, unknown> | undefined;
  sourceJobs: EditorJob[];
  onClose: () => void;
  onConfirm: (name: string, spec: Record<string, unknown>) => void;
}

export function CreateVersionJobConfigDrawer({
  open,
  jobName,
  spec,
  sourceJobs,
  onClose,
  onConfirm,
}: CreateVersionJobConfigDrawerProps) {
  const [name, setName] = useState(jobName);
  const [draft, setDraft] = useState<CreateVersionJobSpec>(() => parseCreateVersionJobSpec(spec));

  useEffect(() => {
    if (!open) return;
    setName(jobName);
    const parsed = parseCreateVersionJobSpec(spec);
    const fallback = sourceJobs.length > 0 ? sourceJobs[sourceJobs.length - 1].name : "";
    setDraft({
      ...parsed,
      jobName: parsed.jobName.trim() || fallback,
    });
  }, [open, jobName, spec, sourceJobs]);

  const canConfirm =
    name.trim().length > 0 && draft.versionTag.trim().length > 0 && draft.jobName.trim().length > 0;

  return (
    <Drawer
      open={open}
      title="创建交付版本"
      description="执行时将快照当前 Service/Deployment YAML，并绑定上游构建镜像。"
      onClose={onClose}
      panelClassName="drawer-panel--config"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn-primary"
            disabled={!canConfirm}
            onClick={() => onConfirm(name.trim(), createVersionJobSpecToRecord(draft))}
          >
            确定
          </button>
        </div>
      }
    >
      <div className="pwf-build-form">
        <label className="pwf-build-field">
          <span>
            任务名称
            <span className="pwf-required" aria-hidden>
              *
            </span>
          </span>
          <span className="pwf-build-control">
            <input
              className="input"
              value={name}
              onChange={(event) => setName(event.target.value)}
              aria-label="任务名称"
            />
          </span>
        </label>

        <label className="pwf-build-field">
          <span>
            版本号
            <span className="pwf-required" aria-hidden>
              *
            </span>
          </span>
          <span className="pwf-build-control">
            <input
              className="input input-mono"
              value={draft.versionTag}
              placeholder="{{version}}"
              onChange={(event) => setDraft((prev) => ({ ...prev, versionTag: event.target.value }))}
              aria-label="版本号"
            />
          </span>
        </label>

        <label className="pwf-build-field">
          <span>
            上游构建任务
            <span className="pwf-required" aria-hidden>
              *
            </span>
          </span>
          <span className="pwf-build-control">
            <select
              className="input"
              value={draft.jobName}
              onChange={(event) => setDraft((prev) => ({ ...prev, jobName: event.target.value }))}
              aria-label="上游构建任务"
            >
              <option value="">请选择</option>
              {sourceJobs.map((job) => (
                <option key={job.name} value={job.name}>
                  {job.name}
                </option>
              ))}
            </select>
          </span>
        </label>

        {sourceJobs.length === 0 ? (
          <div className="create-form-notice">
            <p>当前阶段没有可用的上游构建任务，请先在本阶段或上游阶段添加构建任务。</p>
          </div>
        ) : null}
      </div>
    </Drawer>
  );
}
