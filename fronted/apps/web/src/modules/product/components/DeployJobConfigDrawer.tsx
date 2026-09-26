"use client";

import { useEffect, useState } from "react";
import type { EnvironmentVM, ProductProjectVM } from "@sparkops/contracts/product";
import { PRODUCT_ENVIRONMENT_TYPES, environmentTypeLabel } from "@sparkops/contracts/product";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { cluster, delivery, secret, workspace } from "@sparkops/web-sdk";
import { Drawer } from "@/components/ui/Drawer";
import { BuildNotifyList } from "./BuildNotifyList";
import { DeployConfigResourceList, type ConfigResourceGroup } from "./DeployConfigResourceList";
import {
  DEPLOY_CONTENTS,
  buildConfigResourceItems,
  defaultConfigResources,
  deployJobSpecToRecord,
  parseDeployJobSpec,
  sourceImageLines,
  type ConfigResourceRef,
  type DeployJobSpec,
} from "./deploy-job-spec";
import type { EditorJob } from "./workflow-editor-utils";

type DeployTab = "basic" | "strategy" | "notify";

interface DeployJobConfigDrawerProps {
  open: boolean;
  workspaceId: string;
  jobName: string;
  jobType?: string;
  spec: Record<string, unknown> | undefined;
  sourceJobs: EditorJob[];
  sourceVersionJobs: EditorJob[];
  environments: EnvironmentVM[];
  projects: ProductProjectVM[];
  onClose: () => void;
  onConfirm: (name: string, spec: Record<string, unknown>) => void;
}

function isRegistry(item: SecretVM): boolean {
  const type = item.service_type.toLowerCase();
  return type.includes("harbor") || type.includes("registry") || type.includes("docker");
}

export function DeployJobConfigDrawer({
  open,
  workspaceId,
  jobName,
  jobType,
  spec,
  sourceJobs,
  sourceVersionJobs,
  environments,
  projects,
  onClose,
  onConfirm,
}: DeployJobConfigDrawerProps) {
  const [tab, setTab] = useState<DeployTab>("basic");
  const [name, setName] = useState(jobName);
  const [draft, setDraft] = useState<DeployJobSpec>(() => parseDeployJobSpec(spec, jobType));
  const [envType, setEnvType] = useState("test");
  const [projectDetails, setProjectDetails] = useState<ProjectVM[]>([]);
  const [registries, setRegistries] = useState<SecretVM[]>([]);
  const [configGroups, setConfigGroups] = useState<ConfigResourceGroup[]>([]);
  const [configLoading, setConfigLoading] = useState(false);
  const [configError, setConfigError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    const parsed = parseDeployJobSpec(spec, jobType);
    const current = environments.find((item) => item.id === parsed.environmentId);
    const versionJobs =
      parsed.artifactSource === "version"
        ? sourceVersionJobs.filter((job) => job.name.trim())
        : [];
    const nextDraft =
      parsed.artifactSource === "version" &&
      !parsed.jobName.trim() &&
      versionJobs.length === 1
        ? { ...parsed, jobName: versionJobs[0].name }
        : parsed;
    setTab("basic");
    setName(jobName);
    setDraft(nextDraft);
    setEnvType(current?.env_type || environments[0]?.env_type || "test");
    if (!workspaceId) return;
    let cancelled = false;
    void Promise.all([
      secret.listSecrets(workspaceId).catch(() => [] as SecretVM[]),
      workspace.listProjects(workspaceId).catch(() => [] as ProjectVM[]),
    ]).then(([secretItems, projectItems]) => {
      if (cancelled) return;
      const allowed = new Set(projects.map((item) => item.id));
      setRegistries(secretItems.filter(isRegistry));
      setProjectDetails(projectItems.filter((item) => allowed.has(item.id)));
    });
    return () => {
      cancelled = true;
    };
  }, [open, jobName, jobType, spec, workspaceId, environments, projects, sourceVersionJobs]);

  const envTypes = PRODUCT_ENVIRONMENT_TYPES.map((item) => item.id);
  const isVersionDeploy = draft.artifactSource === "version";
  const upstreamJobs = isVersionDeploy ? sourceVersionJobs : sourceJobs;

  const typedEnvironments = environments.filter((item) => (item.env_type || "test") === envType);
  const sourceJob = upstreamJobs.find((item) => item.name === draft.jobName);
  const images = sourceImageLines(
    sourceJob?.spec,
    registries.map((item) => ({ id: item.id, serviceUrl: item.service_url })),
    projects.map((item) => {
      const detail = projectDetails.find((project) => project.id === item.id);
      return {
        id: item.id,
        name: detail?.name || item.name,
        namespace: detail?.namespace,
        displayName: detail?.display_name || item.display_name,
      };
    }),
  );
  const knownSource = upstreamJobs.some((item) => item.name === draft.jobName);
  const configEnabled = !isVersionDeploy && draft.contents.includes("config");
  const canConfirm =
    name.trim().length > 0 &&
    draft.environmentId.trim().length > 0 &&
    draft.jobName.trim().length > 0 &&
    (isVersionDeploy || draft.contents.length > 0) &&
    !(configEnabled && configLoading);

  function toggleContent(id: string) {
    setDraft((current) => {
      const has = current.contents.includes(id);
      const contents = has ? current.contents.filter((item) => item !== id) : [...current.contents, id];
      return { ...current, contents };
    });
  }

  const configProjectKey = images.map((item) => `${item.projectId}:${item.name}`).join("|");

  useEffect(() => {
    if (!open || !configEnabled || !workspaceId) {
      setConfigGroups([]);
      setConfigLoading(false);
      setConfigError(null);
      return;
    }
    const targets = images.map((item) => ({ projectId: item.projectId, name: item.name }));
    if (targets.length === 0) {
      setConfigGroups([]);
      setConfigLoading(false);
      setConfigError(null);
      return;
    }
    let cancelled = false;
    setConfigLoading(true);
    setConfigError(null);
    void Promise.all(
      targets.map(async (target) => {
        const [mapsResult, previewResult] = await Promise.allSettled([
          cluster.listConfigMaps(target.projectId, workspaceId),
          delivery.getSyncPreview(target.projectId, workspaceId),
        ]);
        const maps = mapsResult.status === "fulfilled" ? mapsResult.value : [];
        const preview = previewResult.status === "fulfilled" ? previewResult.value : null;
        return {
          failed: mapsResult.status === "rejected" && previewResult.status === "rejected",
          group: {
            projectId: target.projectId,
            name: target.name,
            items: buildConfigResourceItems(target.projectId, maps, preview?.items ?? null),
          },
        };
      }),
    ).then((results) => {
      if (cancelled) return;
      const groups = results.map((item) => item.group);
      setConfigGroups(groups);
      setConfigLoading(false);
      setConfigError(results.every((item) => item.failed) ? "读取配置资源失败，请确认项目已连接到集群" : null);
      setDraft((current) => {
        if (current.configResourcesSet || results.every((item) => item.failed)) return current;
        return {
          ...current,
          configResources: defaultConfigResources(groups.flatMap((group) => group.items)),
          configResourcesSet: true,
        };
      });
    });
    return () => {
      cancelled = true;
    };
  }, [open, configEnabled, workspaceId, configProjectKey]);

  function toggleConfigResource(item: ConfigResourceRef, selected: boolean) {
    setDraft((current) => {
      const rest = current.configResources.filter(
        (row) => !(row.projectId === item.projectId && row.kind === item.kind && row.name === item.name),
      );
      return {
        ...current,
        configResourcesSet: true,
        configResources: selected
          ? [...rest, { projectId: item.projectId, kind: item.kind, name: item.name, yaml: item.yaml ?? "" }]
          : rest,
      };
    });
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title="部署"
      panelClassName="drawer-panel-wide pwf-build-drawer"
      tabs={
        <>
          {(
            [
              ["basic", "基础配置"],
              ["strategy", "策略配置"],
              ["notify", "通知配置"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              className={`drawer-tab${tab === id ? " drawer-tab-active" : ""}`}
              onClick={() => setTab(id)}
            >
              {label}
            </button>
          ))}
          <span className="pwf-build-tab-spacer" />
          <button
            type="button"
            className="btn btn-primary text-sm"
            disabled={!canConfirm}
            onClick={() => {
              const picked = upstreamJobs.find((item) => item.name === draft.jobName);
              const sourceKind =
                draft.artifactSource === "version"
                  ? picked?.type === "sparkops-create-version"
                    ? "create-version"
                    : picked?.type === "sparkops-build"
                      ? "build"
                      : draft.sourceKind
                  : draft.sourceKind;
              onConfirm(
                name.trim(),
                deployJobSpecToRecord({ ...draft, sourceKind, jobName: draft.jobName.trim() }),
              );
            }}
          >
            确定
          </button>
          <button type="button" className="btn btn-secondary text-sm" onClick={onClose}>
            取消
          </button>
        </>
      }
    >
      {tab === "basic" ? (
        <div className="pwf-build-form">
          <label className="pwf-build-field">
            <span>任务名称</span>
            <span className="pwf-build-control">
              <span className="pwf-required" aria-hidden>
                *
              </span>
              <input className="input" value={name} aria-label="任务名称" onChange={(e) => setName(e.target.value)} />
            </span>
          </label>

          <div className="pwf-build-field">
            <span>环境</span>
            <span className="pwf-build-control">
              <span className="pwf-required" aria-hidden>
                *
              </span>
              <span className="pwf-deploy-env">
                <select
                  className="input"
                  aria-label="环境类型"
                  value={envType}
                  onChange={(e) => {
                    const nextType = e.target.value;
                    setEnvType(nextType);
                    setDraft((current) => {
                      const stillValid = environments.some(
                        (item) => item.id === current.environmentId && (item.env_type || "test") === nextType,
                      );
                      return stillValid ? current : { ...current, environmentId: "" };
                    });
                  }}
                >
                  {envTypes.map((type) => (
                    <option key={type} value={type}>
                      {environmentTypeLabel(type)}
                    </option>
                  ))}
                </select>
                <select
                  className="input"
                  aria-label="环境"
                  value={draft.environmentId}
                  onChange={(e) => setDraft((current) => ({ ...current, environmentId: e.target.value }))}
                >
                  <option value="">{typedEnvironments.length === 0 ? "该类型下还没有环境" : "请选择环境"}</option>
                  {typedEnvironments.map((env) => (
                    <option key={env.id} value={env.id}>
                      {env.name}
                    </option>
                  ))}
                </select>
              </span>
            </span>
          </div>

          <label className="pwf-build-field">
            <span>部署来源</span>
            <span className="pwf-build-control">
              <span className="pwf-required" aria-hidden>
                *
              </span>
              <select
                className="input"
                aria-label="部署来源"
                value={draft.artifactSource}
                onChange={(e) => {
                  const artifactSource = e.target.value === "version" ? "version" : "build";
                  const versionJobs = sourceVersionJobs.filter((job) => job.name.trim());
                  setDraft((current) => ({
                    ...current,
                    artifactSource,
                    jobName:
                      artifactSource === "version" && versionJobs.length === 1 ? versionJobs[0].name : "",
                    configResourcesSet: false,
                    configResources: [],
                  }));
                }}
              >
                <option value="build">构建产物</option>
                <option value="version">交付版本</option>
              </select>
            </span>
          </label>

          <label className="pwf-build-field">
            <span>{isVersionDeploy ? "版本来源 Job" : "镜像来源 Job"}</span>
            <span className="pwf-build-control">
              <span className="pwf-required" aria-hidden>
                *
              </span>
              <select
                className="input"
                aria-label={isVersionDeploy ? "版本来源 Job" : "镜像来源 Job"}
                value={draft.jobName}
                onChange={(e) => setDraft((current) => ({ ...current, jobName: e.target.value }))}
              >
                <option value="">
                  {isVersionDeploy ? "请选择上游构建或创建版本任务" : "请选择上游构建任务"}
                </option>
                {upstreamJobs.map((job, index) => (
                  <option key={`${job.name}-${index}`} value={job.name}>
                    {job.name}
                    {isVersionDeploy
                      ? job.type === "sparkops-build"
                        ? "（构建）"
                        : "（创建交付版本）"
                      : ""}
                  </option>
                ))}
                {draft.jobName && !knownSource ? <option value={draft.jobName}>{draft.jobName}（已不在上游）</option> : null}
              </select>
              {isVersionDeploy && upstreamJobs.length === 0 ? (
                <span className="pwf-deploy-empty-hint">
                  当前部署任务之前没有可用的上游任务。请在本阶段或上游阶段添加「SparkOps 构建」或「创建交付版本」，并排在部署之前。
                </span>
              ) : null}
            </span>
          </label>

          {!isVersionDeploy ? (
            <div className="pwf-build-field pwf-build-field-top">
              <span>镜像名称</span>
              <div className="pwf-deploy-images">
                {!draft.jobName ? <span className="text-xs text-muted">选择上游构建任务后显示</span> : null}
                {draft.jobName && !sourceJob ? (
                  <span className="text-xs text-muted">该构建任务已不在上游，无法显示镜像名称</span>
                ) : null}
                {sourceJob && images.length === 0 ? <span className="text-xs text-muted">该构建任务还没有项目组件</span> : null}
                {images.map((line) => (
                  <div key={line.projectId} className="pwf-deploy-image">
                    <span>{line.name}</span>
                    <code>{line.image}</code>
                  </div>
                ))}
                {images.some((line) => !line.complete) ? (
                  <span className="text-xs text-muted">镜像仓库或命名空间未配置时，只显示项目名和 Tag</span>
                ) : null}
              </div>
            </div>
          ) : upstreamJobs.length > 0 ? (
            <p className="text-xs text-muted pwf-deploy-version-hint">
              {sourceJob?.type === "sparkops-build"
                ? "将基于上游构建产物自动创建交付版本并部署；运行工作流时需填写版本号。"
                : "将使用上游「创建交付版本」任务产生的 YAML 快照与镜像坐标进行部署。"}
            </p>
          ) : null}

          {!isVersionDeploy ? (
            <div className="pwf-build-field">
              <span>部署内容</span>
              <span className="pwf-build-control">
                <span className="pwf-required" aria-hidden>
                  *
                </span>
                <span className="pwf-deploy-checks">
                  {DEPLOY_CONTENTS.map((item) => (
                    <label key={item.id} className="pwf-check">
                      <input
                        type="checkbox"
                        checked={draft.contents.includes(item.id)}
                        onChange={() => toggleContent(item.id)}
                      />
                      {item.label}
                    </label>
                  ))}
                </span>
              </span>
            </div>
          ) : null}

          {configEnabled ? (
            <div className="pwf-build-field pwf-build-field-top">
              <span>配置资源</span>
              <DeployConfigResourceList
                loading={configLoading}
                error={configError}
                groups={configGroups}
                selected={draft.configResources}
                onToggle={toggleConfigResource}
              />
            </div>
          ) : null}

          <div className="pwf-build-field">
            <span title="部署完成后检查工作负载是否就绪">状态检测</span>
            <button
              type="button"
              className={`menu-switch${draft.statusCheck ? " menu-switch-on" : ""}`}
              aria-pressed={draft.statusCheck}
              aria-label="状态检测"
              onClick={() => setDraft((current) => ({ ...current, statusCheck: !current.statusCheck }))}
            >
              <span className="menu-switch-thumb" />
            </button>
          </div>
        </div>
      ) : null}

      {tab === "strategy" ? (
        <div className="pwf-build-form">
          <label className="pwf-build-field">
            <span>超时（分钟）</span>
            <input
              className="input"
              type="number"
              min={1}
              aria-label="超时（分钟）"
              value={draft.timeoutMinutes}
              onChange={(e) =>
                setDraft((current) => ({
                  ...current,
                  timeoutMinutes: Math.max(1, Number(e.target.value) || 60),
                }))
              }
            />
          </label>
        </div>
      ) : null}

      {tab === "notify" ? <BuildNotifyList items={draft.notify} onChange={(notify) => setDraft((current) => ({ ...current, notify }))} /> : null}
    </Drawer>
  );
}
