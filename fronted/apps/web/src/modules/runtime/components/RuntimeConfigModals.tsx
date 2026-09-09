"use client";

import { useEffect, useMemo, useState } from "react";
import type {
  DeploymentOverviewVM,
  HealthProbeOverviewVM,
  HealthProbeInput,
  LogCollectionInput,
  LogCollectionOverviewVM,
  PodVM,
  RuntimeServicePortInput,
  ServiceOverviewVM,
} from "@sparkops/contracts/cluster";
import { BffError } from "@sparkops/contracts";
import { cluster } from "@sparkops/web-sdk";
import { Modal } from "@/components/ui/Modal";
import {
  buildDeploymentYaml,
  buildPodYaml,
  buildServiceYaml,
} from "@/modules/runtime/runtime-yaml";

const DEFAULT_LOG_PATTERN = "^20[0-9]{2}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}";

interface RuntimeLogCollectionModalProps {
  open: boolean;
  projectId: string;
  workspaceId: string;
  initial?: LogCollectionOverviewVM | null;
  onClose: () => void;
  onSaved: () => void;
}

export function RuntimeLogCollectionModal({
  open,
  projectId,
  workspaceId,
  initial,
  onClose,
  onSaved,
}: RuntimeLogCollectionModalProps) {
  const [path, setPath] = useState("/var/log/");
  const [pattern, setPattern] = useState(DEFAULT_LOG_PATTERN);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setPath(initial?.path || "/var/log/");
    setPattern(initial?.pattern || DEFAULT_LOG_PATTERN);
    setError(null);
  }, [open, initial]);

  async function handleSubmit() {
    setSubmitting(true);
    setError(null);
    const body: LogCollectionInput = { path, pattern };
    try {
      await cluster.putLogCollection(projectId, workspaceId, body);
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存日志采集配置失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={initial?.configured ? "编辑日志采集器" : "添加日志采集器"}
      panelClassName="runtime-log-collection-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn btn-secondary" onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button type="button" className="btn btn-primary" onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "保存中…" : "确定"}
          </button>
        </div>
      }
    >
      {error ? <p className="runtime-config-modal-error">{error}</p> : null}
      <div className="runtime-log-collection-form">
        <div className="runtime-log-collection-row">
          <label className="runtime-log-collection-label" htmlFor="runtime-log-path">
            文件路径
          </label>
          <div className="runtime-log-collection-control">
            <div className="runtime-log-collection-path">
              <input
                id="runtime-log-path"
                className="input"
                value={path}
                onChange={(e) => setPath(e.target.value)}
                placeholder="/var/log/"
              />
              <span className="runtime-log-collection-suffix">*.log</span>
            </div>
            <p className="runtime-log-collection-help">容器内日志目录，采集该目录下所有 .log 文件</p>
          </div>
        </div>
        <div className="runtime-log-collection-row">
          <label className="runtime-log-collection-label" htmlFor="runtime-log-pattern">
            日志规则（正则）
          </label>
          <div className="runtime-log-collection-control">
            <textarea
              id="runtime-log-pattern"
              className="input runtime-log-collection-pattern"
              rows={3}
              value={pattern}
              onChange={(e) => setPattern(e.target.value)}
              placeholder={DEFAULT_LOG_PATTERN}
              spellCheck={false}
            />
            <p className="runtime-log-collection-help">用于识别单行日志起始位置的正则表达式</p>
          </div>
        </div>
      </div>
    </Modal>
  );
}

interface RuntimeHealthProbeModalProps {
  open: boolean;
  projectId: string;
  workspaceId: string;
  initial?: HealthProbeOverviewVM | null;
  onClose: () => void;
  onSaved: () => void;
}

export function RuntimeHealthProbeModal({
  open,
  projectId,
  workspaceId,
  initial,
  onClose,
  onSaved,
}: RuntimeHealthProbeModalProps) {
  const [port, setPort] = useState(8080);
  const [path, setPath] = useState("");
  const [readiness, setReadiness] = useState(true);
  const [liveness, setLiveness] = useState(false);
  const [initialDelay, setInitialDelay] = useState(10);
  const [period, setPeriod] = useState(10);
  const [timeout, setTimeoutValue] = useState(1);
  const [successThreshold, setSuccessThreshold] = useState(1);
  const [failureThreshold, setFailureThreshold] = useState(3);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setPort(initial?.port || 8080);
    setPath(initial?.path || "");
    setReadiness(initial?.readiness ?? true);
    setLiveness(initial?.liveness ?? false);
    setInitialDelay(initial?.initial_delay_seconds || 10);
    setPeriod(initial?.period_seconds || 10);
    setTimeoutValue(initial?.timeout_seconds || 1);
    setSuccessThreshold(initial?.success_threshold || 1);
    setFailureThreshold(initial?.failure_threshold || 3);
    setError(null);
  }, [open, initial]);

  async function handleSubmit() {
    setSubmitting(true);
    setError(null);
    const probe: Array<"Readiness" | "Liveness"> = [];
    if (readiness) probe.push("Readiness");
    if (liveness) probe.push("Liveness");
    if (probe.length === 0) {
      setError("请至少选择一种探针类型");
      setSubmitting(false);
      return;
    }
    const body: HealthProbeInput = {
      port,
      probe,
      path: path.trim() || undefined,
      initial_delay_seconds: initialDelay,
      period_seconds: period,
      timeout_seconds: timeout,
      success_threshold: successThreshold,
      failure_threshold: failureThreshold,
    };
    try {
      await cluster.putHealthProbe(projectId, workspaceId, body);
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存健康检测配置失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={initial?.configured ? "编辑健康检测" : "添加健康检测"}
      panelClassName="runtime-health-probe-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn btn-secondary" onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button type="button" className="btn btn-primary" onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "保存中…" : "确定"}
          </button>
        </div>
      }
    >
      {error ? <p className="runtime-config-modal-error">{error}</p> : null}
      <div className="runtime-health-probe-form">
        <div className="runtime-health-probe-row">
          <label className="runtime-health-probe-label" htmlFor="runtime-health-port">
            检测端口
          </label>
          <div className="runtime-health-probe-control">
            <input
              id="runtime-health-port"
              className="input runtime-health-probe-number"
              type="number"
              min={1}
              max={65535}
              value={port}
              onChange={(e) => setPort(Number(e.target.value))}
            />
            <p className="runtime-health-probe-help">容器对外提供健康检查的端口，范围 1 ~ 65535</p>
          </div>
        </div>

        <div className="runtime-health-probe-row">
          <span className="runtime-health-probe-label">探针类型</span>
          <div className="runtime-health-probe-control">
            <div className="runtime-health-probe-checkboxes">
              <label className="runtime-health-probe-checkbox">
                <input type="checkbox" checked={readiness} onChange={(e) => setReadiness(e.target.checked)} />
                Readiness
              </label>
              <label className="runtime-health-probe-checkbox">
                <input type="checkbox" checked={liveness} onChange={(e) => setLiveness(e.target.checked)} />
                Liveness
              </label>
            </div>
            <p className="runtime-health-probe-help">至少选择一种探针类型</p>
          </div>
        </div>

        <div className="runtime-health-probe-row">
          <label className="runtime-health-probe-label" htmlFor="runtime-health-path">
            HTTP 路径
          </label>
          <div className="runtime-health-probe-control">
            <input
              id="runtime-health-path"
              className="input"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="/health"
            />
            <p className="runtime-health-probe-help">留空则使用 TCP 探针</p>
          </div>
        </div>

        <div className="runtime-health-probe-grid">
          <div className="runtime-health-probe-pair">
            <label className="runtime-health-probe-pair-label" htmlFor="runtime-health-initial-delay">
              初始化等候时间（秒）
            </label>
            <input
              id="runtime-health-initial-delay"
              className="input runtime-health-probe-number"
              type="number"
              min={1}
              value={initialDelay}
              onChange={(e) => setInitialDelay(Number(e.target.value))}
            />
          </div>
          <div className="runtime-health-probe-pair">
            <label className="runtime-health-probe-pair-label" htmlFor="runtime-health-period">
              检测间隔（秒）
            </label>
            <input
              id="runtime-health-period"
              className="input runtime-health-probe-number"
              type="number"
              min={1}
              value={period}
              onChange={(e) => setPeriod(Number(e.target.value))}
            />
          </div>
          <div className="runtime-health-probe-pair">
            <label className="runtime-health-probe-pair-label" htmlFor="runtime-health-timeout">
              检测超时（秒）
            </label>
            <input
              id="runtime-health-timeout"
              className="input runtime-health-probe-number"
              type="number"
              min={1}
              value={timeout}
              onChange={(e) => setTimeoutValue(Number(e.target.value))}
            />
          </div>
          <div className="runtime-health-probe-pair">
            <label className="runtime-health-probe-pair-label" htmlFor="runtime-health-success">
              连续成功次数
            </label>
            <input
              id="runtime-health-success"
              className="input runtime-health-probe-number"
              type="number"
              min={1}
              value={successThreshold}
              onChange={(e) => setSuccessThreshold(Number(e.target.value))}
            />
          </div>
          <div className="runtime-health-probe-pair">
            <label className="runtime-health-probe-pair-label" htmlFor="runtime-health-failure">
              连续失败次数
            </label>
            <input
              id="runtime-health-failure"
              className="input runtime-health-probe-number"
              type="number"
              min={1}
              value={failureThreshold}
              onChange={(e) => setFailureThreshold(Number(e.target.value))}
            />
          </div>
        </div>
      </div>
    </Modal>
  );
}

interface ServicePortRow extends RuntimeServicePortInput {
  key: string;
}

const PORT_NAME_PATTERN = /^[a-zA-Z][-_a-zA-Z0-9]*$/;

function portsFromService(service: ServiceOverviewVM | undefined): ServicePortRow[] {
  if (!service?.ports?.length) {
    return [{ key: "0", port: 8080, name: "http-8080", protocol: "TCP" }];
  }
  return service.ports.map((item, index) => ({
    key: String(index),
    port: item.port,
    name: item.name || `http-${item.port}`,
    protocol: item.protocol || "TCP",
  }));
}

function validatePortRows(rows: ServicePortRow[]): string | null {
  for (const row of rows) {
    if (!row.port || row.port < 1 || row.port > 65535) {
      return "端口必须在 0 ~ 65535 之间";
    }
    if (!row.name.trim()) {
      return "端口名称不能为空";
    }
    if (!PORT_NAME_PATTERN.test(row.name.trim())) {
      return "端口名称必须以英文字母开头，且仅包含字母、数字、-、_";
    }
    if (!row.protocol) {
      return "协议不能为空";
    }
  }
  return null;
}

export function ServiceYamlIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" aria-hidden>
      <path
        d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinejoin="round"
      />
      <path
        d="M14 2v6h6"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinejoin="round"
      />
      <path
        d="M9 13l-1.5 1.5L9 16M15 13l1.5 1.5L15 16"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

interface RuntimeYamlViewModalProps {
  open: boolean;
  title: string;
  yaml: string;
  emptyMessage: string;
  onClose: () => void;
}

export function RuntimeYamlViewModal({
  open,
  title,
  yaml,
  emptyMessage,
  onClose,
}: RuntimeYamlViewModalProps) {
  const lines = useMemo(() => yaml.split("\n"), [yaml]);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      size="xl"
      panelClassName="runtime-service-yaml-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            关闭
          </button>
        </div>
      }
    >
      {!yaml ? (
        <p className="project-runtime-empty">{emptyMessage}</p>
      ) : (
        <div className="k8s-yaml-editor" data-theme="light">
          <div className="k8s-yaml-gutter" aria-hidden>
            {lines.map((_, index) => (
              <div key={index}>{index + 1}</div>
            ))}
          </div>
          <textarea className="k8s-yaml-textarea" value={yaml} readOnly spellCheck={false} />
        </div>
      )}
    </Modal>
  );
}

interface RuntimeServiceYamlModalProps {
  open: boolean;
  service?: ServiceOverviewVM | null;
  onClose: () => void;
}

export function RuntimeServiceYamlModal({ open, service, onClose }: RuntimeServiceYamlModalProps) {
  const yaml = useMemo(() => (service ? buildServiceYaml(service) : ""), [service]);

  return (
    <RuntimeYamlViewModal
      open={open}
      title="Service YAML"
      yaml={yaml}
      emptyMessage="暂无 Service。"
      onClose={onClose}
    />
  );
}

interface RuntimeDeploymentYamlModalProps {
  open: boolean;
  deployment?: DeploymentOverviewVM | null;
  pod?: PodVM | null;
  onClose: () => void;
}

export function RuntimeDeploymentYamlModal({
  open,
  deployment,
  pod,
  onClose,
}: RuntimeDeploymentYamlModalProps) {
  const yaml = useMemo(
    () => (deployment ? buildDeploymentYaml(deployment, pod) : ""),
    [deployment, pod],
  );

  return (
    <RuntimeYamlViewModal
      open={open}
      title="Deployment YAML"
      yaml={yaml}
      emptyMessage="暂无 Deployment。"
      onClose={onClose}
    />
  );
}

interface RuntimePodYamlModalProps {
  open: boolean;
  pod?: PodVM | null;
  onClose: () => void;
}

export function RuntimePodYamlModal({ open, pod, onClose }: RuntimePodYamlModalProps) {
  const yaml = useMemo(() => (pod ? buildPodYaml(pod) : ""), [pod]);

  return (
    <RuntimeYamlViewModal
      open={open}
      title="Pod YAML"
      yaml={yaml}
      emptyMessage="暂无 Pod。"
      onClose={onClose}
    />
  );
}

function PortCircleIcon({ kind }: { kind: "plus" | "minus" }) {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden>
      <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="1.5" />
      {kind === "plus" ? (
        <>
          <path d="M12 8v8M8 12h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </>
      ) : (
        <path d="M8 12h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      )}
    </svg>
  );
}

interface RuntimeServicePortModalProps {
  open: boolean;
  projectId: string;
  workspaceId: string;
  service?: ServiceOverviewVM | null;
  onClose: () => void;
  onSaved: () => void;
}

export function RuntimeServicePortModal({
  open,
  projectId,
  workspaceId,
  service,
  onClose,
  onSaved,
}: RuntimeServicePortModalProps) {
  const [rows, setRows] = useState<ServicePortRow[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setRows(portsFromService(service ?? undefined));
    setError(null);
  }, [open, service]);

  function updateRow(key: string, patch: Partial<ServicePortRow>) {
    setRows((prev) => prev.map((row) => (row.key === key ? { ...row, ...patch } : row)));
  }

  function addRow() {
    setRows((prev) => [
      ...prev,
      { key: String(Date.now()), port: 80, name: "", protocol: "TCP" },
    ]);
  }

  function removeRow(key: string) {
    setRows((prev) => (prev.length <= 1 ? prev : prev.filter((row) => row.key !== key)));
  }

  async function handleSubmit() {
    const validationError = validatePortRows(rows);
    if (validationError) {
      setError(validationError);
      return;
    }
    setSubmitting(true);
    setError(null);
    const ports: RuntimeServicePortInput[] = rows.map(({ port, name, protocol }) => ({
      port,
      name: name.trim(),
      protocol,
    }));
    try {
      await cluster.putServicePorts(projectId, workspaceId, { ports });
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "调整端口失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="调整【端口及协议】"
      panelClassName="runtime-service-port-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn btn-secondary" onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button type="button" className="btn btn-primary" onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "保存中…" : "确定"}
          </button>
        </div>
      }
    >
      {error ? <p className="runtime-config-modal-error">{error}</p> : null}
      <div className="runtime-service-port-form">
        {rows.map((row) => (
          <div key={row.key} className="runtime-service-port-row">
            <span className="runtime-service-port-label">端口及协议：</span>
            <div className="runtime-service-port-fields">
              <div className="runtime-service-port-field runtime-service-port-field-port">
                <input
                  type="number"
                  min={1}
                  max={65535}
                  value={row.port}
                  placeholder="端口号"
                  onChange={(e) => updateRow(row.key, { port: Number(e.target.value) })}
                />
                <span className="runtime-service-port-help">0 ~ 65535之间</span>
              </div>
              <div className="runtime-service-port-field runtime-service-port-field-name">
                <input
                  value={row.name}
                  placeholder="端口名称"
                  onChange={(e) => updateRow(row.key, { name: e.target.value })}
                />
              </div>
              <div className="runtime-service-port-field runtime-service-port-field-protocol">
                <select
                  value={row.protocol}
                  onChange={(e) => updateRow(row.key, { protocol: e.target.value })}
                >
                  <option value="TCP">TCP</option>
                  <option value="UDP">UDP</option>
                </select>
                <span className="runtime-service-port-help">协议</span>
              </div>
              <div className="runtime-service-port-actions">
                <button
                  type="button"
                  className="runtime-service-port-icon"
                  onClick={addRow}
                  aria-label="添加端口"
                >
                  <PortCircleIcon kind="plus" />
                </button>
                {rows.length > 1 ? (
                  <button
                    type="button"
                    className="runtime-service-port-icon"
                    onClick={() => removeRow(row.key)}
                    aria-label="删除端口"
                  >
                    <PortCircleIcon kind="minus" />
                  </button>
                ) : null}
              </div>
            </div>
          </div>
        ))}
      </div>
    </Modal>
  );
}

interface RuntimeScaleModalProps {
  open: boolean;
  projectId: string;
  workspaceId: string;
  currentReplicas: number;
  onClose: () => void;
  onSaved: () => void;
}

export function RuntimeScaleModal({
  open,
  projectId,
  workspaceId,
  currentReplicas,
  onClose,
  onSaved,
}: RuntimeScaleModalProps) {
  const [replicas, setReplicas] = useState(currentReplicas);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setReplicas(currentReplicas);
    setError(null);
  }, [open, currentReplicas]);

  async function handleSubmit() {
    if (!Number.isInteger(replicas) || replicas < 0 || replicas > 1000) {
      setError("副本数必须在 0 ~ 1000 之间");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await cluster.scaleDeployment(projectId, workspaceId, { replicas });
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "伸缩失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="伸缩"
      panelClassName="runtime-scale-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn btn-secondary" onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button type="button" className="btn btn-primary" onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? "保存中…" : "确定"}
          </button>
        </div>
      }
    >
      {error ? <p className="runtime-config-modal-error">{error}</p> : null}
      <div className="runtime-scale-form">
        <span className="runtime-scale-label">副本数</span>
        <div className="project-replicas-field">
          <div className="project-stepper">
            <button
              type="button"
              className="project-stepper-btn"
              disabled={submitting || replicas <= 0}
              onClick={() => setReplicas((value) => Math.max(0, value - 1))}
              aria-label="减少副本"
            >
              −
            </button>
            <input
              type="number"
              className="project-stepper-input"
              min={0}
              max={1000}
              value={replicas}
              disabled={submitting}
              onChange={(e) => setReplicas(Math.max(0, Number(e.target.value) || 0))}
            />
            <button
              type="button"
              className="project-stepper-btn"
              disabled={submitting || replicas >= 1000}
              onClick={() => setReplicas((value) => Math.min(1000, value + 1))}
              aria-label="增加副本"
            >
              +
            </button>
          </div>
          <span className="runtime-scale-help">当前 {currentReplicas} 个，范围 0 ~ 1000</span>
        </div>
      </div>
    </Modal>
  );
}
