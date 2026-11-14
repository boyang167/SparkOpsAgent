"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { createElement, useState } from "react";
import type { KeyboardEvent } from "react";
import type { FileNamespaceVM, FileNodeVM } from "@sparkops/contracts/filesystem";
import { moveFileNode } from "@sparkops/web-sdk/filesystem";
import { Drawer } from "@/components/ui/Drawer";
import { DirectoryTree } from "./DirectoryTree";
import { FileExplorerPanel } from "./FileExplorerPanel";
import { MoveNodesModal } from "./MoveNodesModal";
import type { FileView, WorkbenchState, WorkbenchTab } from "./workbench-state";

export type FilesystemWorkbenchProps = {
  workspaceId: string;
  namespace: FileNamespaceVM;
  initialNodes: FileNodeVM[];
  initialState: WorkbenchState;
  permissions: string[];
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

export function FilesystemWorkbench({
  workspaceId,
  namespace,
  initialNodes,
  initialState,
  permissions,
}: FilesystemWorkbenchProps) {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [view, setView] = useState<FileView>(initialState.view);
  const [directoryDrawerOpen, setDirectoryDrawerOpen] = useState(false);
  const [moveNodes, setMoveNodes] = useState<FileNodeVM[]>([]);
  const [treeVersion, setTreeVersion] = useState(0);
  const [moveError, setMoveError] = useState<string | null>(null);
  const visibleTabs = tabs.filter(
    (tab) => !tab.permission || permissions.includes(tab.permission),
  );
  const activeTab = visibleTabs.some((tab) => tab.id === initialState.tab)
    ? initialState.tab
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
      key: treeVersion,
      workspaceId,
      root: rootNode,
      currentId: initialState.parent ?? rootNode.id,
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
    { role: "tablist", "aria-label": "文件工作台", className: "flex flex-wrap gap-2" },
    visibleTabs.map((tab, tabIndex) =>
      createElement(
        "button",
        {
          key: tab.id,
          id: `workbench-tab-${tab.id}`,
          type: "button",
          role: "tab",
          "aria-selected": activeTab === tab.id,
          "aria-controls": activeTab === tab.id ? `workbench-panel-${tab.id}` : undefined,
          tabIndex: activeTab === tab.id ? 0 : -1,
          className: activeTab === tab.id ? "btn btn-secondary" : "btn btn-ghost",
          onClick: () => selectTab(tab.id),
          onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) =>
            handleTabKeyDown(event, tabIndex),
        },
        tab.label,
      ),
    ),
  );

  const panel =
    activeTab === "files" && namespace.root_node_id
      ? createElement(FileExplorerPanel, {
          workspaceId,
          namespaceId: namespace.id,
          parentId: initialState.parent ?? namespace.root_node_id,
          initialNodes,
          view,
          canMove: permissions.includes("file:write"),
          onMove: setMoveNodes,
        })
      : createElement("div", { className: "empty-state" }, "此功能将在后续任务中接入。");

  return createElement(
    "div",
    { className: "stack-lg" },
    tabList,
    activeTab === "files"
      ? createElement(
          "div",
          {
            role: "toolbar",
            "aria-label": "文件视图",
            className: "flex justify-end gap-2",
          },
          createElement(
            "button",
            {
              type: "button",
              className:
                view === "list" ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm",
              "aria-label": "列表视图",
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
                view === "grid" ? "btn btn-secondary btn-sm" : "btn btn-ghost btn-sm",
              "aria-label": "网格视图",
              "aria-pressed": view === "grid",
              onClick: () => selectView("grid"),
            },
            createElement("span", { "aria-hidden": true }, "▦"),
          ),
          createElement(
            "button",
            {
              type: "button",
              className: "btn btn-ghost btn-sm lg:hidden",
              "aria-label": "打开目录",
              onClick: () => setDirectoryDrawerOpen(true),
            },
            "目录",
          ),
        )
      : null,
    createElement(
      "div",
      {
        id: `workbench-panel-${activeTab}`,
        role: "tabpanel",
        "aria-labelledby": `workbench-tab-${activeTab}`,
      },
      activeTab === "files"
        ? createElement(
            "div",
            { className: "flex items-start gap-6" },
            createElement(
              "aside",
              {
                "aria-label": "目录",
                className: "hidden w-64 shrink-0 lg:block",
              },
              renderDirectoryTree(),
            ),
            createElement("div", { className: "min-w-0 flex-1" }, panel),
          )
        : panel,
    ),
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
  );
}
