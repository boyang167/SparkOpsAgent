"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { createElement, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { ChangeEvent, FormEvent, KeyboardEvent } from "react";
import type { FileNamespaceVM, FileNodeVM } from "@sparkops/contracts/filesystem";
import {
  abortTransfer,
  createTransferSession,
  getTransferSession,
  moveFileNode,
  uploadTransfer,
} from "@sparkops/web-sdk/filesystem";
import { Drawer } from "@/components/ui/Drawer";
import { AuditWorkspace } from "./AuditWorkspace";
import { DirectoryTree } from "./DirectoryTree";
import { FileDetailDrawer } from "./FileDetailDrawer";
import { FileDropZone } from "./FileDropZone";
import { FileExplorerPanel } from "./FileExplorerPanel";
import { GovernanceWorkspace } from "./GovernanceWorkspace";
import { MoveNodesModal } from "./MoveNodesModal";
import { TransferCenter, TransferSummary } from "./TransferCenter";
import { createTransferQueue } from "./transfer-queue";
import type { TransferQueue } from "./transfer-queue";
import type { FileView, WorkbenchState, WorkbenchTab } from "./workbench-state";

export type FilesystemWorkbenchProps = {
  workspaceId: string;
  namespace: FileNamespaceVM;
  initialNodes: FileNodeVM[];
  initialNextCursor?: string;
  initialState: WorkbenchState;
  permissions: string[];
  initialSelectedNode?: FileNodeVM;
  initialSelectedNodeError?: "forbidden" | "not-found" | "error";
};

const tabs: Array<{
  id: WorkbenchTab;
  label: string;
  permission?: string;
}> = [
  { id: "files", label: "文件" },
  { id: "transfers", label: "传输" },
  { id: "governance", label: "存储治理", permission: "storage:read" },
  { id: "audit", label: "审计", permission: "file:admin" },
];

function uploadIdempotencyKey() {
  return `workbench-upload-${
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
  }`;
}

function abortableDelay(milliseconds: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const onAbort = () => {
      clearTimeout(timer);
      reject(new DOMException("Aborted", "AbortError"));
    };
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", onAbort);
      resolve();
    }, milliseconds);
    signal.addEventListener("abort", onAbort, { once: true });
  });
}

export function FilesystemWorkbench({
  workspaceId,
  namespace,
  initialNodes,
  initialNextCursor = "",
  initialState,
  permissions,
  initialSelectedNode,
  initialSelectedNodeError,
}: FilesystemWorkbenchProps) {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const scopeKey = `${workspaceId}:${namespace.id}:${namespace.root_node_id ?? ""}`;
  const scopeKeyRef = useRef(scopeKey);
  const scopeCleanupRef = useRef<{
    key: string;
    parent?: string;
    node?: string;
    cursor?: string;
  } | null>(null);
  const scopeChanged = scopeKeyRef.current !== scopeKey;
  if (scopeChanged) {
    scopeCleanupRef.current = {
      key: scopeKey,
      parent: initialState.parent,
      node: initialState.node,
      cursor: initialState.cursor,
    };
  }
  const cleanup =
    scopeCleanupRef.current?.key === scopeKey ? scopeCleanupRef.current : null;
  const staleParent = Boolean(
    cleanup?.parent && cleanup.parent === initialState.parent,
  );
  const staleNode = Boolean(cleanup?.node && cleanup.node === initialState.node);
  const staleCursor = Boolean(
    cleanup?.cursor && cleanup.cursor === initialState.cursor,
  );
  const effectiveState = {
    ...initialState,
    parent: staleParent ? undefined : initialState.parent,
    node: staleNode ? undefined : initialState.node,
    cursor: staleCursor ? undefined : initialState.cursor,
  };
  const scopedInitialSelectedNode = staleNode
    ? undefined
    : initialSelectedNode;
  const scopedInitialSelectedNodeError = staleNode
    ? undefined
    : initialSelectedNodeError;
  const [view, setView] = useState<FileView>(initialState.view);
  const [searchValue, setSearchValue] = useState(initialState.query);
  const cursorTrail = useRef<string[]>([]);
  const cursorScopeRef = useRef(
    `${scopeKey}:${initialState.parent ?? ""}:${initialState.query}`,
  );
  const [directoryDrawerOpen, setDirectoryDrawerOpen] = useState(false);
  const [moveNodes, setMoveNodes] = useState<FileNodeVM[]>([]);
  const [treeVersion, setTreeVersion] = useState(0);
  const [moveError, setMoveError] = useState<string | null>(null);
  const serverSelectionKey = [
    workspaceId,
    effectiveState.node ?? "",
    scopedInitialSelectedNode?.id ?? "",
    scopedInitialSelectedNodeError ?? "",
  ].join(":");
  const serverSelectionKeyRef = useRef(serverSelectionKey);
  const [selection, setSelection] = useState<{
    node: FileNodeVM | null;
    error?: "forbidden" | "not-found" | "error";
  }>({
    node: scopedInitialSelectedNode ?? null,
    error: scopedInitialSelectedNodeError,
  });
  const selected =
    serverSelectionKeyRef.current === serverSelectionKey
      ? selection
      : {
          node: scopedInitialSelectedNode ?? null,
          error: scopedInitialSelectedNodeError,
        };
  useLayoutEffect(() => {
    if (serverSelectionKeyRef.current === serverSelectionKey) return;
    serverSelectionKeyRef.current = serverSelectionKey;
    setSelection({
      node: scopedInitialSelectedNode ?? null,
      error: scopedInitialSelectedNodeError,
    });
  }, [scopedInitialSelectedNode, scopedInitialSelectedNodeError, serverSelectionKey]);
  useLayoutEffect(() => {
    if (scopeKeyRef.current === scopeKey) return;
    scopeKeyRef.current = scopeKey;
    setView(initialState.view);
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    nextSearchParams.delete("parent");
    nextSearchParams.delete("node");
    nextSearchParams.delete("cursor");
    const query = nextSearchParams.toString();
    router.replace(query ? `${pathname}?${query}` : pathname);
  }, [initialState.view, pathname, router, scopeKey, searchParams]);
  useLayoutEffect(() => {
    setSearchValue(initialState.query);
    const cursorScope = `${scopeKey}:${initialState.parent ?? ""}:${initialState.query}`;
    if (cursorScopeRef.current !== cursorScope) {
      cursorScopeRef.current = cursorScope;
      cursorTrail.current = [];
    }
  }, [initialState.parent, initialState.query, scopeKey]);
  useLayoutEffect(() => {
    setView(initialState.view);
  }, [initialState.view, scopeKey]);
  const transferQueues = useRef(new Map<string, TransferQueue>());
  const queueLifecycleGeneration = useRef(0);
  const transferQueueKey = `${workspaceId}:${namespace.id}`;
  let transferQueue = transferQueues.current.get(transferQueueKey);
  if (!transferQueue) {
    const scopedWorkspaceId = workspaceId;
    const scopedNamespaceId = namespace.id;
    transferQueue = createTransferQueue({
      concurrency: 3,
      abort: async (session) => {
        try {
          await abortTransfer(session.session_id, scopedWorkspaceId);
          return "canceled";
        } catch (abortError) {
          const current = await getTransferSession(session.session_id, scopedWorkspaceId);
          if (current.status === "COMPLETED") {
            router.refresh();
            return "completed";
          }
          if (current.status === "ABORTED") return "canceled";
          throw abortError;
        }
      },
      upload: async (file, targetParentId, controls) => {
        const input = {
          parent_id: targetParentId,
          namespace_id: scopedNamespaceId,
          name: file.name,
          protocol: "DIRECT",
          size: file.size,
          classification: "INTERNAL",
        } as const;
        const terminalStatuses = new Set(["FAILED", "REJECTED", "ABORTED", "EXPIRED"]);

        async function createWithKey(idempotencyKey: string) {
          try {
            return await createTransferSession(scopedWorkspaceId, input, {
              idempotencyKey,
              signal: controls.signal,
            });
          } catch (error) {
            if (!controls.signal.aborted) throw error;
            return createTransferSession(scopedWorkspaceId, input, { idempotencyKey });
          }
        }

        async function waitForCompletion(
          sessionId: string,
          initial?: Awaited<ReturnType<typeof getTransferSession>>,
        ) {
          controls.setStatus("verifying");
          let current =
            initial ??
            (await getTransferSession(sessionId, scopedWorkspaceId, {
              signal: controls.signal,
            }));
          while (!controls.signal.aborted) {
            if (current.status === "COMPLETED") {
              router.refresh();
              return;
            }
            if (terminalStatuses.has(current.status)) {
              throw new Error(current.error_code || "上传处理失败");
            }
            await abortableDelay(500, controls.signal);
            current = await getTransferSession(sessionId, scopedWorkspaceId, {
              signal: controls.signal,
            });
          }
          throw new DOMException("Aborted", "AbortError");
        }

        async function uploadSession(
          session: Awaited<ReturnType<typeof createTransferSession>>,
        ) {
          if (controls.signal.aborted) {
            throw new DOMException("Aborted", "AbortError");
          }
          controls.setStatus("uploading");
          await uploadTransfer(file, {
            workspaceId: scopedWorkspaceId,
            session,
            signal: controls.signal,
            onProgress: (progress) =>
              controls.setProgress({
                uploadedBytes: progress.uploadedBytes,
                totalBytes: progress.totalBytes,
              }),
          });
          await waitForCompletion(session.session_id);
        }

        async function startFreshSession() {
          const idempotencyKey = uploadIdempotencyKey();
          controls.setIdempotencyKey(idempotencyKey);
          controls.setProgress({ uploadedBytes: 0, totalBytes: file.size });
          const session = await createWithKey(idempotencyKey);
          controls.setSession(session);
          await uploadSession(session);
        }

        if (controls.retrying && controls.session) {
          const current = await getTransferSession(
            controls.session.session_id,
            scopedWorkspaceId,
            { signal: controls.signal },
          );
          if (current.status === "COMPLETED") {
            router.refresh();
            return;
          }
          if (terminalStatuses.has(current.status)) {
            controls.setSession(undefined);
            await startFreshSession();
            return;
          }
          if (current.status === "INITIATED") {
            await uploadSession(current);
            return;
          }
          await waitForCompletion(current.session_id, current);
          return;
        }

        if (controls.retrying && controls.idempotencyKey) {
          const recovered = await createWithKey(controls.idempotencyKey);
          controls.setSession(recovered);
          if (recovered.status === "COMPLETED") {
            router.refresh();
            return;
          }
          if (terminalStatuses.has(recovered.status)) {
            controls.setSession(undefined);
            await startFreshSession();
            return;
          }
          if (recovered.status !== "INITIATED") {
            await waitForCompletion(recovered.session_id, recovered);
            return;
          }
          await uploadSession(recovered);
          return;
        }

        await startFreshSession();
      },
    });
    transferQueues.current.set(transferQueueKey, transferQueue);
  }
  useEffect(() => {
    queueLifecycleGeneration.current += 1;
    return () => {
      const cleanupGeneration = ++queueLifecycleGeneration.current;
      queueMicrotask(() => {
        if (cleanupGeneration !== queueLifecycleGeneration.current) return;
        transferQueues.current.forEach((queue) => queue.dispose());
        transferQueues.current.clear();
      });
    };
  }, []);
  const visibleTabs = tabs.filter(
    (tab) => !tab.permission || permissions.includes(tab.permission),
  );
  const activeTab = visibleTabs.some((tab) => tab.id === effectiveState.tab)
    ? effectiveState.tab
    : "files";

  function selectTab(tab: WorkbenchTab) {
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    nextSearchParams.set("tab", tab);
    router.replace(`${pathname}?${nextSearchParams.toString()}`);
  }

  function selectView(nextView: FileView) {
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    nextSearchParams.set("view", nextView);
    setView(nextView);
    window.history.replaceState(null, "", `${pathname}?${nextSearchParams.toString()}`);
  }

  function openNode(node: FileNodeVM) {
    if (node.type === "DIRECTORY") {
      navigateDirectory(node);
      return;
    }
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    nextSearchParams.set("node", node.id);
    setSelection({ node });
    router.replace(`${pathname}?${nextSearchParams.toString()}`);
  }

  function closeNode() {
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    nextSearchParams.delete("node");
    setSelection({ node: null });
    const query = nextSearchParams.toString();
    router.replace(query ? `${pathname}?${query}` : pathname);
  }

  const rootNode: FileNodeVM | null = namespace.root_node_id
    ? {
        id: namespace.root_node_id,
        namespace_id: namespace.id,
        parent_id: null,
        type: "DIRECTORY",
        name: "全部文件",
        current_revision_id: null,
        state: "ACTIVE",
        owner_id: namespace.id,
      }
    : null;

  function navigateDirectory(node: FileNodeVM) {
    setDirectoryDrawerOpen(false);
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    nextSearchParams.delete("cursor");
    if (node.id === namespace.root_node_id) {
      nextSearchParams.delete("parent");
    } else {
      nextSearchParams.set("parent", node.id);
    }
    const query = nextSearchParams.toString();
    router.push(query ? `${pathname}?${query}` : pathname);
  }

  function submitSearch() {
    cursorTrail.current = [];
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    const query = searchValue.trim();
    if (query) nextSearchParams.set("q", query);
    else nextSearchParams.delete("q");
    nextSearchParams.delete("cursor");
    const nextQuery = nextSearchParams.toString();
    router.push(nextQuery ? `${pathname}?${nextQuery}` : pathname);
  }

  function nextPage() {
    if (!initialNextCursor) return;
    cursorTrail.current.push(effectiveState.cursor ?? "");
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    nextSearchParams.set("cursor", initialNextCursor);
    router.push(`${pathname}?${nextSearchParams.toString()}`);
  }

  function previousPage() {
    if (cursorTrail.current.length === 0) return;
    const previousCursor = cursorTrail.current.pop() ?? "";
    const nextSearchParams = new URLSearchParams(searchParams.toString());
    if (previousCursor) nextSearchParams.set("cursor", previousCursor);
    else nextSearchParams.delete("cursor");
    const query = nextSearchParams.toString();
    router.push(query ? `${pathname}?${query}` : pathname);
  }

  function refreshDirectories() {
    setTreeVersion((value) => value + 1);
    router.refresh();
  }

  async function dropNodes(nodeIds: string[], target: FileNodeVM) {
    const nodes = initialNodes.filter((node) => nodeIds.includes(node.id));
    if (nodes.length !== nodeIds.length) {
      setMoveError("无法确认待移动项目，请刷新后重试。");
      refreshDirectories();
      return;
    }
    setMoveError(null);
    try {
      for (const node of nodes) {
        await moveFileNode(workspaceId, node.id, target.id, node.name);
      }
      refreshDirectories();
    } catch {
      setMoveError("移动失败，已刷新源目录和目标目录，请重试。");
      refreshDirectories();
    }
  }

  function renderDirectoryTree() {
    if (!rootNode) return null;
    return createElement(DirectoryTree, {
      key: `${scopeKey}:${treeVersion}`,
      workspaceId,
      root: rootNode,
      currentId: effectiveState.parent ?? rootNode.id,
      onNavigate: navigateDirectory,
      onDropNodes: (nodeIds: string[], target: FileNodeVM) => {
        void dropNodes(nodeIds, target);
      },
      canWrite: permissions.includes("file:write"),
    });
  }

  function handleTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, tabIndex: number) {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;

    event.preventDefault();
    const offset = event.key === "ArrowRight" ? 1 : -1;
    const nextIndex = (tabIndex + offset + visibleTabs.length) % visibleTabs.length;
    const nextTab = visibleTabs[nextIndex];
    if (!nextTab) return;

    document.getElementById(`workbench-tab-${nextTab.id}`)?.focus();
    selectTab(nextTab.id);
  }

  const tabList = createElement(
    "div",
    {
      role: "tablist",
      "aria-label": "文件系统功能",
      className: "filesystem-workbench-tabs flex flex-wrap gap-2",
    },
    visibleTabs.map((tab, tabIndex) =>
      createElement(
        "button",
        {
          key: tab.id,
          id: `workbench-tab-${tab.id}`,
          type: "button",
          role: "tab",
          "aria-selected": activeTab === tab.id,
          "aria-controls": `workbench-panel-${tab.id}`,
          tabIndex: activeTab === tab.id ? 0 : -1,
          className:
            activeTab === tab.id
              ? "btn btn-secondary filesystem-touch-target"
              : "btn btn-ghost filesystem-touch-target",
          onClick: () => selectTab(tab.id),
          onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) =>
            handleTabKeyDown(event, tabIndex),
        },
        tab.label,
      ),
    ),
  );

  const filesPanel =
    namespace.root_node_id
      ? createElement(FileExplorerPanel, {
          workspaceId,
          namespaceId: namespace.id,
          key: `${scopeKey}:${effectiveState.parent ?? namespace.root_node_id}`,
          parentId: effectiveState.parent ?? namespace.root_node_id,
          initialNodes,
          view,
          canWrite: permissions.includes("file:write"),
          canMove: permissions.includes("file:write"),
          onMove: setMoveNodes,
          onOpen: openNode,
          onFilesSelected: permissions.includes("file:write")
            ? (files: File[]) =>
                transferQueue.enqueue(
                  files,
                  effectiveState.parent ?? namespace.root_node_id!,
                )
            : undefined,
        })
      : createElement("div", { className: "empty-state" }, "文件命名空间尚未初始化。");
  const filesContent = createElement(
    "div",
    {
      className: "filesystem-browser-layout gap-6",
      "data-testid": "filesystem-browser-layout",
    },
    createElement(
      "aside",
      {
        "aria-label": "目录",
        className: "filesystem-directory-sidebar",
      },
      renderDirectoryTree(),
    ),
    createElement(
      FileDropZone,
      {
        targetParentId: effectiveState.parent ?? namespace.root_node_id ?? "",
        targetName: "当前目录",
        canWrite:
          Boolean(effectiveState.parent ?? namespace.root_node_id) &&
          permissions.includes("file:write"),
        onFiles: (files: File[], targetParentId: string) =>
          transferQueue.enqueue(files, targetParentId),
      },
      createElement(
        "div",
        {
          className: "filesystem-browser-main",
          "aria-labelledby": "workbench-tab-files",
        },
        createElement(
          "form",
          {
            role: "search",
            "aria-label": "文件搜索",
            className: "mb-4 flex flex-wrap gap-2",
            onSubmit: (event: FormEvent) => {
              event.preventDefault();
              submitSearch();
            },
          },
          createElement("input", {
            type: "search",
            className: "input min-w-0 flex-1",
            "aria-label": "搜索文件",
            value: searchValue,
            onChange: (event: ChangeEvent<HTMLInputElement>) =>
              setSearchValue(event.currentTarget.value),
          }),
          createElement(
            "button",
            { type: "submit", className: "btn btn-secondary" },
            "搜索",
          ),
        ),
        filesPanel,
        createElement(
          "nav",
          { "aria-label": "文件分页", className: "mt-4 flex gap-2" },
          cursorTrail.current.length > 0
            ? createElement(
                "button",
                {
                  type: "button",
                  className: "btn btn-ghost btn-sm",
                  onClick: previousPage,
                },
                "上一页",
              )
            : null,
          initialNextCursor
            ? createElement(
                "button",
                {
                  type: "button",
                  className: "btn btn-secondary btn-sm",
                  onClick: nextPage,
                },
                "下一页",
              )
            : null,
        ),
      ),
    ),
  );

  return createElement(
    "div",
    { className: "filesystem-workbench stack-lg" },
    tabList,
    activeTab === "files"
      ? createElement(
          "div",
          {
            role: "toolbar",
            "aria-label": "文件视图",
            className: "filesystem-workbench-toolbar flex flex-wrap justify-end gap-2",
          },
          createElement(
            "button",
            {
              type: "button",
              className:
                view === "list"
                  ? "btn btn-secondary btn-sm filesystem-touch-target"
                  : "btn btn-ghost btn-sm filesystem-touch-target",
              "aria-label": "切换到列表视图",
              "aria-pressed": view === "list",
              onClick: () => selectView("list"),
            },
            createElement("span", { "aria-hidden": true }, "☰"),
          ),
          createElement(
            "button",
            {
              type: "button",
              className:
                view === "grid"
                  ? "btn btn-secondary btn-sm filesystem-touch-target"
                  : "btn btn-ghost btn-sm filesystem-touch-target",
              "aria-label": "切换到网格视图",
              "aria-pressed": view === "grid",
              onClick: () => selectView("grid"),
            },
            createElement("span", { "aria-hidden": true }, "▦"),
          ),
          createElement(
            "button",
            {
              type: "button",
              className:
                "btn btn-ghost btn-sm filesystem-directory-trigger filesystem-touch-target",
              "aria-label": "打开目录",
              onClick: () => setDirectoryDrawerOpen(true),
            },
            "目录",
          ),
        )
      : null,
    visibleTabs.map((tab) =>
      createElement(
        "div",
        {
          key: tab.id,
          id: `workbench-panel-${tab.id}`,
          role: "tabpanel",
          "aria-labelledby": `workbench-tab-${tab.id}`,
          hidden: activeTab !== tab.id,
        },
        activeTab !== tab.id
          ? null
          : tab.id === "files"
            ? filesContent
            : tab.id === "transfers"
              ? createElement(TransferCenter, { queue: transferQueue, workspaceId })
              : tab.id === "governance"
                ? createElement(GovernanceWorkspace, { workspaceId, permissions })
                : createElement(AuditWorkspace, { workspaceId, permissions }),
      ),
    ),
    activeTab === "files"
      ? createElement(TransferSummary, {
          queue: transferQueue,
          onViewAll: () => selectTab("transfers"),
        })
      : null,
    moveError
      ? createElement(
          "p",
          { role: "alert", className: "text-sm text-danger" },
          moveError,
        )
      : null,
    rootNode
      ? createElement(MoveNodesModal, {
          open: moveNodes.length > 0,
          workspaceId,
          root: rootNode,
          nodes: moveNodes,
          onClose: () => setMoveNodes([]),
          onMoved: (nodes: FileNodeVM[], target: FileNodeVM) => {
            const directories = new Set(
              nodes
                .map((node) => node.parent_id)
                .filter((id): id is string => Boolean(id)),
            );
            directories.add(target.id);
            refreshDirectories();
            setMoveNodes([]);
          },
          onRefreshDirectories: refreshDirectories,
        })
      : null,
    activeTab === "files"
      ? createElement(
          Drawer,
          {
            open: directoryDrawerOpen,
            onClose: () => setDirectoryDrawerOpen(false),
            title: "目录",
            children: renderDirectoryTree(),
          },
        )
      : null,
    selected.node || selected.error
      ? createElement(FileDetailDrawer, {
          open: true,
          node: selected.node,
          loadError: selected.error,
          workspaceId,
          onClose: closeNode,
          onRetry: () => router.refresh(),
        })
      : null,
  );
}
