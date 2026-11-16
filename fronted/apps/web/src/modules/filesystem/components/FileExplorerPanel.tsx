"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { FormEvent } from "react";
import type { FileNodeVM, GovernedPermission } from "@sparkops/contracts/filesystem";
import { createDirectoryNode } from "@sparkops/web-sdk/filesystem";
import { FileBrowser } from "./FileBrowser";
import { deleteDialogCopy, trashPanelNodes } from "./file-explorer-panel-delete";
import { createPanelRenameHandler, submitPanelRename } from "./file-explorer-panel-rename";
import type { FileExplorerDialog } from "./file-explorer-panel-state";
import { showCreateDialog, showDeleteDialog, showRenameDialog } from "./file-explorer-panel-state";
import { UploadPanel } from "./UploadPanel";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
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
  view?: FileView;
  onFilesSelected?: (files: File[]) => void;
  onRename?: (nodeId: string, name: string) => Promise<void>;
  onMove?: (nodes: FileNodeVM[]) => void;
  canMove?: boolean;
  canDelete?: boolean;
  onCreateDirectory?: (name: string) => Promise<void>;
  onNavigate?: () => void;
  onOpen?: (node: FileNodeVM) => void;
  canWrite?: boolean;
  organizationId?: string;
  onShare?: (node: FileNodeVM, effectiveActions: GovernedPermission[]) => void;
  searchValue?: string;
  onSearchChange?: (value: string) => void;
  onSearchSubmit?: () => void;
  onViewChange?: (view: FileView) => void;
  onOpenDirectory?: () => void;
  spaceLabel?: string;
  currentFolderLabel?: string;
  workspaceRootHref?: string;
  isListingSearch?: boolean;
};

export function FileExplorerPanel({
  workspaceId,
  namespaceId,
  parentId,
  initialNodes,
  breadcrumb = [],
  view = "list",
  onFilesSelected,
  onRename,
  onMove,
  canMove = false,
  canDelete = false,
  onCreateDirectory,
  onNavigate,
  onOpen,
  canWrite = true,
  organizationId,
  onShare,
  searchValue = "",
  onSearchChange,
  onSearchSubmit,
  onViewChange,
  onOpenDirectory,
  spaceLabel,
  currentFolderLabel = "根目录",
  workspaceRootHref = "/files/data/workspace",
  isListingSearch = false,
}: FileExplorerPanelProps) {
  const router = useRouter();
  const [nodes, setNodes] = useState(initialNodes);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [dialog, setDialog] = useState<FileExplorerDialog>(null);
  const [renameValue, setRenameValue] = useState("");
  const [createName, setCreateName] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const scopeKey = `${workspaceId}:${namespaceId ?? ""}:${parentId}`;
  const createInputRef = useRef<HTMLInputElement>(null);
  const renameInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setNodes(initialNodes);
    setSelectedIds([]);
    setDialog(null);
    setError(null);
  }, [initialNodes, scopeKey]);

  const doRename = useMemo(
    () => createPanelRenameHandler(workspaceId, onRename),
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

  function closeCreateDialog() {
    if (pending) return;
    setDialog(null);
    setCreateName("");
    setError(null);
  }

  function closeRenameDialog() {
    if (pending) return;
    setDialog(null);
    setRenameValue("");
    setError(null);
  }

  function closeDeleteDialog() {
    if (pending) return;
    setDialog(null);
    setError(null);
  }

  async function submitDelete() {
    if (dialog?.type !== "delete" || dialog.nodes.length === 0) return;
    setPending(true);
    setError(null);
    const result = await trashPanelNodes(
      workspaceId,
      dialog.nodes.map((node) => node.id),
      { namespaceId },
    );
    if (result.removed.length > 0) {
      const removed = new Set(result.removed);
      setNodes((current) => current.filter((node) => !removed.has(node.id)));
      setSelectedIds((current) => current.filter((nodeId) => !removed.has(nodeId)));
    }
    if (result.ok) {
      setDialog(null);
    } else {
      const removed = new Set(result.removed);
      const remaining = dialog.nodes.filter((node) => !removed.has(node.id));
      setError(result.message ?? "删除失败");
      setDialog(remaining.length > 0 ? showDeleteDialog(remaining) : null);
    }
    setPending(false);
  }

  useEffect(() => {
    if (dialog?.type !== "create") return;
    const timer = window.setTimeout(() => createInputRef.current?.focus(), 0);
    return () => window.clearTimeout(timer);
  }, [dialog?.type]);

  useEffect(() => {
    if (dialog?.type !== "rename") return;
    const timer = window.setTimeout(() => {
      renameInputRef.current?.focus();
      renameInputRef.current?.select();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [dialog]);

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

  const nodeCountLabel = `${nodes.length} 个项目`;
  const locationLabel = isListingSearch
    ? "搜索结果"
    : currentFolderLabel || "根目录";

  return (
  <div className="filesystem-explorer-shell">
    <header className="filesystem-explorer-header">
      <nav aria-label="当前位置" className="filesystem-explorer-location">
        <span className="filesystem-location-space">{spaceLabel ?? "文件"}</span>
        <span className="filesystem-location-sep" aria-hidden="true">
          /
        </span>
        <Link href={workspaceRootHref} className="filesystem-location-link">
          根目录
        </Link>
        {!isListingSearch && locationLabel !== "根目录" ? (
          <>
            <span className="filesystem-location-sep" aria-hidden="true">
              /
            </span>
            <span className="filesystem-location-current">{locationLabel}</span>
          </>
        ) : null}
        {isListingSearch ? (
          <>
            <span className="filesystem-location-sep" aria-hidden="true">
              /
            </span>
            <span className="filesystem-location-current">搜索结果</span>
          </>
        ) : null}
        <span className="filesystem-location-count">{nodeCountLabel}</span>
      </nav>

      <div className="filesystem-explorer-command-bar">
        <div className="filesystem-explorer-command-primary">
          {canWrite ? (
            <div className="filesystem-explorer-actions">
              <button
                type="button"
                className="btn btn-secondary btn-sm filesystem-touch-target"
                onClick={() => {
                  setError(null);
                  setDialog(showCreateDialog());
                  setCreateName("");
                }}
              >
                新建文件夹
              </button>
              {onFilesSelected ? (
                <UploadPanel
                  onFilesSelected={onFilesSelected}
                  emphasis={dialog ? "secondary" : "primary"}
                  size="sm"
                />
              ) : null}
            </div>
          ) : (
            <span className="filesystem-explorer-readonly-hint text-sm text-muted">只读浏览</span>
          )}
        </div>

        {onSearchSubmit && onSearchChange ? (
          <form
            role="search"
            aria-label="文件搜索"
            className="filesystem-explorer-search-form"
            onSubmit={(event) => {
              event.preventDefault();
              onSearchSubmit();
            }}
          >
            <input
              type="search"
              className="input input-sm filesystem-explorer-search"
              aria-label="搜索文件"
              placeholder="搜索名称或路径，Enter 确认"
              value={searchValue}
              onChange={(event) => onSearchChange(event.target.value)}
            />
          </form>
        ) : (
          <div className="filesystem-explorer-search-form" aria-hidden="true" />
        )}

        <div className="filesystem-explorer-command-secondary">
          {onViewChange ? (
            <div className="filesystem-view-toggle" role="group" aria-label="视图切换">
              <button
                type="button"
                className={view === "list" ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm"}
                aria-label="列表视图"
                aria-pressed={view === "list"}
                onClick={() => onViewChange("list")}
              >
                列表
              </button>
              <button
                type="button"
                className={view === "grid" ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm"}
                aria-label="网格视图"
                aria-pressed={view === "grid"}
                onClick={() => onViewChange("grid")}
              >
                网格
              </button>
            </div>
          ) : null}
          {onOpenDirectory ? (
            <button
              type="button"
              className="btn btn-ghost btn-sm filesystem-directory-trigger filesystem-touch-target"
              aria-label="打开文件夹导航"
              onClick={onOpenDirectory}
            >
              文件夹
            </button>
          ) : null}
        </div>
      </div>
    </header>

    <div className="filesystem-explorer-body">
      <FileBrowser
        nodes={nodes}
        workspaceId={workspaceId}
        namespaceId={namespaceId}
        view={view}
        selectedIds={selectedIds}
        onSelectionChange={setSelectedIds}
        onOpen={(node) => {
          if (onOpen) {
            onOpen(node);
            return;
          }
          router.push(
            node.type === "DIRECTORY"
              ? `/files/${node.id}`
              : `/files/${node.id}?mode=revisions`,
          );
        }}
        onRename={(node) => {
          setDialog(showRenameDialog(node));
          setRenameValue(node.name);
        }}
        onMove={(moveNodes) => onMove?.(moveNodes)}
        onDelete={(deleteNodes) => {
          setError(null);
          setDialog(showDeleteDialog(deleteNodes));
        }}
        canRename={canWrite}
        canMove={canMove && Boolean(onMove)}
        canDelete={canDelete}
        organizationId={organizationId}
        onShare={onShare}
      />
    </div>

      {!dialog && error ? (
        <p className="filesystem-explorer-inline-error text-sm text-danger">{error}</p>
      ) : null}

      <Modal
        open={dialog?.type === "create"}
        title="新建文件夹"
        description={`在「${locationLabel}」下创建子目录`}
        size="sm"
        onClose={closeCreateDialog}
        footer={
          <div className="flex flex-wrap justify-end gap-2">
            <button
              type="button"
              className="btn btn-ghost"
              disabled={pending}
              onClick={closeCreateDialog}
            >
              取消
            </button>
            <button
              type="submit"
              form="filesystem-create-folder-form"
              className="btn btn-primary"
              disabled={pending || !createName.trim()}
            >
              {pending ? "创建中…" : "创建"}
            </button>
          </div>
        }
      >
        <form
          id="filesystem-create-folder-form"
          className="stack-md"
          onSubmit={(event: FormEvent<HTMLFormElement>) => {
            event.preventDefault();
            void submitCreateDirectory();
          }}
        >
          {error && dialog?.type === "create" ? (
            <InlineAlert tone="error">{error}</InlineAlert>
          ) : null}
          <label className="stack-xs">
            <span>文件夹名称</span>
            <input
              ref={createInputRef}
              className="input"
              placeholder="例如：项目资料"
              autoComplete="off"
              disabled={pending}
              value={createName}
              onChange={(event) => {
                setError(null);
                setCreateName(event.target.value);
              }}
            />
          </label>
        </form>
      </Modal>

      <Modal
        open={dialog?.type === "rename"}
        title="重命名"
        description={
          dialog?.type === "rename" ? `当前名称：${dialog.node.name}` : undefined
        }
        size="sm"
        onClose={closeRenameDialog}
        footer={
          <div className="flex flex-wrap justify-end gap-2">
            <button
              type="button"
              className="btn btn-ghost"
              disabled={pending}
              onClick={closeRenameDialog}
            >
              取消
            </button>
            <button
              type="submit"
              form="filesystem-rename-form"
              className="btn btn-primary"
              disabled={pending || !renameValue.trim()}
            >
              {pending ? "保存中…" : "保存"}
            </button>
          </div>
        }
      >
        <form
          id="filesystem-rename-form"
          className="stack-md"
          onSubmit={(event: FormEvent<HTMLFormElement>) => {
            event.preventDefault();
            void submitRename();
          }}
        >
          {error && dialog?.type === "rename" ? (
            <InlineAlert tone="error">{error}</InlineAlert>
          ) : null}
          <label className="stack-xs">
            <span>新名称</span>
            <input
              ref={renameInputRef}
              className="input"
              autoComplete="off"
              disabled={pending}
              value={renameValue}
              onChange={(event) => {
                setError(null);
                setRenameValue(event.target.value);
              }}
            />
          </label>
        </form>
      </Modal>

      <Modal
        open={dialog?.type === "delete"}
        title={dialog?.type === "delete" ? deleteDialogCopy(dialog.nodes).title : "删除"}
        description={
          dialog?.type === "delete" ? deleteDialogCopy(dialog.nodes).description : undefined
        }
        size="sm"
        onClose={closeDeleteDialog}
        footer={
          <div className="flex flex-wrap justify-end gap-2">
            <button
              type="button"
              className="btn btn-ghost"
              disabled={pending}
              onClick={closeDeleteDialog}
            >
              取消
            </button>
            <button
              type="button"
              className="btn-danger-solid"
              disabled={pending || dialog?.type !== "delete" || dialog.nodes.length === 0}
              onClick={() => void submitDelete()}
            >
              {pending ? "删除中…" : "删除"}
            </button>
          </div>
        }
      >
        {error && dialog?.type === "delete" ? (
          <InlineAlert tone="error">{error}</InlineAlert>
        ) : null}
      </Modal>
    </div>
  );
}
