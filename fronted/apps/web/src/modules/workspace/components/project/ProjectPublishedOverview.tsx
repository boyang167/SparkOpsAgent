"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { cluster } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type {
  DeploymentOverviewVM,
  PodVM,
  ProjectRuntimeOverviewVM,
  ServiceOverviewVM,
} from "@sparkops/contracts/cluster";
import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { K8sEmptyState, useK8sConfirm } from "@/components/k8s";
import { PodDetailPanel, type PodDetailTab } from "@/modules/runtime/components/PodDetailPanel";
import {
  RuntimeDeploymentYamlModal,
  RuntimeHealthProbeModal,
  RuntimeLogCollectionModal,
  RuntimePodYamlModal,
  RuntimeServicePortModal,
  RuntimeServiceYamlModal,
  ServiceYamlIcon,
} from "@/modules/runtime/components/RuntimeConfigModals";
import { formatDateTime } from "./project-utils";
import { deploymentRollbackSourceTitle } from "./deployment-console-utils";
import { latestSuccessfulDeployment } from "./project-deployment-state";

interface ProjectPublishedOverviewProps {
  workspaceId: string;
  project: ProjectVM;
  deploymentPage: DeploymentRunListVM | null;
  refreshKey?: number;
}

function phaseBadgeClass(phase: string) {
  if (phase === "Running" || phase === "Succeeded") return "badge-success";
  if (phase === "Pending") return "badge-warning";
  return "badge-warning";
}

function labelTags(labels: Record<string, string> | undefined) {
  if (!labels || Object.keys(labels).length === 0) {
    return <span>-</span>;
  }
  return (
    <span className="project-unpublished-tags">
      {Object.entries(labels).map(([key, value]) => (
        <span key={key} className="project-tag project-tag-gray">
          {key}:{value}
        </span>
      ))}
    </span>
  );
}

function endpointTags(endpoints: string[] | undefined) {
  if (!endpoints?.length) return <span>-</span>;
  return (
    <span className="project-unpublished-tags">
      {endpoints.map((endpoint) => (
        <span key={endpoint} className="project-tag project-tag-gray">
          {endpoint}
        </span>
      ))}
    </span>
  );
}

function servicePortTags(ports: ServiceOverviewVM["ports"]) {
  if (!ports?.length) return <span>-</span>;
  return (
    <span className="project-unpublished-tags">
      {ports.map((port, index) => (
        <span key={`${port.port}-${port.protocol}-${index}`} className="project-tag project-tag-gray">
          {port.port}-{port.protocol}
          {port.name ? `-${port.name}` : ""}
          {port.target_port ? `-${port.target_port}` : ""}
        </span>
      ))}
    </span>
  );
}

function RuntimeSectionCard({
  title,
  action,
  children,
}: {
  title: string;
  action?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <section className="project-runtime-card">
      <header className="project-runtime-card-header">
        <h4 className="project-runtime-card-title">{title}</h4>
        {action}
      </header>
      {children}
    </section>
  );
}


function probeTypeLabel(probe: ProjectRuntimeOverviewVM["health_probe"]) {
  if (!probe?.configured) return "-";
  const types: string[] = [];
  if (probe.readiness) types.push(probe.protocol || "TCP");
  if (probe.liveness && !probe.readiness) types.push(probe.protocol || "TCP");
  return types[0] || probe.protocol || "TCP";
}

function replicaStatus(deployment: DeploymentOverviewVM | undefined) {
  if (!deployment) return <span>-</span>;
  const available = deployment.available_replicas ?? 0;
  const total = deployment.replicas ?? 0;
  const failed = deployment.unavailable_replicas ?? 0;
  return (
    <span className="project-unpublished-tags">
      <span className="project-tag project-tag-green">
        {available} / {total} 个运行中
      </span>
      {failed > 0 ? <span className="project-tag project-tag-red">{failed} 个失败</span> : null}
    </span>
  );
}

function DetailItem({ label, value, wide = false }: { label: string; value: ReactNode; wide?: boolean }) {
  return (
    <div className={`project-runtime-detail-item${wide ? " is-wide" : ""}`}>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function formatCpuCores(millicores: number | undefined) {
  if (millicores === undefined || millicores < 0) return "-";
  if (millicores === 0) return "0";
  const cores = millicores / 1000;
  if (cores >= 1) return cores.toFixed(2).replace(/\.?0+$/, "");
  if (cores >= 0.01) return cores.toFixed(3).replace(/\.?0+$/, "");
  return `${millicores}m`;
}

function formatMemoryMiB(bytes: number | undefined) {
  if (bytes === undefined || bytes < 0) return "-";
  if (bytes === 0) return "0 MiB";
  const mib = bytes / (1024 * 1024);
  if (mib < 0.01) return "<0.01 MiB";
  if (mib < 10) return `${mib.toFixed(2)} MiB`;
  return `${mib.toFixed(1)} MiB`;
}

function metricLevel(value: number | undefined, max: number) {
  if (value === undefined || value <= 0 || max <= 0) return 12;
  return Math.max(12, Math.min(100, Math.round((value / max) * 100)));
}

function MetricSparkline({
  tone,
  value,
  max,
  label,
}: {
  tone: "cpu" | "memory";
  value: number | undefined;
  max: number;
  label: string;
}) {
  const level = metricLevel(value, max);
  return (
    <span className="project-runtime-metric">
      <span
        className={`project-runtime-sparkline project-runtime-sparkline-${tone}`}
        style={{ ["--metric-level" as string]: `${level}%` }}
        aria-hidden
      />
      <span className="project-runtime-metric-value">{label}</span>
    </span>
  );
}

export function ProjectPublishedOverview({
  workspaceId,
  project,
  deploymentPage,
  refreshKey = 0,
}: ProjectPublishedOverviewProps) {
  const [overview, setOverview] = useState<ProjectRuntimeOverviewVM | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [podPanel, setPodPanel] = useState<{ pod: PodVM; tab: PodDetailTab } | null>(null);
  const [restartingPodId, setRestartingPodId] = useState<string | null>(null);
  const [logModalOpen, setLogModalOpen] = useState(false);
  const [probeModalOpen, setProbeModalOpen] = useState(false);
  const [portModalOpen, setPortModalOpen] = useState(false);
  const [serviceYamlModalOpen, setServiceYamlModalOpen] = useState(false);
  const [deploymentYamlModalOpen, setDeploymentYamlModalOpen] = useState(false);
  const [podYamlModalPod, setPodYamlModalPod] = useState<PodVM | null>(null);
  const [savingRuntime, setSavingRuntime] = useState(false);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const latestDeploy = latestSuccessfulDeployment(deploymentPage);
  const pods = overview?.pods ?? [];
  const deployment = overview?.deployment;
  const service = overview?.service;
  const logCollection = overview?.log_collection;
  const healthProbe = overview?.health_probe;
  const ingress = overview?.ingress;
  const namespace = deployment?.namespace || project.namespace || workspaceId;
  const imageTag = deployment?.image_tag || latestDeploy?.image_tag;
  const memoryLimit = deployment?.memory_limit || project.resources?.memory_limit;
  const maxCpuMillicores = useMemo(
    () => Math.max(1000, ...pods.map((pod) => pod.cpu_millicores ?? 0)),
    [pods],
  );
  const maxMemoryBytes = useMemo(
    () => Math.max(1024 * 1024, ...pods.map((pod) => pod.memory_bytes ?? 0)),
    [pods],
  );

  const loadOverview = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await cluster.getProjectRuntimeOverview(project.id, workspaceId);
      setOverview(data);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载运行概览失败");
      setOverview(null);
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    void loadOverview();
  }, [loadOverview, refreshKey]);

  const handleRestartPod = useCallback(
    async (pod: PodVM) => {
      const ok = await ask({
        title: "重启 Pod",
        description: `确定重启 Pod「${pod.name}」吗？`,
        confirmLabel: "重启",
        danger: true,
      });
      if (!ok) return;
      setRestartingPodId(pod.id);
      setError(null);
      try {
        await cluster.restartPod(project.id, pod.id, workspaceId);
        await loadOverview();
      } catch (err) {
        setError(err instanceof BffError ? err.message : "重启 Pod 失败");
      } finally {
        setRestartingPodId(null);
      }
    },
    [ask, loadOverview, project.id, workspaceId],
  );

  const handleCreateService = useCallback(async () => {
    const ok = await ask({
      title: "创建 Service",
      description: "确定创建 Service 吗？",
      confirmLabel: "创建",
      danger: false,
    });
    if (!ok) return;
    setSavingRuntime(true);
    setError(null);
    try {
      await cluster.createRuntimeService(project.id, workspaceId);
      await loadOverview();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建 Service 失败");
    } finally {
      setSavingRuntime(false);
    }
  }, [ask, loadOverview, project.id, workspaceId]);

  const handleCreateIngress = useCallback(async () => {
    const ok = await ask({
      title: "添加外部访问",
      description: "确定添加外部访问地址吗？将自动创建 Ingress。",
      confirmLabel: "创建",
      danger: false,
    });
    if (!ok) return;
    setSavingRuntime(true);
    setError(null);
    try {
      await cluster.createRuntimeIngress(project.id, workspaceId);
      await loadOverview();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建 Ingress 失败");
    } finally {
      setSavingRuntime(false);
    }
  }, [ask, loadOverview, project.id, workspaceId]);

  return (
    <div className="project-runtime-overview">
      {error ? <p className="error">{error}</p> : null}

      <section className="project-runtime-card">
        <header className="project-runtime-card-header">
          <h4 className="project-runtime-card-title">详情</h4>
          <button
            type="button"
            className="project-runtime-card-action"
            onClick={() => void loadOverview()}
            disabled={loading}
            title="刷新"
            aria-label="刷新"
          >
            ↻
          </button>
        </header>

        <dl className="project-runtime-detail-grid">
          <DetailItem label="名称" value={deployment?.name || project.name} />
          <DetailItem label="命名空间" value={namespace} />
          <DetailItem
            label="创建时间"
            value={
              deployment?.created_at
                ? formatDateTime(deployment.created_at)
                : latestDeploy?.finished_at
                  ? formatDateTime(latestDeploy.finished_at)
                  : "-"
            }
          />

          <DetailItem
            label="镜像"
            value={deployment?.image || (imageTag ? `${project.name}:${imageTag}` : "-")}
          />
          <DetailItem
            label="当前镜像版本"
            value={
              imageTag ? <span className="project-tag project-tag-blue">{imageTag}</span> : "-"
            }
          />
          <DetailItem label="升级策略" value={deployment?.strategy || "RollingUpdate"} />

          <DetailItem label="容器组" value={loading ? "加载中…" : replicaStatus(deployment)} />
          <DetailItem
            label="配置资源"
            value={
              memoryLimit ? (
                <span className="project-tag project-tag-blue">最大内存: {memoryLimit}</span>
              ) : (
                "-"
              )
            }
          />
          <DetailItem label="命令、参数" value="-" />

          <DetailItem
            label="Git"
            value={
              project.git_url ? <span className="project-tag project-tag-gray">{project.git_url}</span> : "-"
            }
          />
          <DetailItem
            label="TOKEN"
            value={
              deployment?.uid ? (
                <span className="project-tag project-tag-orange">{deployment.uid}</span>
              ) : (
                "-"
              )
            }
          />
          <DetailItem label="Hosts" value="-" />

          {latestDeploy?.run_type === "rollback" && latestDeploy.rollback_of_run_id ? (
            <DetailItem
              wide
              label="回滚来源"
              value={
                <span className="project-tag project-tag-orange">
                  {deploymentRollbackSourceTitle(latestDeploy, deploymentPage?.items ?? [])}
                </span>
              }
            />
          ) : null}
          <DetailItem wide label="标签" value={loading ? "加载中…" : labelTags(deployment?.labels)} />
        </dl>
      </section>

      <section className="project-runtime-card">
        <header className="project-runtime-card-header">
          <h4 className="project-runtime-card-title">容器组</h4>
          <div className="project-runtime-card-actions">
            <button
              type="button"
              className="project-runtime-card-yaml-icon"
              onClick={() => setDeploymentYamlModalOpen(true)}
              disabled={loading || !deployment}
              title="查看Deployment YAML信息"
              aria-label="查看Deployment YAML信息"
            >
              <ServiceYamlIcon />
            </button>
          </div>
        </header>

        {!loading && pods.length === 0 ? (
          <K8sEmptyState
            variant="empty"
            title="暂无 Pod"
            description="请确认 Deployment 已成功调度。"
            compact
          />
        ) : (
          <div className="project-runtime-table-wrap">
            <table className="project-runtime-table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>节点</th>
                  <th>状态</th>
                  <th>已重启</th>
                  <th>CPU（核）</th>
                  <th>内存（MiB）</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {pods.map((pod) => (
                  <tr key={pod.id}>
                    <td>
                      <button
                        type="button"
                        className="project-runtime-pod-link"
                        onClick={() => setPodPanel({ pod, tab: "overview" })}
                      >
                        {pod.name}
                      </button>
                      {pod.message ? (
                        <div className="project-runtime-pod-message">{pod.message}</div>
                      ) : null}
                    </td>
                    <td>{pod.node || "-"}</td>
                    <td>
                      <span className={`badge ${phaseBadgeClass(pod.phase)}`}>{pod.phase}</span>
                    </td>
                    <td>{pod.restarts}</td>
                    <td>
                      <MetricSparkline
                        tone="cpu"
                        value={pod.cpu_millicores}
                        max={maxCpuMillicores}
                        label={formatCpuCores(pod.cpu_millicores)}
                      />
                    </td>
                    <td>
                      <MetricSparkline
                        tone="memory"
                        value={pod.memory_bytes}
                        max={maxMemoryBytes}
                        label={formatMemoryMiB(pod.memory_bytes)}
                      />
                    </td>
                    <td>{pod.started_at ? formatDateTime(pod.started_at) : "-"}</td>
                    <td>
                      <div className="project-runtime-row-actions">
                        <button
                          type="button"
                          className="project-runtime-action"
                          disabled={loading || restartingPodId === pod.id}
                          onClick={() => void handleRestartPod(pod)}
                        >
                          {restartingPodId === pod.id ? "重启中…" : "重启"}
                        </button>
                        <span className="project-runtime-action-sep">|</span>
                        <button
                          type="button"
                          className="project-runtime-action"
                          onClick={() => setPodPanel({ pod, tab: "logs" })}
                        >
                          日志
                        </button>
                        <span className="project-runtime-action-sep">|</span>
                        <button
                          type="button"
                          className="project-runtime-action"
                          onClick={() => setPodPanel({ pod, tab: "console" })}
                        >
                          控制台
                        </button>
                        <span className="project-runtime-action-sep">|</span>
                        <button
                          type="button"
                          className="project-runtime-card-yaml-icon project-runtime-row-yaml-icon"
                          onClick={() => setPodYamlModalPod(pod)}
                          title="查看Pod YAML信息"
                          aria-label="查看Pod YAML信息"
                        >
                          <ServiceYamlIcon />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {!loading && service ? (
        <RuntimeSectionCard
          title="Service"
          action={
            <div className="project-runtime-card-actions">
              <button
                type="button"
                className="project-runtime-card-add"
                onClick={() => setPortModalOpen(true)}
                disabled={loading || savingRuntime}
              >
                调整端口
              </button>
              <button
                type="button"
                className="project-runtime-card-yaml-icon"
                onClick={() => setServiceYamlModalOpen(true)}
                disabled={loading || savingRuntime}
                title="查看Service YAML信息"
                aria-label="查看Service YAML信息"
              >
                <ServiceYamlIcon />
              </button>
            </div>
          }
        >
          <dl className="project-runtime-detail-grid">
            <DetailItem label="名称" value={service.name} />
            <DetailItem label="标签" value={labelTags(service.labels)} />
            <DetailItem label="选择器" value={labelTags(service.selector)} />
            <DetailItem label="端口" value={servicePortTags(service.ports)} />
            <DetailItem label="注释" value={labelTags(service.annotations)} />
            <DetailItem label="端点" value={endpointTags(service.endpoints)} />
            <DetailItem
              label="创建时间"
              value={service.created_at ? formatDateTime(service.created_at) : "-"}
            />
          </dl>
        </RuntimeSectionCard>
      ) : (
        <RuntimeSectionCard
          title="Service"
          action={
            <button
              type="button"
              className="project-runtime-card-add"
              onClick={() => void handleCreateService()}
              disabled={loading || savingRuntime}
            >
              创建服务
            </button>
          }
        >
          {!loading ? <K8sEmptyState variant="empty" title="暂无 Service" compact /> : null}
        </RuntimeSectionCard>
      )}

      <RuntimeSectionCard
        title="外部地址"
        action={
          !ingress ? (
            <button
              type="button"
              className="project-runtime-card-add"
              onClick={() => void handleCreateIngress()}
              disabled={loading || savingRuntime || !service}
              title={!service ? "请先创建 Service" : undefined}
            >
              添加
            </button>
          ) : null
        }
      >
        {ingress ? (
          <dl className="project-runtime-detail-grid">
            <DetailItem
              label="域名"
              value={
                <a href={`http://${ingress.host}`} target="_blank" rel="noreferrer">
                  {ingress.host}
                </a>
              }
            />
            {(ingress.paths ?? []).map((item, index) => (
              <DetailItem
                key={`${item.path}-${index}`}
                label={`路径 ${index + 1}`}
                value={`${item.path} → ${item.service_name}:${item.service_port}`}
              />
            ))}
            <DetailItem
              label="创建时间"
              value={ingress.created_at ? formatDateTime(ingress.created_at) : "-"}
            />
          </dl>
        ) : !loading ? (
          <K8sEmptyState variant="empty" title="暂无外部访问地址" compact />
        ) : null}
      </RuntimeSectionCard>

      <RuntimeSectionCard
        title="日志采集"
        action={
          <button
            type="button"
            className="project-runtime-card-add"
            onClick={() => setLogModalOpen(true)}
            disabled={loading || savingRuntime}
          >
            {logCollection?.configured ? "编辑" : "添加"}
          </button>
        }
      >
        {logCollection?.configured ? (
          <dl className="project-runtime-detail-grid">
            <DetailItem label="日志路径" value={logCollection.path || "-"} />
            <DetailItem label="收集规则" value={logCollection.pattern || "-"} />
            <DetailItem label="文件后缀" value={logCollection.file_suffix || "*.log"} />
          </dl>
        ) : !loading ? (
          <K8sEmptyState variant="empty" title="尚未配置日志采集" compact />
        ) : null}
      </RuntimeSectionCard>

      <RuntimeSectionCard
        title="健康检测"
        action={
          <button
            type="button"
            className="project-runtime-card-add"
            onClick={() => setProbeModalOpen(true)}
            disabled={loading || savingRuntime}
          >
            {healthProbe?.configured ? "编辑" : "添加"}
          </button>
        }
      >
        {healthProbe?.configured ? (
          <dl className="project-runtime-detail-grid">
            <DetailItem label="就绪" value={probeTypeLabel(healthProbe)} />
            <DetailItem label="端口" value={healthProbe.port ?? "-"} />
            <DetailItem label="初始化等候时间" value={`${healthProbe.initial_delay_seconds ?? "-"}s`} />
            <DetailItem label="检测时间间隔" value={`${healthProbe.period_seconds ?? "-"}s`} />
            <DetailItem label="检测超时时间" value={`${healthProbe.timeout_seconds ?? "-"}s`} />
            <DetailItem label="连续成功次数" value={healthProbe.success_threshold ?? "-"} />
            <DetailItem label="连续失败次数" value={healthProbe.failure_threshold ?? "-"} />
          </dl>
        ) : !loading ? (
          <K8sEmptyState variant="empty" title="尚未配置健康检测" compact />
        ) : null}
      </RuntimeSectionCard>

      <RuntimeLogCollectionModal
        open={logModalOpen}
        projectId={project.id}
        workspaceId={workspaceId}
        initial={logCollection}
        onClose={() => setLogModalOpen(false)}
        onSaved={() => void loadOverview()}
      />
      <RuntimeHealthProbeModal
        open={probeModalOpen}
        projectId={project.id}
        workspaceId={workspaceId}
        initial={healthProbe}
        onClose={() => setProbeModalOpen(false)}
        onSaved={() => void loadOverview()}
      />
      <RuntimeServicePortModal
        open={portModalOpen}
        projectId={project.id}
        workspaceId={workspaceId}
        service={service}
        onClose={() => setPortModalOpen(false)}
        onSaved={() => void loadOverview()}
      />
      <RuntimeServiceYamlModal
        open={serviceYamlModalOpen}
        service={service}
        onClose={() => setServiceYamlModalOpen(false)}
      />
      <RuntimeDeploymentYamlModal
        open={deploymentYamlModalOpen}
        projectId={project.id}
        workspaceId={workspaceId}
        deployment={deployment}
        pod={pods[0]}
        onClose={() => setDeploymentYamlModalOpen(false)}
      />
      <RuntimePodYamlModal
        open={Boolean(podYamlModalPod)}
        projectId={project.id}
        workspaceId={workspaceId}
        pod={podYamlModalPod}
        onClose={() => setPodYamlModalPod(null)}
      />

      {podPanel ? (
        <PodDetailPanel
          workspaceId={workspaceId}
          projectId={project.id}
          pod={podPanel.pod}
          initialTab={podPanel.tab}
          onClose={() => setPodPanel(null)}
        />
      ) : null}
      {confirmDialog}
    </div>
  );
}
