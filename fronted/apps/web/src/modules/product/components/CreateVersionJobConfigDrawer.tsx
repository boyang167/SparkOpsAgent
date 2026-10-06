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

  return (
    <Drawer open={open} title="创建交付版本" onClose={onClose} width={480}>
      <div className="pwf-drawer-form">
        <label className="pwf-field">
          <span>任务名称</span>
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </label>
        <label className="pwf-field">
          <span>版本号</span>
          <input
            value={draft.versionTag}
            placeholder="{{version}}"
            onChange={(event) => setDraft((prev) => ({ ...prev, versionTag: event.target.value }))}
          />
        </label>
        <label className="pwf-field">
          <span>上游构建任务</span>
          <select
            value={draft.jobName}
            onChange={(event) => setDraft((prev) => ({ ...prev, jobName: event.target.value }))}
          >
            <option value="">请选择</option>
            {sourceJobs.map((job) => (
              <option key={job.name} value={job.name}>
                {job.name}
              </option>
            ))}
          </select>
        </label>
        <p className="pwf-field-hint">执行时将快照当前 Service/Deployment YAML，并绑定上游构建镜像。</p>
        <div className="pwf-drawer-actions">
          <button type="button" className="btn-secondary" onClick={onClose}>
            取消
          </button>
          <button
            type="button"
            className="btn-primary"
            disabled={!name.trim() || !draft.versionTag.trim() || !draft.jobName.trim()}
            onClick={() => onConfirm(name.trim(), createVersionJobSpecToRecord(draft))}
          >
            确定
          </button>
        </div>
      </div>
    </Drawer>
  );
}
