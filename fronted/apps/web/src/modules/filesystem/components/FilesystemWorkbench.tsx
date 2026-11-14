"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { createElement } from "react";
import type { KeyboardEvent } from "react";
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
          "aria-controls": `workbench-panel-${tab.id}`,
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
        })
      : createElement("div", { className: "empty-state" }, "此功能将在后续任务中接入。");

  return createElement(
    "div",
    { className: "stack-lg" },
    tabList,
    createElement(
      "div",
      {
        id: `workbench-panel-${activeTab}`,
        role: "tabpanel",
        "aria-labelledby": `workbench-tab-${activeTab}`,
      },
      panel,
    ),
  );
}
