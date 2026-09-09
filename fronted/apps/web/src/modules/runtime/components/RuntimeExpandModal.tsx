"use client";

import { useEffect, useMemo, useState } from "react";
import type { DeploymentOverviewVM } from "@sparkops/contracts/cluster";
import { BffError } from "@sparkops/contracts";
import { cluster } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import { DualRangeSlider } from "@/modules/workspace/components/project/project-dual-slider";
import {
  EXPAND_CPU_MARKS,
  EXPAND_MEMORY_MARKS,
  formatCpuMillicores,
  formatMemoryMi,
  parseCpuToMillicores,
  parseMemoryToMi,
  snapToNearestMark,
} from "@/modules/workspace/components/project/project-resource-utils";

interface RuntimeExpandModalProps {
  open: boolean;
  projectId: string;
  workspaceId: string;
  deployment?: DeploymentOverviewVM | null;
  fallback?: {
    cpu_request?: string;
    cpu_limit?: string;
    memory_request?: string;
    memory_limit?: string;
  };
  onClose: () => void;
  onSaved: () => void;
}

function marksToSlider(marks: readonly { label: string; millicores?: number; mebibytes?: number }[]) {
  return marks.map((mark) => ({
    label: mark.label,
    value: mark.millicores ?? mark.mebibytes ?? 0,
  }));
}

function ExpandResourceSection({
  title,
  requestLabel,
  limitLabel,
  marks,
  requestValue,
  limitValue,
  format,
  parse,
  onRequestChange,
  onLimitChange,
}: {
  title: string;
  requestLabel: string;
  limitLabel: string;
  marks: readonly { label: string; millicores?: number; mebibytes?: number }[];
  requestValue: string;
  limitValue: string;
  format: (value: number) => string;
  parse: (raw: string) => number;
  onRequestChange: (value: string) => void;
  onLimitChange: (value: string) => void;
}) {
  const sliderMarks = useMemo(() => marksToSlider(marks), [marks]);
  const markValues = sliderMarks.map((mark) => mark.value);
  const min = markValues[0] ?? 0;
  const max = markValues[markValues.length - 1] ?? min;
  const requestNum = Math.min(Math.max(parse(requestValue), min), max);
  const limitNum = Math.min(Math.max(parse(limitValue), min), max);

  return (
    <section className="runtime-expand-section">
      <h4 className="runtime-expand-section-title">{title}</h4>
      <DualRangeSlider
        low={requestNum}
        high={Math.max(limitNum, requestNum)}
        marks={sliderMarks}
        onLowChange={(value) =>
          onRequestChange(format(snapToNearestMark(Math.round(value), markValues)))
        }
        onHighChange={(value) =>
          onLimitChange(format(snapToNearestMark(Math.round(value), markValues)))
        }
      />
      <p className="runtime-expand-summary">
        {requestLabel}为 {requestValue}，{limitLabel}为 {limitValue}
      </p>
    </section>
  );
}

export function RuntimeExpandModal({
  open,
  projectId,
  workspaceId,
  deployment,
  fallback,
  onClose,
  onSaved,
}: RuntimeExpandModalProps) {
  const [cpuRequest, setCpuRequest] = useState("500m");
  const [cpuLimit, setCpuLimit] = useState("1");
  const [memoryRequest, setMemoryRequest] = useState("64Mi");
  const [memoryLimit, setMemoryLimit] = useState("64Mi");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setCpuRequest(deployment?.cpu_request || fallback?.cpu_request || "500m");
    setCpuLimit(deployment?.cpu_limit || fallback?.cpu_limit || "1");
    setMemoryRequest(deployment?.memory_request || fallback?.memory_request || "64Mi");
    setMemoryLimit(deployment?.memory_limit || fallback?.memory_limit || "64Mi");
    setError(null);
  }, [open, deployment, fallback]);

  async function handleSubmit() {
    setSubmitting(true);
    setError(null);
    try {
      await cluster.putDeploymentResources(projectId, workspaceId, {
        cpu_request: cpuRequest,
        cpu_limit: cpuLimit,
        memory_request: memoryRequest,
        memory_limit: memoryLimit,
      });
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "服务扩容失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="服务扩容"
      panelClassName="runtime-expand-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn btn-secondary" onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button type="button" className="btn btn-primary" onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "保存中…" : "保存"}
          </button>
        </div>
      }
    >
      {error ? <p className="runtime-config-modal-error">{error}</p> : null}
      <div className="runtime-expand-alert" role="status">
        <span className="runtime-expand-alert-icon" aria-hidden>
          i
        </span>
        <span>
          服务当前内存为 {memoryRequest}，最大内存为 {memoryLimit}
        </span>
      </div>
      <ExpandResourceSection
        title="CPU"
        requestLabel="CPU"
        limitLabel="最大CPU"
        marks={EXPAND_CPU_MARKS}
        requestValue={cpuRequest}
        limitValue={cpuLimit}
        format={formatCpuMillicores}
        parse={parseCpuToMillicores}
        onRequestChange={setCpuRequest}
        onLimitChange={setCpuLimit}
      />
      <ExpandResourceSection
        title="内存"
        requestLabel="内存"
        limitLabel="最大内存"
        marks={EXPAND_MEMORY_MARKS}
        requestValue={memoryRequest}
        limitValue={memoryLimit}
        format={(value) => formatMemoryMi(value, 64)}
        parse={(raw) => parseMemoryToMi(raw, 64)}
        onRequestChange={setMemoryRequest}
        onLimitChange={setMemoryLimit}
      />
    </Modal>
  );
}
