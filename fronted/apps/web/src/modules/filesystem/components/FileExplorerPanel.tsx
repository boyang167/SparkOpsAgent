"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { createDirectoryNode } from "@sparkops/web-sdk/filesystem";
import { FileBrowser } from "./FileBrowser";
import { createPanelRenameHandler, submitPanelRename } from "./file-explorer-panel-rename";
import {
  createFileBrowserMoveAdapter,
  type FileExplorerDialog,
  showCreateDialog,
  showRenameDialog,
} from "./file-explorer-panel-state";
import { UploadPanel } from "./UploadPanel";
import type { FileView } from "./workbench-state";

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
  view?: FileView;
  onRename?: (nodeId: string, name: string) => Promise<void>;
  onMove?: (nodeId: string) => void;
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
  view = "list",
  onRename,
  onMove,
  onCreateDirectory,
  onNavigate,
}: FileExplorerPanelProps) {
  const router = useRouter();
  const [nodes, setNodes] = useState(initialNodes);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [dialog, setDialog] = useState<FileExplorerDialog>(null);
  const [renameValue, setRenameValue] = useState("");
  const [createName, setCreateName] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const doRename = useMemo(
    () => createPanelRenameHandler(workspaceId, onRename),
    [onRename, workspaceId],
  );
  const moveAdapter = useMemo(() => createFileBrowserMoveAdapter(onMove), [onMove]);

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
    if (dialog?.type !== "rename" || !renameValue.trim()) return;
    setPending(true);
    setError(null);
    try {
      const nextName = await submitPanelRename(dialog.node.id, renameValue, doRename);
      setNodes((current) =>
        current.map((node) => (node.id === dialog.node.id ? { ...node, name: nextName } : node)),
      );
      setDialog(null);
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
      setDialog(null);
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
            setDialog(showCreateDialog());
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

      <FileBrowser
        nodes={nodes}
        view={view}
        selectedIds={selectedIds}
        onSelectionChange={setSelectedIds}
        onOpen={(node) =>
          router.push(
            node.type === "DIRECTORY"
              ? `/files/${node.id}`
              : `/files/${node.id}?mode=revisions`,
          )
        }
        onRename={(node) => {
          setDialog(showRenameDialog(node));
          setRenameValue(node.name);
        }}
        onMove={moveAdapter.onMove}
        canMove={moveAdapter.canMove}
      />

      {dialog?.type === "rename" ? (
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
              onClick={() => setDialog(null)}
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

      {dialog?.type === "create" ? (
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
              onClick={() => setDialog(null)}
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
