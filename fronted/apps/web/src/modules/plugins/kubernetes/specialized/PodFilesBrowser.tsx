"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { K8sEmptyState, K8sLoadingState, useK8sConfirm } from "@/components/k8s";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, KubernetesApiError, parseApiError, type ApiErrorInfo } from "../api";
import { podOperationPath } from "../operations";
import type { PodObject } from "../node-view";
import {
  joinPodFilePath,
  normalizePodFilePath,
  parseLsOutput,
  podFileBreadcrumbs,
  podFilePathFromBreadcrumbs,
  type PodFileEntry,
} from "../pod-files-view";

type PodFilesBrowserProps = {
  clusterId: string;
  pod: PodObject;
  container: string;
  canManage: boolean;
};

export function PodFilesBrowser({ clusterId, pod, container, canManage }: PodFilesBrowserProps) {
  const { ask, dialog: confirmDialog } = useK8sConfirm();
  const [currentPath, setCurrentPath] = useState("/");
  const [pathDraft, setPathDraft] = useState("/");
  const [files, setFiles] = useState<PodFileEntry[]>([]);
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const active = useRef<AbortController | null>(null);
  const namespace = pod.metadata?.namespace ?? "";
  const podName = pod.metadata?.name ?? "";
  const base = namespace && podName ? podOperationPath(clusterId, namespace, podName, "") : "";

  const breadcrumbs = podFileBreadcrumbs(currentPath);

  const listFiles = useCallback(async (path: string) => {
    if (!base) return;
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({ container, path });
      const data = await k8sFetch<{ output: string }>(`${base}files?${params.toString()}`, { signal: controller.signal });
      if (active.current !== controller) return;
      setFiles(parseLsOutput(data.output ?? ""));
    } catch (err) {
      if (!controller.signal.aborted && active.current === controller) {
        setError(errorInfo(err, "读取文件列表失败"));
        setFiles([]);
      }
    } finally {
      if (active.current === controller) {
        active.current = null;
        setLoading(false);
      }
    }
  }, [base, container]);

  useEffect(() => {
    setPathDraft(currentPath);
    void listFiles(currentPath);
    return () => active.current?.abort();
  }, [currentPath, listFiles]);

  function query(path: string) {
    return new URLSearchParams({ container, path }).toString();
  }

  async function upload(file?: File) {
    if (!file || !base || !canManage) return;
    setError(null);
    const form = new FormData();
    form.append("file", file);
    try {
      const res = await fetch(`${base}files?${query(currentPath)}`, { method: "POST", body: form });
      if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
      await listFiles(currentPath);
    } catch (err) {
      setError(errorInfo(err, "上传文件失败"));
    }
  }

  async function download(entry: PodFileEntry) {
    if (!base || entry.isDir || entry.isSymlink) return;
    const targetPath = joinPodFilePath(currentPath, entry.name);
    setError(null);
    try {
      const res = await fetch(`${base}files/download?${query(targetPath)}`, { cache: "no-store" });
      if (!res.ok) throw new KubernetesApiError(await parseApiError(res));
      const blob = await res.blob();
      const href = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = href;
      anchor.download = entry.name;
      anchor.click();
      URL.revokeObjectURL(href);
    } catch (err) {
      setError(errorInfo(err, "下载文件失败"));
    }
  }

  async function remove(entry: PodFileEntry) {
    if (!base || !canManage) return;
    const targetPath = joinPodFilePath(currentPath, entry.name);
    const kind = entry.isDir ? "文件夹" : "文件";
    const ok = await ask({
      title: "删除文件",
      description: `确定删除 ${entry.name} ${kind}？该操作不可恢复。`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await k8sFetch(`${base}files?${query(targetPath)}`, { method: "DELETE" });
      await listFiles(currentPath);
    } catch (err) {
      setError(errorInfo(err, "删除失败"));
    }
  }

  function openDirectory(name: string) {
    setCurrentPath((path) => joinPodFilePath(path, name));
  }

  function openBreadcrumb(index: number) {
    if (index < 0) {
      setCurrentPath("/");
      return;
    }
    setCurrentPath(podFilePathFromBreadcrumbs(breadcrumbs.slice(0, index + 1)));
  }

  function goToPathDraft() {
    setCurrentPath(normalizePodFilePath(pathDraft));
  }

  const pathDirty = normalizePodFilePath(pathDraft) !== currentPath;

  return (
    <div className="k8s-pod-files">
      <div className="k8s-pod-files-path-row">
        <label className="k8s-pod-files-path-field">
          <span className="label">当前目录</span>
          <input
            className="input font-mono text-sm k8s-pod-files-path-input"
            value={pathDraft}
            aria-label="容器内路径"
            placeholder="/"
            onChange={(event) => setPathDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") goToPathDraft();
            }}
          />
        </label>
        <button type="button" className="btn-secondary text-sm" disabled={!pathDirty} onClick={goToPathDraft}>
          前往
        </button>
        {canManage ? (
          <label className="btn-secondary text-sm k8s-pod-files-upload" title={`上传到 ${currentPath}`}>
            上传
            <input className="hidden" type="file" onChange={(event) => void upload(event.target.files?.[0])} />
          </label>
        ) : null}
      </div>
      <nav className="k8s-pod-files-breadcrumb" aria-label="路径快捷导航">
        <button type="button" className="btn-ghost text-sm" onClick={() => openBreadcrumb(-1)}>
          根目录
        </button>
        {breadcrumbs.map((segment, index) => (
          <span key={`${segment}-${index}`} className="k8s-pod-files-crumb">
            <span className="text-muted">/</span>
            {index === breadcrumbs.length - 1 ? (
              <span className="text-sm text-muted">{segment}</span>
            ) : (
              <button type="button" className="btn-ghost text-sm" onClick={() => openBreadcrumb(index)}>
                {segment}
              </button>
            )}
          </span>
        ))}
        {canManage ? <span className="k8s-pod-files-upload-hint text-xs text-muted">上传至当前目录</span> : null}
      </nav>

      <ApiErrorBanner error={error} />

      <div className="table-wrap k8s-pod-files-table-wrap">
        <table className="data-table text-sm k8s-pod-files-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>大小</th>
              <th>权限</th>
              <th>修改时间</th>
              <th className="w-28">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5}>
                  <K8sLoadingState compact />
                </td>
              </tr>
            ) : files.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <K8sEmptyState variant="empty" title="暂无文件" compact />
                </td>
              </tr>
            ) : (
              files.map((entry) => (
                <tr key={`${entry.mode}-${entry.name}-${entry.link}`}>
                  <td>
                    {entry.isDir ? (
                      <button type="button" className="k8s-cluster-name-link" onClick={() => openDirectory(entry.name)}>
                        <span className="k8s-file-kind">目录</span>
                        {entry.name}
                      </button>
                    ) : (
                      <span className="k8s-pod-files-name" title={entry.link ? `${entry.name} -> ${entry.link}` : entry.name}>
                        <span className="k8s-file-kind">{entry.isSymlink ? "链接" : "文件"}</span>
                        {entry.name}
                      </span>
                    )}
                  </td>
                  <td className="text-muted text-xs tabular-nums">{entry.size}</td>
                  <td className="font-mono text-xs text-muted">{entry.mode}</td>
                  <td className="text-muted text-xs whitespace-nowrap">{entry.modTime}</td>
                  <td>
                    <div className="k8s-row-actions">
                      <button
                        type="button"
                        className="btn-ghost text-sm"
                        disabled={entry.isDir || entry.isSymlink}
                        onClick={() => void download(entry)}
                      >
                        下载
                      </button>
                      {canManage ? (
                        <button type="button" className="btn-ghost text-sm text-danger" onClick={() => void remove(entry)}>
                          删除
                        </button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      {confirmDialog}
    </div>
  );
}
