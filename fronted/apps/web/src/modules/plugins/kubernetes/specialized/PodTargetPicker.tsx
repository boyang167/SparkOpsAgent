"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { K8sLoadingState } from "@/components/k8s";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import { CATALOG, proxyPath, type ResourceDef } from "../catalog";
import { resourceCollectionUrl } from "../resource-url";
import { podContainerNames, podPhase, type PodObject } from "../node-view";

export type PodTargetValue = {
  namespace: string;
  pod: string;
  container: string;
};

const namespaceDef = CATALOG.find((item) => item.kind === "Namespace");
const podDef = CATALOG.find((item) => item.kind === "Pod");

type K8sList<T> = { items?: T[]; total?: number };

function pickNamespace(names: string[], current: string): string {
  if (current && names.includes(current)) return current;
  if (names.includes("default")) return "default";
  return names[0] ?? "";
}

function pickPod(items: PodObject[], current: string): PodObject | undefined {
  if (current) {
    const existing = items.find((item) => item.metadata?.name === current);
    if (existing) return existing;
  }
  return items.find((item) => podPhase(item) === "Running") ?? items[0];
}

export function PodTargetPicker({
  clusterId,
  value,
  onChange,
  disabled,
  onError,
}: {
  clusterId: string;
  value: PodTargetValue;
  onChange: (next: PodTargetValue) => void;
  disabled?: boolean;
  onError?: (error: ApiErrorInfo | null) => void;
}) {
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [pods, setPods] = useState<PodObject[]>([]);
  const [loadingNamespaces, setLoadingNamespaces] = useState(false);
  const [loadingPods, setLoadingPods] = useState(false);
  const nsSeq = useRef(0);
  const podSeq = useRef(0);

  const loadNamespaces = useCallback(async () => {
    if (!clusterId || !namespaceDef) return;
    const seq = ++nsSeq.current;
    setLoadingNamespaces(true);
    onError?.(null);
    try {
      const names = await listAllNames(clusterId, namespaceDef);
      if (seq !== nsSeq.current) return;
      setNamespaces(names);
    } catch (err) {
      if (seq !== nsSeq.current) return;
      setNamespaces([]);
      onError?.(errorInfo(err, "加载命名空间失败"));
    } finally {
      if (seq === nsSeq.current) setLoadingNamespaces(false);
    }
  }, [clusterId, onError]);

  const loadPods = useCallback(
    async (namespace: string) => {
      if (!clusterId || !podDef || !namespace) {
        setPods([]);
        return;
      }
      const seq = ++podSeq.current;
      setLoadingPods(true);
      onError?.(null);
      try {
        const items = await listAllPods(clusterId, podDef, namespace);
        if (seq !== podSeq.current) return;
        setPods(items);
      } catch (err) {
        if (seq !== podSeq.current) return;
        setPods([]);
        onError?.(errorInfo(err, "加载 Pod 失败"));
      } finally {
        if (seq === podSeq.current) setLoadingPods(false);
      }
    },
    [clusterId, onError],
  );

  useEffect(() => {
    void loadNamespaces();
  }, [loadNamespaces]);

  useEffect(() => {
    if (namespaces.length === 0) return;
    const nextNamespace = pickNamespace(namespaces, value.namespace);
    if (nextNamespace !== value.namespace) {
      onChange({ namespace: nextNamespace, pod: "", container: "" });
    }
  }, [namespaces, onChange, value.namespace]);

  useEffect(() => {
    void loadPods(value.namespace);
  }, [loadPods, value.namespace]);

  useEffect(() => {
    if (!value.namespace) return;
    if (pods.length === 0) {
      if (value.pod || value.container) onChange({ namespace: value.namespace, pod: "", container: "" });
      return;
    }
    const pod = pickPod(pods, value.pod);
    const podName = pod?.metadata?.name ?? "";
    const containers = pod ? podContainerNames(pod) : [];
    const container = containers.includes(value.container) ? value.container : containers[0] ?? "";
    if (podName !== value.pod || container !== value.container) {
      onChange({ namespace: value.namespace, pod: podName, container });
    }
  }, [pods, onChange, value.container, value.namespace, value.pod]);

  const containers = value.pod ? podContainerNames(pods.find((item) => item.metadata?.name === value.pod) ?? {}) : [];
  const ready = Boolean(value.namespace && value.pod && value.container);
  const busy = loadingNamespaces || loadingPods;

  function refresh() {
    void loadNamespaces();
    void loadPods(value.namespace);
  }

  return (
    <div className="k8s-pod-target-picker">
      <div className="k8s-pod-target-fields">
        <label className="k8s-pod-target-field">
          <span className="label">命名空间</span>
          <select
            className="input k8s-pod-target-select"
            value={value.namespace}
            disabled={disabled || !clusterId || loadingNamespaces}
            onChange={(event) => onChange({ namespace: event.target.value, pod: "", container: "" })}
          >
            {namespaces.length === 0 ? <option value="">{loadingNamespaces ? "加载中…" : "暂无命名空间"}</option> : null}
            {namespaces.map((item) => (
              <option key={item} value={item}>
                {item}
              </option>
            ))}
          </select>
        </label>
        <label className="k8s-pod-target-field">
          <span className="label">Pod</span>
          <select
            className="input k8s-pod-target-select"
            value={value.pod}
            disabled={disabled || !value.namespace || loadingPods || pods.length === 0}
            onChange={(event) => {
              const pod = pods.find((item) => item.metadata?.name === event.target.value);
              const nextContainers = pod ? podContainerNames(pod) : [];
              onChange({
                namespace: value.namespace,
                pod: event.target.value,
                container: nextContainers[0] ?? "",
              });
            }}
          >
            {!value.namespace ? <option value="">请先选择命名空间</option> : null}
            {value.namespace && pods.length === 0 ? (
              <option value="">{loadingPods ? "加载中…" : "暂无 Pod"}</option>
            ) : null}
            {pods.map((item) => {
              const name = item.metadata?.name ?? "";
              const phase = podPhase(item);
              return (
                <option key={name} value={name}>
                  {name}
                  {phase !== "Running" ? ` · ${phase}` : ""}
                </option>
              );
            })}
          </select>
        </label>
        <label className="k8s-pod-target-field">
          <span className="label">容器</span>
          <select
            className="input k8s-pod-target-select"
            value={value.container}
            disabled={disabled || !value.pod || containers.length === 0}
            onChange={(event) => onChange({ ...value, container: event.target.value })}
          >
            {!value.pod ? <option value="">请先选择 Pod</option> : null}
            {value.pod && containers.length === 0 ? <option value="">暂无容器</option> : null}
            {containers.map((item) => (
              <option key={item} value={item}>
                {item}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          className="btn-secondary k8s-clusters-refresh-btn"
          disabled={disabled || !clusterId || busy}
          aria-label="刷新目标列表"
          onClick={refresh}
        >
          ↻
        </button>
      </div>
      <div className="k8s-pod-target-meta">
        {busy ? <K8sLoadingState compact label="同步目标…" /> : null}
        {!busy && ready ? (
          <span className="text-xs text-muted">
            {value.namespace}/{value.pod} · {value.container}
          </span>
        ) : null}
        {!busy && !ready && clusterId ? <span className="text-xs text-muted">选择 Pod 与容器后连接</span> : null}
      </div>
    </div>
  );
}

async function listAllNames(clusterId: string, def: ResourceDef): Promise<string[]> {
  const pageSize = 500;
  let page = 1;
  let total: number | undefined;
  const all: string[] = [];
  for (;;) {
    const url = resourceCollectionUrl(clusterId, def, { page, pageSize }) ?? proxyPath(clusterId, def);
    const data = await k8sFetch<K8sList<{ metadata?: { name?: string } }>>(url);
    const items = data.items ?? [];
    for (const item of items) {
      if (item.metadata?.name) all.push(item.metadata.name);
    }
    total = data.total ?? total;
    if (items.length < pageSize || (total !== undefined && all.length >= total)) break;
    page += 1;
  }
  return all.sort((a, b) => a.localeCompare(b));
}

async function listAllPods(clusterId: string, def: ResourceDef, namespace: string): Promise<PodObject[]> {
  const pageSize = 500;
  let page = 1;
  let total: number | undefined;
  const all: PodObject[] = [];
  for (;;) {
    const url = resourceCollectionUrl(clusterId, def, { namespace, page, pageSize });
    if (!url) break;
    const data = await k8sFetch<K8sList<PodObject>>(url);
    const items = data.items ?? [];
    all.push(...items);
    total = data.total ?? total;
    if (items.length < pageSize || (total !== undefined && all.length >= total)) break;
    page += 1;
  }
  return all.sort((a, b) => (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? ""));
}
