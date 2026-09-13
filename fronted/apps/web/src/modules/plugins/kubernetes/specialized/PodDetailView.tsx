"use client";

import { useState } from "react";
import type { JSX, ReactNode } from "react";
import { K8sEmptyState, K8sLoadingState } from "@/components/k8s";
import { ApiErrorBanner } from "../ApiErrorBanner";
import type { ApiErrorInfo } from "../api";
import {
  eventTimestamp,
  formatContainerCommand,
  formatContainerPorts,
  formatEnvironment,
  sortPodEvents,
  type PodEvent,
  volumeSourceView,
} from "../pod-detail-view";
import { formatDateTime, podPhase, type PodObject, type QuantityMap } from "../node-view";

type PodCondition = {
  type?: string;
  status?: string;
  lastTransitionTime?: string;
  reason?: string;
  message?: string;
};

type OwnerReference = {
  name?: string;
  kind?: string;
};

type ContainerPort = {
  containerPort?: number | string;
  protocol?: string;
};

type EnvVar = {
  name?: string;
  value?: string;
  valueFrom?: Record<string, unknown>;
};

type VolumeMount = {
  name?: string;
  mountPath?: string;
  readOnly?: boolean;
  subPath?: string;
};

type ContainerSpec = {
  name?: string;
  image?: string;
  imagePullPolicy?: string;
  ports?: ContainerPort[];
  env?: EnvVar[];
  command?: unknown[];
  args?: unknown[];
  resources?: {
    requests?: QuantityMap;
    limits?: QuantityMap;
  };
  volumeMounts?: VolumeMount[];
};

type VolumeSpec = {
  name?: string;
} & Record<string, unknown>;

type RichPodObject = Omit<PodObject, "metadata" | "spec" | "status"> & {
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
    ownerReferences?: OwnerReference[];
  };
  spec?: {
    nodeName?: string;
    containers?: ContainerSpec[];
    initContainers?: ContainerSpec[];
    volumes?: VolumeSpec[];
  };
  status?: {
    phase?: string;
    podIP?: string;
    conditions?: PodCondition[];
  };
};

const TABS = [
  { id: "containers", label: "容器" },
  { id: "events", label: "事件" },
  { id: "owners", label: "创建者" },
  { id: "init-containers", label: "初始化容器" },
  { id: "volumes", label: "存储" },
] as const;

export function PodDetailView({
  pod,
  events,
  eventsError,
  eventsLoading = false,
}: {
  pod: PodObject;
  events: PodEvent[];
  eventsError?: ApiErrorInfo | null;
  eventsLoading?: boolean;
}): JSX.Element {
  const data = pod as RichPodObject;
  const [activeTab, setActiveTab] = useState<(typeof TABS)[number]["id"]>("containers");
  const tabBase = `k8s-pod-detail-${stableId(data.metadata?.namespace)}-${stableId(data.metadata?.name)}`;

  return (
    <div className="k8s-pod-detail-root stack-lg">
      <section className="k8s-pod-detail-section stack-sm">
        <h3 className="k8s-pod-detail-title">基本信息</h3>
        <div className="k8s-pod-detail-grid">
          <InfoRow
            left={<Field label="名称">{dash(data.metadata?.name)}</Field>}
            right={<Field label="命名空间">{dash(data.metadata?.namespace)}</Field>}
          />
          <InfoRow
            left={<Field label="状态">{podPhase(pod)}</Field>}
            right={<Field label="创建时间">{formatDateTime(data.metadata?.creationTimestamp)}</Field>}
          />
          <InfoRow
            left={<Field label="节点">{dash(data.spec?.nodeName)}</Field>}
            right={<Field label="Pod IP">{dash(data.status?.podIP)}</Field>}
          />
          <InfoRow
            left={
              <Field label="标签">
                <MetaPills map={data.metadata?.labels} />
              </Field>
            }
            right={
              <Field label="注解">
                <MetaPills map={data.metadata?.annotations} />
              </Field>
            }
          />
        </div>
      </section>

      <section className="k8s-pod-detail-section stack-sm">
        <h3 className="k8s-pod-detail-title">现状详情</h3>
        <ConditionsTable conditions={data.status?.conditions ?? []} />
      </section>

      <section className="k8s-pod-detail-section stack-sm">
        <div className="k8s-pod-detail-tabs">
          <div className="k8s-pod-detail-tab-list" role="tablist" aria-label="Pod 详情分类">
            {TABS.map((tab) => {
              const selected = activeTab === tab.id;
              const tabId = `${tabBase}-tab-${tab.id}`;
              const panelId = `${tabBase}-panel-${tab.id}`;
              return (
                <button
                  key={tab.id}
                  id={tabId}
                  type="button"
                  role="tab"
                  aria-selected={selected}
                  aria-controls={panelId}
                  className={selected ? "k8s-pod-detail-tab k8s-pod-detail-tab-active" : "k8s-pod-detail-tab"}
                  onClick={() => setActiveTab(tab.id)}
                >
                  {tab.label}
                </button>
              );
            })}
          </div>

          <div className="k8s-pod-detail-tab-panels">
            {TABS.map((tab) => {
              const selected = activeTab === tab.id;
              const tabId = `${tabBase}-tab-${tab.id}`;
              const panelId = `${tabBase}-panel-${tab.id}`;
              return (
                <div
                  key={tab.id}
                  id={panelId}
                  className="k8s-pod-detail-tab-panel"
                  role="tabpanel"
                  aria-labelledby={tabId}
                  hidden={!selected}
                >
                  {tab.id === "containers" ? (
                    <ContainersTable containers={data.spec?.containers ?? []} emptyText="暂无容器" />
                  ) : null}
                  {tab.id === "events" ? <EventsTable events={events} error={eventsError} loading={eventsLoading} /> : null}
                  {tab.id === "owners" ? <OwnersTable owners={data.metadata?.ownerReferences ?? []} /> : null}
                  {tab.id === "init-containers" ? (
                    <ContainersTable containers={data.spec?.initContainers ?? []} emptyText="暂无初始化容器" />
                  ) : null}
                  {tab.id === "volumes" ? <VolumesTable volumes={data.spec?.volumes ?? []} /> : null}
                </div>
              );
            })}
          </div>
        </div>
      </section>
    </div>
  );
}

function InfoRow({ left, right }: { left: ReactNode; right?: ReactNode }) {
  return (
    <div className="k8s-pod-detail-row">
      <div>{left}</div>
      {right !== undefined ? <div>{right}</div> : null}
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="k8s-pod-detail-field">
      <span className="k8s-pod-detail-label">{label}</span>
      <span className="k8s-pod-detail-sep">:</span>
      <span className="k8s-pod-detail-value">{children}</span>
    </div>
  );
}

function MetaPills({ map }: { map?: Record<string, string> }) {
  const entries = Object.entries(map ?? {});
  if (entries.length === 0) return <span className="text-muted">-</span>;
  return (
    <div className="k8s-pod-detail-pills">
      {entries.map(([key, value]) => (
        <span key={key} className="k8s-pod-detail-pill" title={`${key}: ${value}`}>
          {key}: {value || '""'}
        </span>
      ))}
    </div>
  );
}

function ConditionsTable({ conditions }: { conditions: PodCondition[] }) {
  return (
    <div className="table-wrap">
      <table className="data-table text-sm">
        <thead>
          <tr>
            <th>类型</th>
            <th>状态</th>
            <th>更新时间</th>
            <th>内容</th>
            <th>消息</th>
          </tr>
        </thead>
        <tbody>
          {conditions.map((item, index) => (
            <tr key={`${item.type ?? "condition"}-${index}`}>
              <td>{dash(item.type)}</td>
              <td>{dash(item.status)}</td>
              <td>{formatDateTime(item.lastTransitionTime)}</td>
              <td>{dash(item.reason)}</td>
              <td className="k8s-pod-detail-break">{dash(item.message)}</td>
            </tr>
          ))}
          {conditions.length === 0 ? <EmptyRow colSpan={5} text="暂无状态" /> : null}
        </tbody>
      </table>
    </div>
  );
}

function ContainersTable({ containers, emptyText }: { containers: ContainerSpec[]; emptyText: string }) {
  return (
    <div className="table-wrap">
      <table className="data-table text-sm">
        <thead>
          <tr>
            <th>名称</th>
            <th>镜像</th>
            <th>端口</th>
          </tr>
        </thead>
        <tbody>
          {containers.map((container, index) => (
            <tr key={`${container.name ?? "container"}-${index}`}>
              <td colSpan={3} className="k8s-pod-detail-expand-cell">
                <details className="k8s-pod-detail-container">
                  <summary className="k8s-pod-detail-container-summary">
                    <span>{dash(container.name)}</span>
                    <span className="k8s-pod-detail-image">{dash(container.image)}</span>
                    <span>{formatContainerPorts(container as Record<string, unknown>)}</span>
                  </summary>
                  <ContainerExpanded container={container} />
                </details>
              </td>
            </tr>
          ))}
          {containers.length === 0 ? <EmptyRow colSpan={3} text={emptyText} /> : null}
        </tbody>
      </table>
    </div>
  );
}

function ContainerExpanded({ container }: { container: ContainerSpec }) {
  const env = formatEnvironment(container as Record<string, unknown>);
  return (
    <div className="k8s-pod-detail-container-body">
      <DetailLine label="镜像拉取策略">{dash(container.imagePullPolicy)}</DetailLine>
      <DetailLine label="环境变量">
        {env.length > 0 ? (
          <div className="k8s-pod-detail-pills">
            {env.map((line) => (
              <span key={line} className="k8s-pod-detail-pill">
                {line}
              </span>
            ))}
          </div>
        ) : (
          "-"
        )}
      </DetailLine>
      <DetailLine label="命令参数">{formatContainerCommand(container as Record<string, unknown>)}</DetailLine>
      <DetailLine label="所需资源">{formatResourcePair(container.resources?.requests)}</DetailLine>
      <DetailLine label="资源限制">{formatResourcePair(container.resources?.limits)}</DetailLine>
      <DetailLine label="挂载点">
        {container.volumeMounts && container.volumeMounts.length > 0 ? (
          <div className="k8s-pod-detail-pills">
            {container.volumeMounts.map((mount, index) => (
              <span key={`${mount.name ?? "mount"}-${mount.mountPath ?? index}`} className="k8s-pod-detail-pill">
                {formatMount(mount)}
              </span>
            ))}
          </div>
        ) : (
          "-"
        )}
      </DetailLine>
    </div>
  );
}

function EventsTable({ events, error, loading }: { events: PodEvent[]; error?: ApiErrorInfo | null; loading: boolean }) {
  const sorted = sortPodEvents(events);
  return (
    <div className="stack-sm">
      <ApiErrorBanner error={error ?? null} />
      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>类型</th>
              <th>对象</th>
              <th>信息</th>
              <th>内容</th>
              <th>时间</th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((event, index) => {
              const item = event as Record<string, unknown>;
              return (
                <tr key={eventKey(item, index)}>
                  <td>{dash(item.type)}</td>
                  <td>{eventObjectView(item)}</td>
                  <td className="k8s-pod-detail-break">{dash(item.message)}</td>
                  <td>{dash(item.reason)}</td>
                  <td>{formatDateTime(eventTimestamp(event))}</td>
                </tr>
              );
            })}
            {sorted.length === 0 ? <EmptyRow colSpan={5} text={loading ? "事件加载中…" : "暂无事件"} /> : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function OwnersTable({ owners }: { owners: OwnerReference[] }) {
  return (
    <div className="table-wrap">
      <table className="data-table text-sm">
        <thead>
          <tr>
            <th>名称</th>
            <th>类型</th>
          </tr>
        </thead>
        <tbody>
          {owners.map((owner, index) => (
            <tr key={`${owner.kind ?? "owner"}-${owner.name ?? index}`}>
              <td>{dash(owner.name)}</td>
              <td>{dash(owner.kind)}</td>
            </tr>
          ))}
          {owners.length === 0 ? <EmptyRow colSpan={2} text="暂无创建者" /> : null}
        </tbody>
      </table>
    </div>
  );
}

function VolumesTable({ volumes }: { volumes: VolumeSpec[] }) {
  return (
    <div className="table-wrap">
      <table className="data-table text-sm">
        <thead>
          <tr>
            <th>名称</th>
            <th>类型</th>
            <th>详情</th>
          </tr>
        </thead>
        <tbody>
          {volumes.map((volume, index) => {
            const source = volumeSourceView(volume);
            return (
              <tr key={`${volume.name ?? "volume"}-${index}`}>
                <td>{dash(volume.name)}</td>
                <td>{source.type}</td>
                <td className="k8s-pod-detail-break">{source.detail}</td>
              </tr>
            );
          })}
          {volumes.length === 0 ? <EmptyRow colSpan={3} text="暂无存储" /> : null}
        </tbody>
      </table>
    </div>
  );
}

function DetailLine({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="k8s-pod-detail-expand-line">
      <span className="k8s-pod-detail-expand-label">{label}:</span>
      <span className="k8s-pod-detail-expand-value">{children}</span>
    </div>
  );
}

function EmptyRow({ colSpan, text }: { colSpan: number; text: string }) {
  const loading = text.includes("加载");
  return (
    <tr>
      <td colSpan={colSpan}>
        {loading ? <K8sLoadingState label={text} compact /> : <K8sEmptyState variant="empty" title={text} compact />}
      </td>
    </tr>
  );
}

function dash(value: unknown): string {
  if (value === undefined || value === null || value === "") return "-";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return JSON.stringify(value);
}

function formatResourcePair(resources?: QuantityMap): string {
  if (!resources || Object.keys(resources).length === 0) return "-";
  const cpu = resources.cpu ?? "-";
  const memory = resources.memory ?? "-";
  return `CPU: ${cpu} / Memory: ${memory}`;
}

function formatMount(mount: VolumeMount): string {
  const parts = [dash(mount.name), mount.mountPath ? `-> ${mount.mountPath}` : ""].filter(Boolean);
  if (mount.subPath) parts.push(`subPath: ${mount.subPath}`);
  if (mount.readOnly) parts.push("readOnly");
  return parts.join(" ");
}

function eventObjectView(event: Record<string, unknown>): string {
  const involved = event.involvedObject;
  if (!involved || typeof involved !== "object" || Array.isArray(involved)) return "-";
  const record = involved as Record<string, unknown>;
  const kind = dash(record.kind);
  const name = dash(record.name);
  if (kind === "-" && name === "-") return "-";
  return `${kind}:${name}`;
}

function eventKey(event: Record<string, unknown>, index: number): string {
  const metadata = event.metadata;
  if (metadata && typeof metadata === "object" && !Array.isArray(metadata)) {
    const uid = (metadata as Record<string, unknown>).uid;
    if (typeof uid === "string" && uid) return uid;
  }
  return `${dash(event.reason)}-${dash(eventTimestamp(event))}-${index}`;
}

function stableId(value?: string): string {
  return (value || "pod").replace(/[^a-zA-Z0-9_-]+/g, "-");
}
