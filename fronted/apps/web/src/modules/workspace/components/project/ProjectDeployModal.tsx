"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type {
  CreateDeploymentCommand,
  DeploymentOptionsVM,
  DeploymentRunVM,
} from "@sparkops/contracts/delivery";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { BffError } from "@sparkops/contracts";
import { delivery, secret } from "@sparkops/web-sdk";
import { BuildLogStream } from "@/modules/delivery/components/BuildLogStream";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { GitRefValueField } from "./GitRefValueField";
import { deploymentGate } from "./deployment-gate";
import {
  createSerialPoller,
  shouldContinueDeploymentPoll,
} from "./deployment-poll";

interface ProjectDeployModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  initialRun?: DeploymentRunVM | null;
  onRunChanged?: (run: DeploymentRunVM) => void;
  onClose: () => void;
}

const ACTIVE_STATUSES = new Set(["queued", "preparing", "building", "deploying"]);
const STAGES = ["preparing", "building", "deploying"] as const;

export function ProjectDeployModal({
  open,
  workspaceId,
  project,
  initialRun = null,
  onRunChanged,
  onClose,
}: ProjectDeployModalProps) {
  const [options, setOptions] = useState<DeploymentOptionsVM>({ clusters: [], missing: [] });
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [activeRun, setActiveRun] = useState<DeploymentRunVM | null>(null);
  const [run, setRun] = useState<DeploymentRunVM | null>(null);
  const [clusterId, setClusterId] = useState("");
  const [gitRefType, setGitRefType] = useState<"branch" | "tag">(
    project.git_ref_type === "tag" ? "tag" : "branch",
  );
  const [gitRefValue, setGitRefValue] = useState(project.git_ref_value ?? "");
  const [imageTag, setImageTag] = useState("");
  const [streamOutput, setStreamOutput] = useState("");
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const mountedRef = useRef(true);
  const pollGenerationRef = useRef(0);
  const initialRunRef = useRef(initialRun);
  initialRunRef.current = initialRun;

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setRun(initialRunRef.current);
    setStreamOutput("");
    setError(null);
    setLoading(true);
    setGitRefType(project.git_ref_type === "tag" ? "tag" : "branch");
    setGitRefValue(project.git_ref_value ?? "");
    setImageTag("");

    void Promise.all([
      delivery.getDeploymentOptions(project.id, workspaceId),
      secret.listSecrets(workspaceId),
      delivery.listDeployments(project.id, workspaceId),
    ])
      .then(([nextOptions, nextSecrets, deploymentPage]) => {
        if (cancelled) return;
        const nextActive =
          deploymentPage.items.find((item) => ACTIVE_STATUSES.has(item.status)) ?? null;
        const selectedRun = initialRunRef.current ?? nextActive;
        setOptions(nextOptions);
        setSecrets(nextSecrets);
        setActiveRun(selectedRun);
        setRun(selectedRun);
        const preferred = nextOptions.clusters.some(
          (cluster) => cluster.id === project.default_cluster_id,
        )
          ? project.default_cluster_id
          : nextOptions.clusters[0]?.id;
        setClusterId(preferred ?? "");
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof BffError ? err.message : "加载部署选项失败");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [open, project, workspaceId]);

  useEffect(() => {
    if (!open || !run || !ACTIVE_STATUSES.has(run.status)) return;
    const generation = ++pollGenerationRef.current;
    let stopStream: (() => void) | null = null;
    const poller = createSerialPoller({
      refresh: () => delivery.getDeployment(run.id, workspaceId),
      onValue: (next) => {
        setRun(next);
        setActiveRun(ACTIVE_STATUSES.has(next.status) ? next : null);
        onRunChanged?.(next);
        if (!shouldContinueDeploymentPoll(next.status, generation, pollGenerationRef.current)) {
          stopStream?.();
          stopStream = null;
        }
      },
      onError: (err) => {
        setError(err instanceof BffError ? err.message : "刷新部署状态失败");
      },
      shouldContinue: (next) =>
        shouldContinueDeploymentPoll(next.status, generation, pollGenerationRef.current),
      schedule: (callback, delayMs) => window.setTimeout(callback, delayMs),
      cancel: (timer) => window.clearTimeout(timer),
      delayMs: 2000,
    });
    stopStream = delivery.streamDeployment(run.id, {
      onDelta: (delta) => {
        if (pollGenerationRef.current !== generation) return;
        setStreamOutput((value) => value + delta);
      },
      onDone: () => {
        stopStream?.();
        stopStream = null;
      },
      onError: () => {
        stopStream?.();
        stopStream = null;
      },
    });
    poller.start();
    return () => {
      if (pollGenerationRef.current === generation) {
        pollGenerationRef.current += 1;
      }
      poller.stop();
      stopStream?.();
    };
  }, [open, run?.id, run?.status, workspaceId, onRunChanged]);

  const gate = useMemo(
    () => deploymentGate(project, secrets, options, activeRun),
    [project, secrets, options, activeRun],
  );
  const gitServiceType = useMemo(
    () => secrets.find((item) => item.id === project.secret_id)?.service_type,
    [project.secret_id, secrets],
  );

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const trimmedRef = gitRefValue.trim();
    if (!trimmedRef) {
      setError("请填写 Git 引用值");
      return;
    }
    setSubmitting(true);
    setError(null);
    const input: CreateDeploymentCommand = {
      cluster_id: clusterId || undefined,
      git_ref_type: gitRefType,
      git_ref_value: trimmedRef,
      ...(imageTag.trim() ? { image_tag: imageTag.trim() } : {}),
    };
    try {
      const created = await delivery.createDeployment(project.id, workspaceId, input);
      if (!mountedRef.current) return;
      setRun(created);
      setActiveRun(created);
      onRunChanged?.(created);
    } catch (err) {
      if (mountedRef.current) {
        setError(err instanceof BffError ? err.message : "创建部署失败");
      }
    } finally {
      if (mountedRef.current) setSubmitting(false);
    }
  }

  async function abort() {
    if (!run) return;
    setSubmitting(true);
    setError(null);
    try {
      const aborted = await delivery.abortDeployment(run.id, workspaceId);
      if (!mountedRef.current) return;
      setRun(aborted);
      setActiveRun(null);
      onRunChanged?.(aborted);
    } catch (err) {
      if (mountedRef.current) setError(err instanceof BffError ? err.message : "终止部署失败");
    } finally {
      if (mountedRef.current) setSubmitting(false);
    }
  }

  const stage = run?.current_stage;

  return (
    <Modal open={open} onClose={onClose} title="部署项目" size="xl">
      <form className="stack-md" onSubmit={submit}>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {loading ? <p className="text-muted">正在加载部署选项…</p> : null}
        {!loading && gate.missing.length > 0 ? (
          <InlineAlert tone="error">
            <span>当前不可部署：</span>
            <ul>
              {gate.missing.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </InlineAlert>
        ) : null}

        <label className="stack-sm">
          <span>目标集群</span>
          <select
            className="input"
            value={clusterId}
            onChange={(event) => setClusterId(event.target.value)}
            disabled={loading || Boolean(run)}
            required
          >
            <option value="">请选择授权集群</option>
            {options.clusters.map((cluster) => (
              <option key={cluster.id} value={cluster.id}>
                {cluster.cluster_name}
              </option>
            ))}
          </select>
        </label>

        <label className="stack-sm">
          <span>Git 引用类型</span>
          <select
            className="input"
            value={gitRefType}
            onChange={(event) => {
              setGitRefType(event.target.value as "branch" | "tag");
              setGitRefValue("");
            }}
            disabled={Boolean(run)}
          >
            <option value="branch">分支</option>
            <option value="tag">标签</option>
          </select>
        </label>

        <GitRefValueField
          key={`${open ? "open" : "closed"}-${gitRefType}`}
          workspaceId={workspaceId}
          gitUrl={project.git_url}
          secretId={project.secret_id}
          gitServiceType={gitServiceType}
          refType={gitRefType}
          value={gitRefValue}
          onChange={setGitRefValue}
          disabled={Boolean(run)}
        />

        <label className="stack-sm">
          <span>镜像版本（可选）</span>
          <input
            className="input"
            value={imageTag}
            onChange={(event) => setImageTag(event.target.value)}
            disabled={Boolean(run)}
            maxLength={255}
            placeholder="留空则使用本次部署 ID 去连字符后的前 12 位"
          />
        </label>

        {run ? (
          <section className="stack-md" aria-live="polite">
            <h3>部署进度：{run.status}</h3>
            <ol className="flex flex-wrap gap-2">
              {STAGES.map((item) => (
                <li key={item} className={stage === item ? "project-tag project-tag-blue" : "project-tag"}>
                  {item}
                </li>
              ))}
            </ol>
            {streamOutput ? <pre className="text-xs whitespace-pre-wrap">{streamOutput}</pre> : null}
            {run.status === "building" && run.build_id ? (
              <BuildLogStream buildId={run.build_id} active initialLog="" />
            ) : null}
          </section>
        ) : null}

        <div className="flex justify-end gap-2">
          <button type="button" className="btn-secondary" onClick={onClose}>
            {run ? "关闭" : "取消"}
          </button>
          {!run ? (
            <button
              type="submit"
              className="btn"
              disabled={loading || submitting || !gate.allowed || !clusterId}
            >
              {submitting ? "提交中…" : "开始部署"}
            </button>
          ) : ACTIVE_STATUSES.has(run.status) ? (
            <button
              type="button"
              className="btn-secondary"
              disabled={submitting}
              onClick={() => void abort()}
            >
              {submitting ? "终止中…" : "终止部署"}
            </button>
          ) : null}
        </div>
      </form>
    </Modal>
  );
}
