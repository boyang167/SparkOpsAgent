"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { DeploymentOptionsVM, DeploymentRunListVM, DeploymentRunVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { delivery } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { computeDagLayout } from "../../lib/dag-layout";
import { STRATEGY_LABELS } from "../../lib/flow-adapter";
import {
  artifactsFromDeploymentRuns,
  defaultReleaseSource,
  hasProjectJenkinsTemplate,
  imageCoordinate,
  pipelineContainsBuild,
  sourceStepWarnings,
  validateReleaseDraft,
} from "../../lib/release-draft";
import { saveReleaseRun } from "../../lib/release-run-store";
import { buildReleasePreflight, getMockPipeline } from "../../mock/data";
import type { ArtifactSource, PipelineVM, ReleaseDraft, ReleasePreflightVM } from "../../types";

interface ReleaseWizardModalProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  deployments?: DeploymentRunListVM | null;
  options?: DeploymentOptionsVM | null;
  onClose: () => void;
  onBuildStarted?: (run: DeploymentRunVM) => void;
}

const STEPS = ["制品来源", "发布编排", "目标环境", "确认发布"] as const;
const DEFAULT_PIPELINE_ID = "pipe-release-staging-then-prod";

function imageRepositoryOf(project: ProjectVM): string {
  const ns = project.namespace || project.name;
  return `${ns}/${project.name}`;
}

function emptyDraft(project: ProjectVM, source: ArtifactSource = "build"): ReleaseDraft {
  return {
    source,
    pipelineId: DEFAULT_PIPELINE_ID,
    parameters: {},
    fleetIds: ["fleet-staging-all"],
    gitRefType: project.git_ref_type === "tag" ? "tag" : "branch",
    gitRefValue: project.git_ref_value || "master",
    imageTag: "",
    imageRepository: imageRepositoryOf(project),
  };
}

export function ReleaseWizardModal({
  open,
  workspaceId,
  project,
  deployments,
  options,
  onClose,
}: ReleaseWizardModalProps) {
  const router = useRouter();
  const [loadedDeployments, setLoadedDeployments] = useState<DeploymentRunListVM | null>(deployments ?? null);
  const [loadedOptions, setLoadedOptions] = useState<DeploymentOptionsVM | null>(options ?? null);
  const [step, setStep] = useState(0);
  const [draft, setDraft] = useState<ReleaseDraft>(() => emptyDraft(project));
  const [submitting, setSubmitting] = useState(false);
  const [doneRunId, setDoneRunId] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [stepError, setStepError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setDraft(emptyDraft(project));
    setStep(0);
    setDoneRunId(null);
    setSubmitError(null);
    setStepError(null);
    let cancelled = false;
    void Promise.allSettled([
      deployments ? Promise.resolve(deployments) : delivery.listDeployments(project.id, workspaceId),
      options ? Promise.resolve(options) : delivery.getDeploymentOptions(project.id, workspaceId),
    ]).then(([pageResult, optionsResult]) => {
      if (cancelled) return;
      const page = pageResult.status === "fulfilled" ? pageResult.value : null;
      if (page) setLoadedDeployments(page);
      if (optionsResult.status === "fulfilled") setLoadedOptions(optionsResult.value);
      const artifactCount = artifactsFromDeploymentRuns(page?.items ?? [], imageRepositoryOf(project)).length;
      setDraft((current) => ({
        ...current,
        source: defaultReleaseSource(artifactCount),
        imageTag:
          artifactCount > 0 && !current.imageTag
            ? artifactsFromDeploymentRuns(page?.items ?? [], imageRepositoryOf(project))[0]!.image_tag
            : current.imageTag,
      }));
    });
    return () => {
      cancelled = true;
    };
  }, [open, project, workspaceId, deployments, options]);

  const preflight = useMemo(() => buildReleasePreflight(), []);
  const artifacts = useMemo(
    () => artifactsFromDeploymentRuns(loadedDeployments?.items ?? [], imageRepositoryOf(project)),
    [loadedDeployments, project],
  );
  const hasJenkinsTemplate = hasProjectJenkinsTemplate(project);
  const stepSourceWarnings = useMemo(
    () =>
      sourceStepWarnings({
        source: draft.source,
        missing: loadedOptions?.missing ?? [],
        clusterCount: loadedOptions?.clusters.length ?? 0,
      }),
    [draft.source, loadedOptions],
  );

  const selectedPipeline = getMockPipeline(draft.pipelineId);
  const previewStages = selectedPipeline?.definition.spec.stages ?? [];
  const dagPreview = useMemo(
    () => computeDagLayout(previewStages, selectedPipeline?.layout),
    [previewStages, selectedPipeline?.layout],
  );
  const containsBuild = pipelineContainsBuild(previewStages);
  const skipBuild = draft.source === "artifact" && containsBuild;

  function resetAndClose() {
    setStep(0);
    setDoneRunId(null);
    setSubmitError(null);
    setStepError(null);
    setDraft(emptyDraft(project));
    onClose();
  }

  function goNext() {
    if (step === 0) {
      if (draft.source === "artifact" && !draft.imageTag.trim()) {
        setStepError("请选择或填写镜像 Tag");
        return;
      }
      if (draft.source === "build" && !draft.gitRefValue.trim()) {
        setStepError("请填写 Git 引用");
        return;
      }
    }
    if (step === 1 && !draft.pipelineId) {
      setStepError("请选择发布 Pipeline");
      return;
    }
    if (step === 2 && draft.fleetIds.length === 0) {
      setStepError("请至少选择一个 Fleet");
      return;
    }
    setStepError(null);
    setStep((s) => s + 1);
  }

  async function onConfirmRelease() {
    const errors = validateReleaseDraft(draft, { hasJenkinsTemplate, buildBlockedReasons: [] });
    if (errors.length > 0) {
      setSubmitError(errors.join("；"));
      return;
    }
    setSubmitting(true);
    setSubmitError(null);
    try {
      let deploymentRunId: string | undefined;
      let imageTag = draft.imageTag.trim();
      if (draft.source === "build") {
        try {
          const build = await delivery.triggerBuild(project.id, workspaceId, draft.gitRefValue.trim());
          deploymentRunId = build.id;
          imageTag = imageTag || build.id.slice(0, 8);
        } catch (err) {
          imageTag = imageTag || "pending";
          setSubmitError(
            err instanceof BffError
              ? `${err.message}。已继续创建发布编排，构建需补齐后再触发。`
              : "构建未能启动，已继续创建发布编排。",
          );
        }
      }

      const runId = `rel-${Date.now()}`;
      const selectedFleets = preflight.fleets.filter((fleet) => draft.fleetIds.includes(fleet.id));
      saveReleaseRun({
        id: runId,
        projectId: project.id,
        projectName: project.name,
        source: draft.source,
        pipelineId: draft.pipelineId,
        pipelineName: selectedPipeline?.name ?? draft.pipelineId,
        fleetIds: draft.fleetIds,
        fleetNames: selectedFleets.map((fleet) => fleet.name),
        imageTag,
        imageRepository: draft.imageRepository,
        gitRefType: draft.gitRefType,
        gitRefValue: draft.gitRefValue,
        skippedBuildSteps: skipBuild,
        strategies: fleetStrategies(selectedPipeline),
        deploymentRunId,
        createdAt: new Date().toISOString(),
      });
      setDoneRunId(runId);
    } catch (err) {
      setSubmitError(err instanceof BffError ? err.message : err instanceof Error ? err.message : "创建发布失败");
    } finally {
      setSubmitting(false);
    }
  }

  const footer = (
    <div className="project-wizard-actions is-centered">
      {step > 0 && !doneRunId ? (
        <button type="button" className="btn-secondary" onClick={() => setStep((s) => s - 1)}>
          上一步
        </button>
      ) : (
        <button type="button" className="btn-secondary" onClick={resetAndClose}>
          {doneRunId ? "关闭" : "取消"}
        </button>
      )}
      {!doneRunId ? (
        step < STEPS.length - 1 ? (
          <button type="button" className="btn project-btn-primary" onClick={goNext}>
            下一步
          </button>
        ) : (
          <button type="button" className="btn project-btn-primary" disabled={submitting} onClick={() => void onConfirmRelease()}>
            {submitting ? (draft.source === "build" ? "正在创建构建…" : "创建中…") : "确认发布"}
          </button>
        )
      ) : (
        <button
          type="button"
          className="btn project-btn-primary"
          onClick={() => {
            const id = doneRunId;
            resetAndClose();
            router.push(`/delivery/runs/${id}`);
          }}
        >
          前往 Fleet Board
        </button>
      )}
    </div>
  );

  return (
    <Modal
      open={open}
      onClose={resetAndClose}
      title="发布向导"
      description={`项目「${project.name}」· 构建归 Jenkins 模板，发布归 Pipeline`}
      size="xl"
      panelClassName="project-create-modal project-wizard-modal"
      footer={footer}
    >
      <div className="project-wizard-form is-modal-layout">
        <div className="project-wizard-steps is-modal is-ref">
          {STEPS.map((label, index) => (
            <div key={label} className="project-wizard-step-wrap">
              <div
                className={`project-wizard-step${index === step ? " is-active" : ""}${index < step ? " is-done" : ""}`}
              >
                <span className="project-wizard-step-circle">{index + 1}</span>
                <span className="project-wizard-step-label">{label}</span>
              </div>
              {index < STEPS.length - 1 ? (
                <div className={`project-wizard-connector${index < step ? " is-done" : ""}`} />
              ) : null}
            </div>
          ))}
        </div>

        <div className="project-wizard-scroll stack-md">
          {doneRunId ? (
            <InlineAlert tone="success">
              已创建发布 Run <code>{doneRunId}</code>
              {draft.source === "artifact"
                ? "。未触发 Jenkins，将按所选 Pipeline 做发布编排（观测为 Mock）。"
                : "。已触发 Jenkins 构建，成功后按 Pipeline 做发布编排（分波观测为 Mock）。"}
            </InlineAlert>
          ) : null}
          {stepError ? <InlineAlert tone="warning">{stepError}</InlineAlert> : null}
          {submitError ? <InlineAlert tone="error">{submitError}</InlineAlert> : null}

          {step === 0 ? (
            <StepSource
              draft={draft}
              artifacts={artifacts}
              hasJenkinsTemplate={hasJenkinsTemplate}
              warnings={stepSourceWarnings}
              onChange={setDraft}
            />
          ) : null}
          {step === 1 ? (
            <StepPipeline
              pipelines={preflight.pipelines}
              selectedId={draft.pipelineId}
              dagPreview={dagPreview}
              stages={previewStages}
              skipBuild={skipBuild}
              onSelect={(pipelineId) => setDraft((d) => ({ ...d, pipelineId }))}
            />
          ) : null}
          {step === 2 ? (
            <StepFleet
              fleets={preflight.fleets}
              selectedIds={draft.fleetIds}
              onChange={(fleetIds) => setDraft((d) => ({ ...d, fleetIds }))}
            />
          ) : null}
          {step === 3 ? (
            <StepConfirm
              draft={draft}
              pipeline={selectedPipeline}
              preflight={preflight}
              project={project}
              skipBuild={skipBuild}
            />
          ) : null}
        </div>
      </div>
    </Modal>
  );
}

function fleetStrategies(pipeline: PipelineVM | undefined): string[] {
  if (!pipeline) return [];
  const out: string[] = [];
  for (const stage of pipeline.definition.spec.stages) {
    for (const step of stage.steps) {
      if (step.uses !== "fleet.deploy") continue;
      const fleet = String(step.with?.fleet ?? "fleet");
      const strategy = String(step.with?.strategy ?? "parallel");
      out.push(`${fleet} · ${STRATEGY_LABELS[strategy] ?? strategy}`);
    }
  }
  return out;
}

function StepSource({
  draft,
  artifacts,
  hasJenkinsTemplate,
  warnings,
  onChange,
}: {
  draft: ReleaseDraft;
  artifacts: ReturnType<typeof artifactsFromDeploymentRuns>;
  hasJenkinsTemplate: boolean;
  warnings: string[];
  onChange: (draft: ReleaseDraft) => void;
}) {
  function selectSource(source: ArtifactSource) {
    onChange({ ...draft, source });
  }

  return (
    <div className="stack-md">
      <p className="muted text-sm">
        已选 Jenkins 模板的项目默认先构建出镜像，再进入发布编排。若已有成功制品，也可以直接选用。
      </p>
      <div className="cd-wizard-template-grid">
        <button
          type="button"
          className={`cd-template-card${draft.source === "artifact" ? " is-selected" : ""}`}
          onClick={() => selectSource("artifact")}
        >
          <strong>使用已有镜像</strong>
          <p className="text-sm">选最近成功构建的 tag，或手填。确认后不触发 Jenkins。</p>
        </button>
        <button
          type="button"
          className={`cd-template-card${draft.source === "build" ? " is-selected" : ""}`}
          onClick={() => selectSource("build")}
        >
          <strong>先构建再发布</strong>
          <p className="text-sm">走现有 Jenkinsfile 构建，成功后再进入同一条发布 DAG。</p>
        </button>
      </div>

      {draft.source === "artifact" ? (
        <div className="cd-form-grid">
          <div className="cd-field">
            <label className="cd-field-label">最近成功制品</label>
            <select
              className="input"
              value={artifacts.some((item) => item.image_tag === draft.imageTag) ? draft.imageTag : ""}
              onChange={(e) => {
                const picked = artifacts.find((item) => item.image_tag === e.target.value);
                onChange({
                  ...draft,
                  imageTag: e.target.value,
                  imageRepository: picked?.image_repository ?? draft.imageRepository,
                  gitRefType: picked?.git_ref_type ?? draft.gitRefType,
                  gitRefValue: picked?.git_ref_value ?? draft.gitRefValue,
                });
              }}
            >
              <option value="">手填 Tag</option>
              {artifacts.map((item) => (
                <option key={item.run_id} value={item.image_tag}>
                  {imageCoordinate(item.image_repository, item.image_tag)}
                </option>
              ))}
            </select>
            {artifacts.length === 0 ? (
              <p className="field-hint">还没有成功构建。新项目请改用上方「先构建再发布」，沿用已生成的 Jenkinsfile。</p>
            ) : null}
          </div>
          <div className="cd-field">
            <label className="cd-field-label">镜像 Tag</label>
            <input
              className="input"
              value={draft.imageTag}
              placeholder="例如 1.4.2 或构建 Run 的短 ID"
              onChange={(e) => onChange({ ...draft, imageTag: e.target.value })}
            />
          </div>
        </div>
      ) : (
        <div className="cd-form-grid two-col">
          <div className="cd-field">
            <label className="cd-field-label">引用类型</label>
            <select
              className="input"
              value={draft.gitRefType}
              onChange={(e) =>
                onChange({ ...draft, gitRefType: e.target.value as ReleaseDraft["gitRefType"] })
              }
            >
              <option value="branch">分支</option>
              <option value="tag">Tag</option>
            </select>
          </div>
          <div className="cd-field">
            <label className="cd-field-label">{draft.gitRefType === "tag" ? "Tag" : "分支"}</label>
            <input
              className="input"
              value={draft.gitRefValue}
              placeholder={draft.gitRefType === "tag" ? "例如 v1.4.2" : "例如 master"}
              onChange={(e) => onChange({ ...draft, gitRefValue: e.target.value })}
            />
          </div>
        </div>
      )}

      {!hasJenkinsTemplate ? (
        <p className="field-hint">未读到 Jenkins 模板绑定。仍可继续选 Pipeline；构建会在模板就绪后触发。</p>
      ) : null}
      {warnings.length > 0 ? <InlineAlert tone="warning">{warnings.join("；")}</InlineAlert> : null}
    </div>
  );
}

function StepPipeline({
  pipelines,
  selectedId,
  dagPreview,
  stages,
  skipBuild,
  onSelect,
}: {
  pipelines: Array<{ id: string; name: string; version: number; description: string }>;
  selectedId: string;
  dagPreview: ReturnType<typeof computeDagLayout>;
  stages: Array<{ id: string; steps: Array<{ uses: string }> }>;
  skipBuild: boolean;
  onSelect: (id: string) => void;
}) {
  return (
    <div className="stack-md">
      <p className="muted text-sm">选择发布编排。DAG 在 Pipeline Studio 维护，这里只读预览。</p>
      {skipBuild ? (
        <InlineAlert tone="info">当前选择「使用已有镜像」，将跳过 Pipeline 中的构建类步骤。</InlineAlert>
      ) : null}
      <div className="cd-wizard-template-grid">
        {pipelines.map((pipe) => (
          <button
            key={pipe.id}
            type="button"
            className={`cd-template-card${selectedId === pipe.id ? " is-selected" : ""}`}
            onClick={() => onSelect(pipe.id)}
          >
            <strong>{pipe.name}</strong>
            <span className="muted text-sm">v{pipe.version}</span>
            <p className="text-sm">{pipe.description}</p>
          </button>
        ))}
      </div>
      <div className="cd-dag-preview section">
        <div className="section-header">
          <h3 className="section-title">Pipeline 预览（只读）</h3>
          <Link href={`/delivery/pipelines/${selectedId}/studio`} className="link-inline text-sm">
            在 Studio 中编辑
          </Link>
        </div>
        <div className="cd-dag-preview-canvas" style={{ width: dagPreview.width, height: Math.min(dagPreview.height, 220) }}>
          <svg width={dagPreview.width} height={dagPreview.height}>
            {dagPreview.edges.map((edge) => {
              const from = dagPreview.nodes.find((n) => n.id === edge.from);
              const to = dagPreview.nodes.find((n) => n.id === edge.to);
              if (!from || !to) return null;
              return (
                <line
                  key={`${edge.from}-${edge.to}`}
                  x1={from.x + 120}
                  y1={from.y + 24}
                  x2={to.x}
                  y2={to.y + 24}
                  stroke="var(--border-strong)"
                />
              );
            })}
          </svg>
          {dagPreview.nodes.map((node) => {
            const stage = stages.find((s) => s.id === node.id);
            return (
              <div key={node.id} className="cd-dag-preview-node" style={{ left: node.x * 0.85, top: node.y * 0.85 }}>
                {node.id}
                <span>{stage?.steps[0]?.uses ?? ""}</span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function StepFleet({
  fleets,
  selectedIds,
  onChange,
}: {
  fleets: ReleasePreflightVM["fleets"];
  selectedIds: string[];
  onChange: (ids: string[]) => void;
}) {
  function toggle(id: string) {
    if (selectedIds.includes(id)) onChange(selectedIds.filter((item) => id !== item));
    else onChange([...selectedIds, id]);
  }

  return (
    <div className="stack-md">
      <p className="muted text-sm">选择目标 Fleet。Blocked 集群会在确认页标出，不阻止进入下一步。</p>
      {fleets.map((fleet) => (
        <label key={fleet.id} className={`cd-fleet-select-card${selectedIds.includes(fleet.id) ? " is-selected" : ""}`}>
          <input type="checkbox" checked={selectedIds.includes(fleet.id)} onChange={() => toggle(fleet.id)} />
          <div>
            <strong>{fleet.name}</strong>
            <div className="cd-fleet-select-clusters">
              {fleet.clusters.map((cluster) => (
                <span key={cluster.id} className={`cd-cluster-tag${cluster.ready ? "" : " is-blocked"}`}>
                  {cluster.name}
                  {cluster.wave ? ` · ${cluster.wave}` : ""}
                  {!cluster.ready ? ` (${cluster.reason})` : ""}
                </span>
              ))}
            </div>
          </div>
        </label>
      ))}
    </div>
  );
}

function StepConfirm({
  draft,
  pipeline,
  preflight,
  project,
  skipBuild,
}: {
  draft: ReleaseDraft;
  pipeline: ReturnType<typeof getMockPipeline>;
  preflight: ReleasePreflightVM;
  project: ProjectVM;
  skipBuild: boolean;
}) {
  const selectedFleets = preflight.fleets.filter((f) => draft.fleetIds.includes(f.id));
  const blockedClusters = selectedFleets.flatMap((fleet) =>
    fleet.clusters.filter((cluster) => !cluster.ready).map((cluster) => `${cluster.name}：${cluster.reason ?? "未就绪"}`),
  );

  return (
    <div className="stack-md">
      <div className="section">
        <h3 className="section-title">发布摘要</h3>
        <dl className="cd-summary-list">
          <div>
            <dt>项目</dt>
            <dd>{project.name}</dd>
          </div>
          <div>
            <dt>制品来源</dt>
            <dd>{draft.source === "artifact" ? "使用已有镜像" : "先构建再发布"}</dd>
          </div>
          <div>
            <dt>制品</dt>
            <dd>
              {draft.source === "artifact"
                ? imageCoordinate(draft.imageRepository, draft.imageTag)
                : draft.imageTag
                  ? imageCoordinate(draft.imageRepository, draft.imageTag)
                  : "构建成功后回填 Tag"}
            </dd>
          </div>
          <div>
            <dt>Pipeline</dt>
            <dd>{pipeline?.name ?? draft.pipelineId}</dd>
          </div>
          <div>
            <dt>发布策略</dt>
            <dd>{fleetStrategies(pipeline).join("；") || "—"}</dd>
          </div>
          <div>
            <dt>Git</dt>
            <dd>
              {draft.gitRefType}:{draft.gitRefValue || "—"}
            </dd>
          </div>
          <div>
            <dt>Fleet</dt>
            <dd>{selectedFleets.map((f) => f.name).join(", ") || "—"}</dd>
          </div>
        </dl>
      </div>
      {skipBuild ? <InlineAlert tone="info">将跳过 Pipeline 中的 jenkins.build 等构建步骤。</InlineAlert> : null}
      {draft.source === "build" ? (
        <InlineAlert tone="info">确认后会触发 Jenkins 构建。镜像就绪后再按所选 Pipeline / Fleet 做发布编排（分波观测为 Mock）。</InlineAlert>
      ) : (
        <InlineAlert tone="success">确认后不会调用 Jenkins。</InlineAlert>
      )}
      {blockedClusters.length > 0 ? (
        <InlineAlert tone="warning">部分集群未就绪：{blockedClusters.join("；")}</InlineAlert>
      ) : null}
    </div>
  );
}
