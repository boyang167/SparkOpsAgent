"use client";

import { createElement } from "react";
import type { NamespaceSummaryVM, FileNodeVM } from "@sparkops/contracts/filesystem";
import { DirectoryTree } from "./DirectoryTree";
import type { FileSpace, FileView } from "./workbench-state";

const spaces: Array<{ id: FileSpace; label: string }> = [
  { id: "my", label: "我的文件" },
  { id: "workspace", label: "当前工作区" },
  { id: "organization", label: "租户文件" },
  { id: "shared", label: "与我共享" },
];

function hrefFor(space: FileSpace, view: FileView) {
  const params = new URLSearchParams();
  if (view !== "list") params.set("view", view);
  if (space !== "workspace") params.set("space", space);
  const query = params.toString();
  return query ? `/files?${query}` : "/files";
}

export function FileSpaceNav({
  activeSpace,
  organizationLabel,
  workspaceLabel,
  namespace,
  workspaceId,
  view,
  currentId,
  onNavigate = () => undefined,
  onDropNodes = () => undefined,
}: {
  activeSpace: FileSpace;
  organizationLabel: string;
  workspaceLabel: string;
  namespace?: NamespaceSummaryVM;
  workspaceId: string;
  view: FileView;
  currentId?: string;
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
    { className: "stack-md min-w-0" },
    createElement(
      "nav",
      { "aria-label": "文件空间", className: "flex min-w-0 flex-wrap gap-2" },
      spaces.map((space) =>
        createElement(
          "a",
          {
            key: space.id,
            href: hrefFor(space.id, view),
            "aria-current": activeSpace === space.id ? "page" : undefined,
            className:
              activeSpace === space.id
                ? "btn btn-secondary filesystem-touch-target"
                : "btn btn-ghost filesystem-touch-target",
          },
          space.label,
        ),
      ),
    ),
    createElement(
      "div",
      { className: "stack-xs text-sm text-muted" },
      createElement("span", null, organizationLabel),
      createElement("span", null, workspaceLabel),
    ),
    activeSpace !== "shared" && root
      ? createElement(DirectoryTree, {
          workspaceId,
          organizationId: namespace!.organization_id,
          namespaceId: namespace!.id,
          root,
          currentId: currentId ?? root.id,
          onNavigate,
          onDropNodes,
          canWrite: namespace!.capabilities.includes("WRITE"),
        })
      : null,
  );
}
