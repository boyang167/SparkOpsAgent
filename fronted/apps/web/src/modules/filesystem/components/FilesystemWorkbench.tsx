"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { createElement, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { ChangeEvent, FormEvent, KeyboardEvent } from "react";
import type {
  FileNamespaceVM,
  FileNodeVM,
  GovernedPermission,
  NamespaceSummaryVM,
} from "@sparkops/contracts/filesystem";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import {
  abortTransfer,
  completeTransferSession,
  createTransferSession,
  getTransferSession,
  moveFileNode,
  uploadTransfer,
} from "@sparkops/web-sdk/filesystem";
import { Drawer } from "@/components/ui/Drawer";
import { AuditWorkspace } from "./AuditWorkspace";
import { DirectoryTree } from "./DirectoryTree";
import { FileDetailDrawer } from "./FileDetailDrawer";
import { nodePathInFolder } from "./file-node-display-path";
import { FileDropZone } from "./FileDropZone";
import { FileExplorerPanel } from "./FileExplorerPanel";
import { FileSpaceNav } from "./FileSpaceNav";
import { FileSpaceTabList } from "./FileSpaceTabList";
import { GovernanceWorkspace, type GovernanceSection } from "./GovernanceWorkspace";
import { MoveNodesModal } from "./MoveNodesModal";
import { ShareFileModal } from "./ShareFileModal";
import { ShareApprovalPanel } from "./ShareApprovalPanel";
import { TransferCenter, TransferSummary } from "./TransferCenter";
import { transferErrorMessage } from "./transfer-error-message";
import { createTransferQueue } from "./transfer-queue";
import type { TransferQueue } from "./transfer-queue";
import {
  hrefForDataWorkspace,
  type FileSpace,
  type FileView,
  type WorkbenchState,
  type WorkbenchTab,
} from "./workbench-state";
import {
  canDeleteGovernedFileSpace,
  canWriteGovernedFileSpace,
  resolveFilesystemAuditMode,
} from "./workbench-permissions";

export type FilesystemWorkbenchProps = {
  workspaceId: string;
  namespace: FileNamespaceVM;
  initialNodes: FileNodeVM[];
  initialNextCursor?: string;
  initialState: WorkbenchState;
  permissions: string[];
  initialSelectedNode?: FileNodeVM;
  initialSelectedNodeError?: "forbidden" | "not-found" | "error";
  fileSpace?: FileSpace;
  namespaceSummary?: NamespaceSummaryVM;
  organizationLabel?: string;
  workspaceLabel?: string;
  organizationId?: string;
  members?: WorkspaceMemberVM[];
  memberDirectoryAvailable?: boolean;
  lockedTab?: WorkbenchTab;
  hideTabList?: boolean;
  /** 存储治理四 Tab 中与路由对齐的当前项 */
  governanceSection?: GovernanceSection;
  currentFolderLabel?: string;
};

function fileSpaceLabel(space?: FileSpace) {
  switch (space) {
    case "my":
      return "我的文件";
    case "organization":
      return "租户文件";
    case "shared":
      return "与我共享";
    default:
      return "工作区文件";
  }
}

const tabs: Array<{
  id: WorkbenchTab;
  label: string;
  permission?: string;
}> = [
  { id: "files", label: "文件" },
  { id: "transfers", label: "传输" },
  { id: "governance", label: "存储治理", permission: "storage:read" },
  { id: "approvals", label: "共享审批", permission: "org:file:admin" },
  { id: "audit", label: "审计" },
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
  fileSpace,
  namespaceSummary,
  organizationLabel,
  workspaceLabel,
  organizationId,
  members = [],
  memberDirectoryAvailable = false,
  lockedTab,
  hideTabList = false,
  governanceSection,
  currentFolderLabel = "根目录",
}: FilesystemWorkbenchProps) {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const scopeKey = `${fileSpace ?? "workspace"}:${workspaceId}:${namespace.id}:${namespace.root_node_id ?? ""}`;
  const effectiveState = initialState;
  const scopedInitialSelectedNode = initialSelectedNode;
  const scopedInitialSelectedNodeError = initialSelectedNodeError;
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
  const [shareTarget, setShareTarget] = useState<{
    node: FileNodeVM;
    effectiveActions: GovernedPermission[];
  } | null>(null);
  const serverSelectionKey = [
    scopeKey,
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
  const transferQueueKey = scopeKey;
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
      throw new Error(transferErrorMessage(current.error_code));
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
          if (session.protocol === "MULTIPART") {
            await completeTransferSession(session.session_id, scopedWorkspaceId, {
              signal: controls.signal,
            });
          }
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
  const auditMode = resolveFilesystemAuditMode(permissions);
  const visibleTabs = tabs.filter((tab) =>
    tab.id === "audit"
      ? Boolean(auditMode)
      : !tab.permission || permissions.includes(tab.permission),
  );
  const activeTab =
    lockedTab ??
    (visibleTabs.some((tab) => tab.id === effectiveState.tab) ? effectiveState.tab : "files");
  const legacyWorkspaceWriteEnabled = canWriteGovernedFileSpace({
    permissions,
    fileSpace,
    namespaceSummary,
    namespace,
    workspaceId,
  });
  const legacyWorkspaceDeleteEnabled = canDeleteGovernedFileSpace({
    permissions,
    fileSpace,
    namespaceSummary,
    namespace,
    workspaceId,
  });

  function selectTab(tab: WorkbenchTab) {
    if (hideTabList) return;
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
    if (
      target.namespace_id !== namespace.id ||
      nodes.some((node) => node.namespace_id !== namespace.id)
    ) {
      setMoveError("跨空间请使用复制、转存或采纳");
      return;
    }
    setMoveError(null);
    try {
      for (const node of nodes) {
        await moveFileNode(workspaceId, node.id, target.id, node.name, {
          namespaceId: namespace.id,
        });
      }
      refreshDirectories();
    } catch {
      setMoveError("移动失败，已刷新源目录和目标目录，请重试。");
      refreshDirectories();
    }
  }

  function renderDirectoryTree() {
    if (!rootNode) return null;
    if (
      fileSpace &&
      namespaceSummary &&
      organizationLabel &&
      workspaceLabel
    ) {
      return createElement(FileSpaceNav, {
        activeSpace: fileSpace,
        organizationLabel,
        workspaceLabel,
        namespace: namespaceSummary,
        workspaceId,
        currentId: effectiveState.parent ?? rootNode.id,
        canWrite: legacyWorkspaceWriteEnabled,
        onNavigate: navigateDirectory,
        onDropNodes: (nodeIds: string[], target: FileNodeVM) => {
          void dropNodes(nodeIds, target);
        },
      });
    }
    return createElement(DirectoryTree, {
      key: `${scopeKey}:${treeVersion}`,
      workspaceId,
      namespaceId: namespace.id,
      rootNode,
      space: fileSpace ?? "workspace",
      currentId: effectiveState.parent ?? rootNode.id,
      onNavigate: navigateDirectory,
      onDropNodes: (nodeIds: string[], target: FileNodeVM) => {
        void dropNodes(nodeIds, target);
      },
      canWrite: legacyWorkspaceWriteEnabled,
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
          canWrite: legacyWorkspaceWriteEnabled,
          canMove: legacyWorkspaceWriteEnabled,
          canDelete: legacyWorkspaceDeleteEnabled,
          onMove: setMoveNodes,
          onOpen: openNode,
          organizationId,
          onShare: (node: FileNodeVM, effectiveActions: GovernedPermission[]) =>
            setShareTarget({ node, effectiveActions }),
          onFilesSelected: legacyWorkspaceWriteEnabled
            ? (files: File[]) =>
                transferQueue.enqueue(
                  files,
                  effectiveState.parent ?? namespace.root_node_id!,
                )
            : undefined,
          searchValue,
          onSearchChange: setSearchValue,
          onSearchSubmit: submitSearch,
          onViewChange: selectView,
          onOpenDirectory: () => setDirectoryDrawerOpen(true),
          spaceLabel: fileSpaceLabel(fileSpace),
          currentFolderLabel,
          workspaceRootHref: hrefForDataWorkspace(fileSpace ?? "workspace", view),
          isListingSearch: Boolean(effectiveState.query?.trim()),
        })
      : createElement("div", { className: "empty-state" }, "文件命名空间尚未初始化。");
  const filesSpaceTabs =
    fileSpace && namespaceSummary && organizationLabel && workspaceLabel
      ? createElement(FileSpaceTabList, { activeSpace: fileSpace, view })
      : null;

  const filesContent = createElement(
    "div",
    { className: "filesystem-data-workspace stack-sm min-w-0" },
    filesSpaceTabs,
    createElement(
    "div",
    {
      className: "filesystem-workspace-frame filesystem-browser-layout",
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
          legacyWorkspaceWriteEnabled,
        onFiles: (files: File[], targetParentId: string) =>
          transferQueue.enqueue(files, targetParentId),
      },
        createElement(
          "div",
          {
            className: "filesystem-browser-main",
            "aria-labelledby": "workbench-tab-files",
          },
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
    ),
  );

  return createElement(
    "div",
    { className: "filesystem-workbench stack-lg" },
    hideTabList ? null : tabList,
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
              ? createElement(TransferCenter, {
                  queue: transferQueue,
                  workspaceId,
                  canWrite: legacyWorkspaceWriteEnabled,
                })
              : tab.id === "governance"
                ? createElement(GovernanceWorkspace, {
                    workspaceId,
                    permissions,
                    activeSection: governanceSection,
                  })
                : tab.id === "approvals"
                  ? organizationId
                    ? createElement(ShareApprovalPanel, { organizationId, workspaceId })
                    : null
                  : auditMode
                    ? createElement(AuditWorkspace, {
                        workspaceId,
                        organizationId,
                        permissions,
                        mode: auditMode,
                      })
                    : null,
      ),
    ),
    activeTab === "files"
      ? createElement(TransferSummary, {
          queue: transferQueue,
          onViewAll: () => {
            if (hideTabList) {
              router.push("/files/data/transfers");
              return;
            }
            selectTab("transfers");
          },
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
          organizationId,
          namespaceRootId: namespace.root_node_id,
          displayPath:
            selected.node &&
            selected.node.parent_id === (effectiveState.parent ?? namespace.root_node_id)
              ? nodePathInFolder(currentFolderLabel, selected.node)
              : undefined,
          onClose: closeNode,
          onRetry: () => router.refresh(),
          members,
        })
      : null,
    shareTarget && organizationId && organizationLabel && workspaceLabel
      ? createElement(ShareFileModal, {
          open: true,
          node: shareTarget.node,
          organizationId,
          organizationLabel,
          workspaceId,
          workspaceLabel,
          members,
          memberDirectoryAvailable,
          effectiveActions: shareTarget.effectiveActions,
          onClose: () => setShareTarget(null),
        })
      : null,
  );
}
