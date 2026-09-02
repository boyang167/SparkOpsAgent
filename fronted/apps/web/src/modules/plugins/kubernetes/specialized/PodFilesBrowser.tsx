"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, KubernetesApiError, parseApiError, type ApiErrorInfo } from "../api";
import { podOperationPath } from "../operations";
import type { PodObject } from "../node-view";
import {
  joinPodFilePath,
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
  const [currentPath, setCurrentPath] = useState("/");
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
    if (!window.confirm(`您确定要删除 ${entry.name} ${kind}？该操作不可逆，是否继续？`)) return;
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

  return (
    <div className="k8s-pod-files">
      <div className="k8s-pod-files-toolbar">
        {canManage ? (
          <label className="btn-primary k8s-pod-files-upload">
            上传文件
            <input className="hidden" type="file" onChange={(event) => void upload(event.target.files?.[0])} />
          </label>
        ) : null}
      </div>

      <div className="k8s-pod-files-breadcrumb">
        <button type="button" className="k8s-action-link" onClick={() => openBreadcrumb(-1)}>
          根目录
        </button>
        {breadcrumbs.map((segment, index) => (
          <span key={`${segment}-${index}`} className="k8s-pod-files-crumb">
            <span>/</span>
            {index === breadcrumbs.length - 1 ? (
              <span>{segment}</span>
            ) : (
              <button type="button" className="k8s-action-link" onClick={() => openBreadcrumb(index)}>
                {segment}
              </button>
            )}
          </span>
        ))}
      </div>

      <ApiErrorBanner error={error} />

      <div className="k8s-table-wrap">
        <table className="k8s-table k8s-pod-files-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>大小</th>
              <th>权限</th>
              <th>所有者</th>
              <th>所属组</th>
              <th>最后修改时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7} className="text-center text-muted py-6">
                  加载中…
                </td>
              </tr>
            ) : files.length === 0 ? (
              <tr>
                <td colSpan={7} className="text-center text-muted py-6">
                  暂无文件
                </td>
              </tr>
            ) : (
              files.map((entry) => (
                <tr key={`${entry.mode}-${entry.name}-${entry.link}`}>
                  <td>
                    {entry.isDir ? (
                      <button type="button" className="k8s-action-link" onClick={() => openDirectory(entry.name)}>
                        📁 {entry.name}
                      </button>
                    ) : (
                      <span className="k8s-pod-files-name">
                        {entry.isSymlink ? "🔗" : "📄"} {entry.name}
                        {entry.link ? ` -> ${entry.link}` : ""}
                      </span>
                    )}
                  </td>
                  <td>{entry.size}</td>
                  <td className="font-mono text-xs">{entry.mode}</td>
                  <td>{entry.user}</td>
                  <td>{entry.group}</td>
                  <td>{entry.modTime}</td>
                  <td className="k8s-pod-files-actions">
                    <button
                      type="button"
                      className="k8s-action-link"
                      disabled={entry.isDir || entry.isSymlink}
                      onClick={() => void download(entry)}
                    >
                      下载
                    </button>
                    {canManage ? (
                      <button type="button" className="k8s-action-link is-danger" onClick={() => void remove(entry)}>
                        删除
                      </button>
                    ) : null}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
