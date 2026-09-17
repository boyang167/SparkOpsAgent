"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { K8sEmptyState, K8sLoadingState, K8sPagination, K8sResourceTabs } from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { CATALOG, proxyPath } from "@/modules/plugins/kubernetes/catalog";
import { errorInfo, k8sFetch } from "@/modules/plugins/kubernetes/api";
import { formatDateTime } from "@/modules/plugins/kubernetes/node-view";
import { resourceCollectionUrl } from "@/modules/plugins/kubernetes/resource-url";
import { stringifyYamlEditorDocument } from "@/modules/plugins/kubernetes/yaml-document";
import { clampPageToTotal } from "@/modules/plugins/kubernetes/workload-view";
import {
  ENV_RESOURCE_GROUPS,
  resourceImages,
  resourceReady,
  type EnvResourceGroupId,
  type EnvResourceKind,
} from "./environment-resource-view";

type K8sObject = {
  kind?: string;
  apiVersion?: string;
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
  };
  spec?: Record<string, unknown>;
  status?: Record<string, unknown>;
};

type K8sList = { items?: K8sObject[]; total?: number };

interface EnvironmentResourceViewProps {
  clusterId: string;
  namespace: string;
}

function readyColumnTitle(kind: EnvResourceKind) {
  if (kind === "Service") return "类型";
  if (kind === "PersistentVolumeClaim") return "状态";
  if (kind === "CronJob") return "状态";
  return "Ready";
}

function extraColumnTitle(kind: EnvResourceKind) {
  if (kind === "Service" || kind === "Ingress" || kind === "Endpoints" || kind === "NetworkPolicy") return "详情";
  if (kind === "ConfigMap" || kind === "Secret" || kind === "HorizontalPodAutoscaler" || kind === "PodDisruptionBudget") {
    return "详情";
  }
  return "镜像";
}

function extraColumnValue(item: K8sObject, kind: EnvResourceKind) {
  if (kind === "Service") {
    const spec = item.spec ?? {};
    const clusterIP = typeof spec.clusterIP === "string" ? spec.clusterIP : "";
    return clusterIP || "—";
  }
  if (kind === "Ingress") {
    const spec = item.spec ?? {};
    const rules = Array.isArray(spec.rules) ? spec.rules : [];
    const hosts = rules
      .map((rule) => (rule && typeof rule === "object" && "host" in rule ? String((rule as { host?: string }).host ?? "") : ""))
      .filter(Boolean);
    return hosts.join(", ") || "—";
  }
  if (kind === "PersistentVolumeClaim") {
    const spec = item.spec ?? {};
    const resources = spec.resources && typeof spec.resources === "object" ? (spec.resources as { requests?: { storage?: string } }) : null;
    return resources?.requests?.storage || "—";
  }
  if (
    kind === "ConfigMap" ||
    kind === "Secret" ||
    kind === "Endpoints" ||
    kind === "NetworkPolicy" ||
    kind === "HorizontalPodAutoscaler" ||
    kind === "PodDisruptionBudget"
  ) {
    return kind;
  }
  return resourceImages(item);
}

export function EnvironmentResourceView({ clusterId, namespace }: EnvironmentResourceViewProps) {
  const [groupId, setGroupId] = useState<EnvResourceGroupId>("workloads");
  const [kind, setKind] = useState<EnvResourceKind>("Deployment");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [items, setItems] = useState<K8sObject[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detail, setDetail] = useState<K8sObject | null>(null);
  const [yamlText, setYamlText] = useState<string | null>(null);
  const [busyName, setBusyName] = useState<string | null>(null);

  const group = ENV_RESOURCE_GROUPS.find((item) => item.id === groupId) ?? ENV_RESOURCE_GROUPS[0];
  const selectedDef = useMemo(() => CATALOG.find((item) => item.kind === kind), [kind]);
  const showKindTabs = group.kinds.length > 1;

  useEffect(() => {
    setGroupId("workloads");
    setKind("Deployment");
    setPage(1);
    setDetail(null);
    setYamlText(null);
  }, [clusterId, namespace]);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!clusterId || !namespace || !selectedDef) return;
      const url = resourceCollectionUrl(clusterId, selectedDef, { namespace, page, pageSize });
      if (!url) return;
      setLoading(true);
      setError(null);
      try {
        const data = await k8sFetch<K8sList>(url, { signal });
        if (signal?.aborted) return;
        const nextItems = (data.items ?? []).map((item) => ({ ...item, kind: item.kind || kind }));
        const nextTotal = data.total ?? nextItems.length;
        const nextPage = clampPageToTotal(page, nextTotal, pageSize);
        if (nextPage !== page) {
          setPage(nextPage);
          return;
        }
        setItems(nextItems);
        setTotal(nextTotal);
      } catch (err) {
        if (signal?.aborted) return;
        setItems([]);
        setTotal(0);
        setError(errorInfo(err, `加载 ${kind} 失败`).message);
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    },
    [clusterId, namespace, selectedDef, page, pageSize, kind],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  function changeGroup(next: EnvResourceGroupId) {
    const found = ENV_RESOURCE_GROUPS.find((item) => item.id === next);
    if (!found) return;
    setGroupId(next);
    setKind(found.kinds[0]);
    setPage(1);
  }

  function changeKind(next: EnvResourceKind) {
    setKind(next);
    setPage(1);
  }

  async function fetchObject(item: K8sObject) {
    const name = item.metadata?.name?.trim();
    if (!selectedDef || !name) return null;
    return k8sFetch<K8sObject>(proxyPath(clusterId, selectedDef, namespace, name));
  }

  async function onView(item: K8sObject) {
    const name = item.metadata?.name ?? "";
    setBusyName(name);
    try {
      const latest = (await fetchObject(item)) ?? item;
      setDetail({ ...latest, kind: latest.kind || kind });
    } catch (err) {
      setError(errorInfo(err, "加载资源详情失败").message);
    } finally {
      setBusyName(null);
    }
  }

  async function onYaml(item: K8sObject) {
    const name = item.metadata?.name ?? "";
    setBusyName(name);
    try {
      const latest = (await fetchObject(item)) ?? item;
      setYamlText(stringifyYamlEditorDocument(latest));
    } catch (err) {
      setError(errorInfo(err, "加载 YAML 失败").message);
    } finally {
      setBusyName(null);
    }
  }

  const colSpan = 5;

  return (
    <div className="product-env-resource">
      <div className="product-env-resource-groups" role="tablist" aria-label="资源分类">
        {ENV_RESOURCE_GROUPS.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={item.id === groupId}
            className={`product-env-resource-group${item.id === groupId ? " is-active" : ""}`}
            onClick={() => changeGroup(item.id)}
          >
            {item.label}
          </button>
        ))}
      </div>

      {showKindTabs ? (
        <K8sResourceTabs
          tabs={group.kinds}
          active={kind}
          onChange={changeKind}
          ariaLabel={`${group.label}资源类型`}
        />
      ) : null}

      {error ? <p className="product-env-resource-error">{error}</p> : null}

      {!selectedDef ? (
        <K8sEmptyState variant="missing-def" kind={kind} compact />
      ) : (
        <>
          <div className="table-wrap product-env-services-table">
            <table className="data-table text-sm">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>{readyColumnTitle(kind)}</th>
                  <th>{extraColumnTitle(kind)}</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {loading ? (
                  <tr>
                    <td colSpan={colSpan}>
                      <K8sLoadingState compact label={`加载 ${kind}…`} />
                    </td>
                  </tr>
                ) : items.length === 0 ? (
                  <tr>
                    <td colSpan={colSpan}>
                      <K8sEmptyState
                        variant="empty"
                        compact
                        title={`暂无 ${kind}`}
                        description="当前环境 Namespace 下没有该类型资源。"
                      />
                    </td>
                  </tr>
                ) : (
                  items.map((item) => {
                    const name = item.metadata?.name ?? "—";
                    return (
                      <tr key={name}>
                        <td>
                          <button type="button" className="product-env-service-name" onClick={() => void onView(item)}>
                            {name}
                          </button>
                        </td>
                        <td>{resourceReady({ ...item, kind })}</td>
                        <td className="font-mono text-xs product-env-image-cell">{extraColumnValue({ ...item, kind }, kind)}</td>
                        <td className="text-muted-foreground">
                          {formatDateTime(item.metadata?.creationTimestamp)}
                        </td>
                        <td>
                          <div className="product-env-row-actions">
                            <button
                              type="button"
                              className="link-primary text-xs"
                              disabled={busyName === name}
                              onClick={() => void onView(item)}
                            >
                              查看
                            </button>
                            <button
                              type="button"
                              className="link-primary text-xs"
                              disabled={busyName === name}
                              onClick={() => void onYaml(item)}
                            >
                              YAML
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
          <K8sPagination
            page={page}
            pageSize={pageSize}
            total={total}
            loading={loading}
            onPageChange={setPage}
            onPageSizeChange={(size) => {
              setPageSize(size);
              setPage(1);
            }}
          />
        </>
      )}

      <Modal
        open={Boolean(detail)}
        onClose={() => setDetail(null)}
        title={detail?.metadata?.name || "资源详情"}
        description={detail ? `${detail.kind || kind} · ${namespace}` : undefined}
        size="lg"
        footer={
          <button type="button" className="btn btn-secondary text-sm" onClick={() => setDetail(null)}>
            关闭
          </button>
        }
      >
        {detail ? (
          <dl className="product-env-resource-detail">
            <div>
              <dt>名称</dt>
              <dd>{detail.metadata?.name}</dd>
            </div>
            <div>
              <dt>类型</dt>
              <dd>{detail.kind || kind}</dd>
            </div>
            <div>
              <dt>{readyColumnTitle(kind)}</dt>
              <dd>{resourceReady({ ...detail, kind: detail.kind || kind })}</dd>
            </div>
            <div>
              <dt>{extraColumnTitle(kind)}</dt>
              <dd className="font-mono">{extraColumnValue({ ...detail, kind: (detail.kind as EnvResourceKind) || kind }, kind)}</dd>
            </div>
            <div>
              <dt>创建时间</dt>
              <dd>{formatDateTime(detail.metadata?.creationTimestamp)}</dd>
            </div>
          </dl>
        ) : null}
      </Modal>

      <Modal
        open={yamlText !== null}
        onClose={() => setYamlText(null)}
        title="YAML"
        size="xl"
        footer={
          <button type="button" className="btn btn-secondary text-sm" onClick={() => setYamlText(null)}>
            关闭
          </button>
        }
      >
        <pre className="product-env-yaml">{yamlText}</pre>
      </Modal>
    </div>
  );
}
