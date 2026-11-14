"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { createElement } from "react";
import type { FileNamespaceVM, FileNodeVM } from "@sparkops/contracts/filesystem";
import { FileExplorerPanel } from "./FileExplorerPanel";
import type { WorkbenchState, WorkbenchTab } from "./workbench-state";

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

  const tabList = createElement(
    "div",
    { role: "tablist", "aria-label": "文件工作台", className: "flex flex-wrap gap-2" },
    visibleTabs.map((tab) =>
      createElement(
        "button",
        {
          key: tab.id,
          type: "button",
          role: "tab",
          "aria-selected": activeTab === tab.id,
          className: activeTab === tab.id ? "btn btn-secondary" : "btn btn-ghost",
          onClick: () => selectTab(tab.id),
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
        })
      : createElement("div", { className: "empty-state" }, "此功能将在后续任务中接入。");

  return createElement(
    "div",
    { className: "stack-lg" },
    tabList,
    createElement("div", { role: "tabpanel" }, panel),
  );
}
