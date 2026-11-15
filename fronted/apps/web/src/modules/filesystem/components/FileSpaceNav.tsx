"use client";

import { createElement } from "react";
import type { NamespaceSummaryVM, FileNodeVM } from "@sparkops/contracts/filesystem";
import { DirectoryTree } from "./DirectoryTree";
import type { FileSpace } from "./workbench-state";

export function FileSpaceNav({
  activeSpace,
  organizationLabel,
  workspaceLabel,
  namespace,
  workspaceId,
  currentId,
  canWrite = false,
  onNavigate = () => undefined,
  onDropNodes = () => undefined,
}: {
  activeSpace: FileSpace;
  organizationLabel: string;
  workspaceLabel: string;
  namespace?: NamespaceSummaryVM;
  workspaceId: string;
  currentId?: string;
  canWrite?: boolean;
  onNavigate?: (node: FileNodeVM) => void;
  onDropNodes?: (nodeIds: string[], target: FileNodeVM) => void;
}) {
  const root: FileNodeVM | undefined = namespace?.root_node_id
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
    : undefined;

  return createElement(
    "div",
    { className: "filesystem-space-nav min-w-0" },
    createElement(
      "p",
      { className: "filesystem-space-eyebrow" },
      `${organizationLabel} · ${workspaceLabel}`,
    ),
    activeSpace !== "shared" && root
      ? createElement(
          "section",
          {
            className: "filesystem-rail-section filesystem-rail-section-tree",
            "aria-labelledby": "filesystem-rail-tree",
          },
          createElement(
            "h2",
            { id: "filesystem-rail-tree", className: "filesystem-rail-section-label" },
            "文件夹",
          ),
          createElement(DirectoryTree, {
            workspaceId,
            organizationId: namespace!.organization_id,
            namespaceId: namespace!.id,
            rootNode: root,
            space: activeSpace,
            currentId: currentId ?? root.id,
            onNavigate,
            onDropNodes,
            canWrite,
          }),
        )
      : null,
  );
}
