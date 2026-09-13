"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { ProjectPVCVM, StorageClassVM } from "@sparkops/contracts/cluster";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { cluster } from "@sparkops/web-sdk";
import { K8sEmptyState, K8sLoadingState, useK8sConfirm } from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import {
  ACCESS_MODES,
  STORAGE_UNITS,
  composeStorage,
  isValidMountPath,
  isValidPvcName,
  type AccessMode,
  type StorageUnit,
} from "./pvc-form";

interface ProjectStorageTabProps {
  workspaceId: string;
  project: ProjectVM;
  active: boolean;
}

function formatStorageError(err: unknown, fallback: string) {
  if (!(err instanceof BffError)) return fallback;
  if (err.status === 404 || /invalid response|not found/i.test(err.message)) {
    return "持久化存储服务未就绪（请确认后端已重启并加载最新接口）";
  }
  if (err.status === 503 || /unavailable/i.test(err.message)) {
    return "后端服务不可用，请稍后重试";
  }
  return err.message || fallback;
}

function phaseBadge(phase?: string) {
  if (phase === "Bound") return "badge-success";
  if (phase === "Pending") return "badge-neutral";
  if (!phase) return "badge-neutral";
  return "badge-warning";
}

export function ProjectStorageTab({ workspaceId, project, active }: ProjectStorageTabProps) {
  const [items, setItems] = useState<ProjectPVCVM[]>([]);
  const [classes, setClasses] = useState<StorageClassVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [bindOpen, setBindOpen] = useState(false);
  const [createName, setCreateName] = useState(project.name);
  const [amount, setAmount] = useState("1");
  const [unit, setUnit] = useState<StorageUnit>("Gi");
  const [accessMode, setAccessMode] = useState<AccessMode>("ReadWriteOnce");
  const [storageClass, setStorageClass] = useState("");
  const [mountPath, setMountPath] = useState("");
  const [bindPath, setBindPath] = useState("/data");
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const selected = useMemo(
    () => items.find((item) => item.name === selectedName) ?? items[0] ?? null,
    [items, selectedName],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [next, nextClasses] = await Promise.all([
        cluster.listPVCs(project.id, workspaceId),
        cluster.listStorageClasses(project.id, workspaceId).catch(() => [] as StorageClassVM[]),
      ]);
      setItems(next);
      setClasses(nextClasses);
      setSelectedName((current) => {
        if (current && next.some((item) => item.name === current)) return current;
        return next[0]?.name ?? null;
      });
    } catch (err) {
      setError(formatStorageError(err, "持久化存储加载失败"));
      setItems([]);
      setSelectedName(null);
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    if (!active) return;
    void load();
  }, [active, load]);

  function openCreateModal() {
    setCreateName(project.name);
    setAmount("1");
    setUnit("Gi");
    setAccessMode("ReadWriteOnce");
    setStorageClass("");
    setMountPath("");
    setFormError(null);
    setCreateOpen(true);
  }

  async function handleCreate() {
    const name = createName.trim() || project.name;
    if (!isValidPvcName(name)) {
      setFormError("名称需为小写字母、数字和短横线，例如 test-repo-data");
      return;
    }
    const parsed = composeStorage(amount, unit);
    if (parsed.error) {
      setFormError(parsed.error);
      return;
    }
    const path = mountPath.trim();
    if (path && !isValidMountPath(path)) {
      setFormError("挂载路径需为绝对路径，例如 /data");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      await cluster.createPVC(project.id, workspaceId, {
        name,
        storage: parsed.storage,
        access_mode: accessMode,
        storage_class: storageClass,
        mount_path: path,
      });
      setCreateOpen(false);
      await load();
      setSelectedName(name);
    } catch (err) {
      setFormError(formatStorageError(err, "创建 PVC 失败"));
    } finally {
      setSaving(false);
    }
  }

  async function handleBind() {
    if (!selected) return;
    const path = bindPath.trim();
    if (!isValidMountPath(path)) {
      setFormError("挂载路径需为绝对路径，例如 /data");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      await cluster.bindPVC(project.id, workspaceId, selected.name, { mount_path: path });
      setBindOpen(false);
      await load();
    } catch (err) {
      setFormError(formatStorageError(err, "挂载失败"));
    } finally {
      setSaving(false);
    }
  }

  async function handleUnbind() {
    if (!selected) return;
    const ok = await ask({
      title: "卸载 PVC",
      description: `确定从工作负载卸载「${selected.name}」？PVC 本身会保留。`,
      confirmLabel: "卸载",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await cluster.unbindPVC(project.id, workspaceId, selected.name);
      await load();
    } catch (err) {
      setError(formatStorageError(err, "卸载失败"));
    }
  }

  async function handleDelete() {
    if (!selected) return;
    const ok = await ask({
      title: "删除 PVC",
      description: `确定删除 PVC「${selected.name}」？若已挂载会先卸载并滚动重启工作负载。`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await cluster.deletePVC(project.id, workspaceId, selected.name);
      await load();
    } catch (err) {
      setError(formatStorageError(err, "删除失败"));
    }
  }

  const showEmpty = !loading && !error && items.length === 0;

  return (
    <div className="project-configmap-page">
      <header className="project-configmap-toolbar">
        <div>
          <h4 className="project-configmap-title">持久化存储</h4>
          <p className="project-configmap-subtitle">
            管理当前应用命名空间内的 PVC，可创建存储声明并挂载到工作负载。
          </p>
        </div>
        <div className="project-configmap-toolbar-actions">
          <button type="button" className="btn btn-secondary text-sm" onClick={() => void load()} disabled={loading}>
            刷新
          </button>
          <button
            type="button"
            className="btn btn-primary text-sm"
            onClick={openCreateModal}
            disabled={Boolean(error) && items.length === 0}
            title={error && items.length === 0 ? error : undefined}
          >
            + 添加
          </button>
        </div>
      </header>

      {loading ? <K8sLoadingState label="持久化存储加载中…" /> : null}

      {error ? (
        <div className="project-configmap-alert">
          <p>{error}</p>
          <button type="button" className="btn btn-secondary text-sm" onClick={() => void load()}>
            重试
          </button>
        </div>
      ) : null}

      {showEmpty ? (
        <K8sEmptyState
          variant="empty"
          title="还没有持久化存储"
          description="点击右上角「添加」，声明一块 PVC。填写挂载路径后会绑定到当前工作负载并滚动重启。"
        />
      ) : null}

      {!error && items.length > 1 ? (
        <div className="project-configmap-names" role="tablist" aria-label="PVC 列表">
          {items.map((item) => (
            <button
              key={item.name}
              type="button"
              role="tab"
              aria-selected={item.name === selected?.name}
              className={`project-configmap-name-chip${item.name === selected?.name ? " is-active" : ""}`}
              onClick={() => setSelectedName(item.name)}
            >
              {item.name}
              <span>{item.storage || item.status || "PVC"}</span>
            </button>
          ))}
        </div>
      ) : null}

      {!error && selected ? (
        <section className="project-configmap-card">
          <div className="project-configmap-meta">
            <div className="project-configmap-meta-item">
              <span>名称</span>
              <strong>{selected.name}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>状态</span>
              <strong>
                <span className={`badge ${phaseBadge(selected.status)}`}>{selected.status || "-"}</span>
              </strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>容量</span>
              <strong>{selected.storage || "-"}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>访问模式</span>
              <strong>{selected.access_modes?.join(", ") || "-"}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>存储类</span>
              <strong>{selected.storage_class || "集群默认"}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>挂载路径</span>
              <strong>{selected.mount_path || "未挂载"}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>PV</span>
              <strong>{selected.volume_name || "-"}</strong>
            </div>
            <div className="project-configmap-meta-item">
              <span>创建时间</span>
              <strong>{selected.created_at || "-"}</strong>
            </div>
          </div>

          <div className="project-configmap-data-head">
            <div>
              <h5>工作负载绑定</h5>
              <p>
                {selected.mounted
                  ? `已挂载为卷 ${selected.mount_name || selected.name}，路径 ${selected.mount_path}。`
                  : "尚未挂载到当前 Deployment。挂载后会滚动重启 Pod。"}
              </p>
            </div>
            <div className="project-configmap-data-actions">
              {selected.mounted ? (
                <button type="button" className="btn btn-secondary text-sm" onClick={() => void handleUnbind()}>
                  卸载
                </button>
              ) : (
                <button
                  type="button"
                  className="btn btn-primary text-sm"
                  onClick={() => {
                    setBindPath(selected.mount_path || "/data");
                    setFormError(null);
                    setBindOpen(true);
                  }}
                >
                  挂载到工作负载
                </button>
              )}
              <button type="button" className="btn btn-secondary text-sm" onClick={() => void handleDelete()}>
                删除 PVC
              </button>
            </div>
          </div>
        </section>
      ) : null}

      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="添加持久化存储"
        description={`将创建到命名空间 ${project.namespace || workspaceId}，并以 app=${project.name} 标记归属。`}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn btn-secondary" onClick={() => setCreateOpen(false)} disabled={saving}>
              取消
            </button>
            <button type="button" className="btn btn-primary" onClick={() => void handleCreate()} disabled={saving}>
              {saving ? "创建中…" : "创建"}
            </button>
          </div>
        }
      >
        <div className="stack-md">
          {formError ? <p className="text-sm text-[var(--danger)]">{formError}</p> : null}
          <label className="stack-sm">
            <span>名称</span>
            <input
              className="input"
              value={createName}
              onChange={(event) => setCreateName(event.target.value)}
              placeholder={project.name}
            />
          </label>
          <div className="project-storage-form-row">
            <label className="stack-sm">
              <span>容量</span>
              <div className="project-storage-quantity">
                <input className="input" value={amount} onChange={(event) => setAmount(event.target.value)} />
                <select className="input" value={unit} onChange={(event) => setUnit(event.target.value as StorageUnit)}>
                  {STORAGE_UNITS.map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </div>
            </label>
            <label className="stack-sm">
              <span>访问模式</span>
              <select
                className="input"
                value={accessMode}
                onChange={(event) => setAccessMode(event.target.value as AccessMode)}
              >
                {ACCESS_MODES.map((item) => (
                  <option key={item} value={item}>
                    {item}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <label className="stack-sm">
            <span>存储类</span>
            <select className="input" value={storageClass} onChange={(event) => setStorageClass(event.target.value)}>
              <option value="">集群默认</option>
              {classes.map((item) => (
                <option key={item.name} value={item.name}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          <label className="stack-sm">
            <span>挂载路径（可选）</span>
            <input
              className="input"
              value={mountPath}
              onChange={(event) => setMountPath(event.target.value)}
              placeholder="例如 /data，留空则只创建 PVC"
            />
          </label>
        </div>
      </Modal>

      <Modal
        open={bindOpen}
        onClose={() => setBindOpen(false)}
        title="挂载到工作负载"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn btn-secondary" onClick={() => setBindOpen(false)} disabled={saving}>
              取消
            </button>
            <button type="button" className="btn btn-primary" onClick={() => void handleBind()} disabled={saving}>
              {saving ? "挂载中…" : "确定"}
            </button>
          </div>
        }
      >
        <div className="stack-sm">
          {formError ? <p className="text-sm text-[var(--danger)]">{formError}</p> : null}
          <label className="stack-sm">
            <span>挂载路径</span>
            <input className="input" value={bindPath} onChange={(event) => setBindPath(event.target.value)} />
          </label>
          <p className="text-sm text-[var(--muted)]">挂载会写入 Deployment volumes/volumeMounts，并滚动重启 Pod。</p>
        </div>
      </Modal>
      {confirmDialog}
    </div>
  );
}
