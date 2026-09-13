"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type {
  CreateDeploymentCommand,
  DeploymentOptionsVM,
  DeploymentRunListVM,
  DeploymentRunVM,
} from "@sparkops/contracts/delivery";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { BffError } from "@sparkops/contracts";
import { delivery, secret } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { GitRefValueField } from "./GitRefValueField";
import { ProjectDeploymentConsolePanel } from "./ProjectDeploymentConsolePanel";
import { deploymentGate } from "./deployment-gate";
import {
  createSerialPoller,
  shouldContinueDeploymentPoll,
} from "./deployment-poll";
import { isActiveDeploymentRun } from "./deployment-console-utils";

interface ProjectDeployModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  initialRun?: DeploymentRunVM | null;
  deploymentPage?: DeploymentRunListVM | null;
  initialConsoleView?: boolean;
  crossCluster?: boolean;
  onRunChanged?: (run: DeploymentRunVM) => void;
  onDeployStarted?: (run: DeploymentRunVM) => void;
  onClose: () => void;
}

const STAGES = ["preparing", "building", "deploying"] as const;

export function ProjectDeployModal({
  open,
  workspaceId,
  project,
  initialRun = null,
  deploymentPage = null,
  initialConsoleView = false,
  crossCluster = false,
  onRunChanged,
  onDeployStarted,
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
  const [viewMode, setViewMode] = useState<"form" | "console">("form");
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
    setViewMode(initialRunRef.current || initialConsoleView ? "console" : "form");
    setGitRefType(project.git_ref_type === "tag" ? "tag" : "branch");
    setGitRefValue(project.git_ref_value ?? "");
    setImageTag("");

    void Promise.all([
      delivery.getDeploymentOptions(project.id, workspaceId),
      secret.listSecrets(workspaceId),
      delivery.listDeployments(project.id, workspaceId),
    ])
      .then(([nextOptions, nextSecrets, deploymentList]) => {
        if (cancelled) return;
        const nextActive =
          deploymentList.items.find((item) => isActiveDeploymentRun(item)) ?? null;
        const selectedRun = initialRunRef.current ?? nextActive;
        setOptions(nextOptions);
        setSecrets(nextSecrets);
        setActiveRun(selectedRun);
        setRun(selectedRun);
        if (selectedRun) setViewMode("console");
        const buildCluster =
          nextOptions.clusters.find((cluster) => cluster.is_build_env) ??
          nextOptions.clusters[0];
        const preferred = crossCluster
          ? buildCluster?.id
          : nextOptions.clusters.some((cluster) => cluster.id === project.default_cluster_id)
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
  }, [open, project, workspaceId, initialConsoleView, crossCluster]);

  useEffect(() => {
    if (!open || !run || !isActiveDeploymentRun(run) || viewMode !== "form") return;
    const generation = ++pollGenerationRef.current;
    let stopStream: (() => void) | null = null;
    const poller = createSerialPoller({
      refresh: () => delivery.getDeployment(run.id, workspaceId),
      onValue: (next) => {
        setRun(next);
        setActiveRun(isActiveDeploymentRun(next) ? next : null);
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
  }, [open, run?.id, run?.status, viewMode, workspaceId, onRunChanged]);

  const gate = useMemo(
    () => deploymentGate(project, secrets, options, activeRun),
    [project, secrets, options, activeRun],
  );
  const gitServiceType = useMemo(
    () => secrets.find((item) => item.id === project.secret_id)?.service_type,
    [project.secret_id, secrets],
  );

  const consoleDeploymentPage = useMemo((): DeploymentRunListVM | null => {
    if (deploymentPage?.items.length) return deploymentPage;
    if (!run) return null;
    return { items: [run], total: 1, page: 1, page_size: 20 };
  }, [deploymentPage, run]);

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
      cluster_id: crossCluster ? undefined : clusterId || undefined,
      git_ref_type: gitRefType,
      git_ref_value: trimmedRef,
      ...(imageTag.trim() ? { image_tag: imageTag.trim() } : {}),
      ...(crossCluster ? { cross_cluster: true } : {}),
    };
    try {
      const created = await delivery.createDeployment(project.id, workspaceId, input);
      if (!mountedRef.current) return;
      setRun(created);
      setActiveRun(created);
      setViewMode("console");
      onRunChanged?.(created);
      onDeployStarted?.(created);
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
  const showConsole = viewMode === "console" && run && consoleDeploymentPage;

  return (
    <Modal open={open} onClose={onClose} title={crossCluster ? "跨集群发布" : "部署项目"} size="xl">
      <form className="stack-md" onSubmit={submit}>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        {showConsole ? (
          <ProjectDeploymentConsolePanel
            workspaceId={workspaceId}
            project={project}
            deploymentPage={consoleDeploymentPage}
            active={open}
            selectedRunId={run.id}
            onRunChanged={onRunChanged}
            headerExtra={
              <button
                type="button"
                className="btn btn-secondary text-sm"
                onClick={() => setViewMode("form")}
              >
                切换部署表单
              </button>
            }
          />
        ) : (
          <>
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
                disabled={loading || Boolean(run) || crossCluster}
                required
              >
                <option value="">请选择授权集群</option>
                {options.clusters.map((cluster) => (
                  <option key={cluster.id} value={cluster.id}>
                    {cluster.cluster_name}
                    {cluster.is_build_env ? "（构建集群）" : ""}
                  </option>
                ))}
              </select>
              {crossCluster ? (
                <p className="text-muted text-sm">
                  跨集群发布将在构建集群构建并部署，随后把同一镜像同步到其余授权集群。
                </p>
              ) : null}

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
                placeholder="留空则使用 Git 引用值（如 main、v1.0.0）"
              />
            </label>

            {run ? (
              <section className="stack-md" aria-live="polite">
                <div className="flex items-center justify-between gap-2">
                  <h3>部署进度：{run.status}</h3>
                  <button
                    type="button"
                    className="btn btn-secondary text-sm"
                    onClick={() => setViewMode("console")}
                  >
                    查看控制台
                  </button>
                </div>
                <ol className="flex flex-wrap gap-2">
                  {STAGES.map((item) => (
                    <li
                      key={item}
                      className={stage === item ? "project-tag project-tag-blue" : "project-tag"}
                    >
                      {item}
                    </li>
                  ))}
                </ol>
                {streamOutput ? (
                  <pre className="text-xs whitespace-pre-wrap">{streamOutput}</pre>
                ) : null}
              </section>
            ) : null}
          </>
        )}

        <div className="flex justify-end gap-2">
          <button type="button" className="btn-secondary" onClick={onClose}>
            {run ? "关闭" : "取消"}
          </button>
          {!showConsole && !run ? (
            <button
              type="submit"
              className="btn"
              disabled={loading || submitting || !gate.allowed || !clusterId}
            >
              {submitting ? "提交中…" : "开始部署"}
            </button>
          ) : null}
          {!showConsole && run && isActiveDeploymentRun(run) ? (
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
