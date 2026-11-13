"use client";

import Link from "next/link";
import { useCallback, useMemo, useState } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { createDirectoryNode, renameFileNode } from "@sparkops/web-sdk/filesystem";
import { renameNodeKeepingIdentity } from "./file-explorer-rename";
import { UploadPanel } from "./UploadPanel";

export type BreadcrumbItem = {
  id: string;
  name: string;
};

export type FileExplorerPanelProps = {
  workspaceId: string;
  namespaceId?: string;
  parentId: string;
  initialNodes: FileNodeVM[];
  breadcrumb?: BreadcrumbItem[];
  providerId?: string;
  onRename?: (nodeId: string, name: string) => Promise<void>;
  onCreateDirectory?: (name: string) => Promise<void>;
  onNavigate?: () => void;
};

export function FileExplorerPanel({
  workspaceId,
  namespaceId,
  parentId,
  initialNodes,
  breadcrumb = [],
  providerId,
  onRename,
  onCreateDirectory,
  onNavigate,
}: FileExplorerPanelProps) {
  const [nodes, setNodes] = useState(initialNodes);
  const [renameTarget, setRenameTarget] = useState<FileNodeVM | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const sortedNodes = useMemo(
    () => [...nodes].sort((a, b) => a.name.localeCompare(b.name, "zh-CN")),
    [nodes],
  );

  const doRename = useCallback(
    async (nodeId: string, name: string) => {
      if (onRename) {
        await onRename(nodeId, name);
        return;
      }
      await renameFileNode(workspaceId, nodeId, { name });
    },
    [onRename, workspaceId],
  );

  const doCreateDirectory = useCallback(
    async (name: string): Promise<FileNodeVM | void> => {
      if (onCreateDirectory) {
        await onCreateDirectory(name);
        return;
      }
      return createDirectoryNode(workspaceId, {
        parent_id: parentId,
        namespace_id: namespaceId,
        name,
      });
    },
    [namespaceId, onCreateDirectory, parentId, workspaceId],
  );

  async function submitRename() {
    if (!renameTarget || !renameValue.trim()) return;
    setPending(true);
    setError(null);
    try {
      const nextName = renameValue.trim();
      await renameNodeKeepingIdentity(renameTarget.id, nextName, doRename);
      setNodes((current) =>
        current.map((node) => (node.id === renameTarget.id ? { ...node, name: nextName } : node)),
      );
      setRenameTarget(null);
      setRenameValue("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "重命名失败");
    } finally {
      setPending(false);
    }
  }

  async function submitCreateDirectory() {
    if (!createName.trim()) return;
    setPending(true);
    setError(null);
    try {
      const created = await doCreateDirectory(createName.trim());
      if (created) {
        setNodes((current) => [...current, created]);
      } else {
        onNavigate?.();
      }
      setCreateOpen(false);
      setCreateName("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建目录失败");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="stack-lg">
      <nav aria-label="路径" className="flex flex-wrap items-center gap-2 text-sm text-muted">
        <Link href="/files" className="link-muted">
          根目录
        </Link>
        {breadcrumb.map((item) => (
          <span key={item.id} className="flex items-center gap-2">
            <span aria-hidden>/</span>
            <Link href={`/files/${item.id}`} className="link-muted">
              {item.name}
            </Link>
          </span>
        ))}
      </nav>

      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => {
            setCreateOpen(true);
            setCreateName("");
          }}
        >
          新建目录
        </button>
        <UploadPanel
          workspaceId={workspaceId}
          parentId={parentId}
          namespaceId={namespaceId}
          providerId={providerId}
          onUploaded={onNavigate}
        />
      </div>

      {sortedNodes.length === 0 ? (
        <div className="empty-state">此目录为空。</div>
      ) : (
        <div className="card overflow-hidden">
          <table className="data-table w-full">
            <thead>
              <tr>
                <th className="text-left">名称</th>
                <th className="text-left">类型</th>
                <th className="text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {sortedNodes.map((node) => (
                <tr key={node.id}>
                  <td>
                    <Link
                      href={
                        node.type === "DIRECTORY"
                          ? `/files/${node.id}`
                          : `/files/${node.id}?mode=revisions`
                      }
                      className="font-medium link"
                    >
                      {node.name}
                    </Link>
                  </td>
                  <td className="text-muted">{node.type === "DIRECTORY" ? "目录" : "文件"}</td>
                  <td className="text-right">
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => {
                        setRenameTarget(node);
                        setRenameValue(node.name);
                      }}
                    >
                      重命名
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {renameTarget ? (
        <div className="card stack-sm max-w-md" role="dialog" aria-labelledby="rename-title">
          <h3 id="rename-title" className="font-semibold">
            重命名
          </h3>
          <label className="stack-xs">
            <span>新名称</span>
            <input
              className="input"
              value={renameValue}
              onChange={(event) => setRenameValue(event.target.value)}
            />
          </label>
          <div className="flex gap-2 justify-end">
            <button
              type="button"
              className="btn btn-ghost"
              disabled={pending}
              onClick={() => setRenameTarget(null)}
            >
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary"
              disabled={pending}
              onClick={() => void submitRename()}
            >
              保存
            </button>
          </div>
        </div>
      ) : null}

      {createOpen ? (
        <div className="card stack-sm max-w-md" role="dialog" aria-labelledby="mkdir-title">
          <h3 id="mkdir-title" className="font-semibold">
            新建目录
          </h3>
          <label className="stack-xs">
            <span>目录名称</span>
            <input
              className="input"
              value={createName}
              onChange={(event) => setCreateName(event.target.value)}
            />
          </label>
          <div className="flex gap-2 justify-end">
            <button
              type="button"
              className="btn btn-ghost"
              disabled={pending}
              onClick={() => setCreateOpen(false)}
            >
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary"
              disabled={pending}
              onClick={() => void submitCreateDirectory()}
            >
              创建
            </button>
          </div>
        </div>
      ) : null}

      {error ? <p className="text-sm text-danger">{error}</p> : null}
    </div>
  );
}
