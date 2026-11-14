// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MoveNodesModal } from "./MoveNodesModal";

const sdk = vi.hoisted(() => ({ moveFileNode: vi.fn() }));
vi.mock("@sparkops/web-sdk/filesystem", () => ({ moveFileNode: sdk.moveFileNode }));

const tree = vi.hoisted(() => ({
  ancestorIds: new Set<string>(),
}));

vi.mock("@/components/ui/InlineAlert", () => ({
  InlineAlert: ({ children }: { children: React.ReactNode }) =>
    createElement("div", null, children),
}));

vi.mock("@/components/ui/Modal", () => ({
  Modal: ({
    open,
    title,
    children,
    footer,
  }: {
    open: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    open
      ? createElement(
          "div",
          { role: "dialog", "aria-label": title },
          children,
          footer,
        )
      : null,
}));

vi.mock("./DirectoryTree", () => ({
  DirectoryTree: ({
    onNavigate,
    onSelectionAncestorsChange,
    selectionOnly,
  }: {
    onNavigate: (node: FileNodeVM) => void;
    onSelectionAncestorsChange?: (ancestorIds: ReadonlySet<string>) => void;
    selectionOnly?: boolean;
  }) =>
    createElement(
      "button",
      {
        type: "button",
        onClick: () => {
          onSelectionAncestorsChange?.(tree.ancestorIds);
          onNavigate(target);
        },
      },
      selectionOnly ? "选择目标目录" : "普通目录树",
    ),
}));

const workspaceId = "workspace-1";
const root: FileNodeVM = {
  id: "root",
  namespace_id: "namespace-1",
  parent_id: null,
  type: "DIRECTORY",
  name: "全部文件",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "owner-1",
};
const target: FileNodeVM = { ...root, id: "target", parent_id: root.id, name: "归档" };
const nodes: FileNodeVM[] = [
  { ...root, id: "file-a", parent_id: "source-a", type: "FILE", name: "A.txt" },
  { ...root, id: "file-b", parent_id: "source-b", type: "FILE", name: "B.txt" },
];

describe("MoveNodesModal", () => {
  beforeEach(() => {
    sdk.moveFileNode.mockReset();
    tree.ancestorIds = new Set();
  });
  afterEach(cleanup);

  it("reuses the directory tree in selection-only mode and moves every node", async () => {
    const user = userEvent.setup();
    const onMoved = vi.fn();
    sdk.moveFileNode.mockResolvedValue(undefined);
    render(
      createElement(MoveNodesModal, {
        open: true,
        workspaceId,
        root,
        nodes,
        onClose: vi.fn(),
        onMoved,
        onRefreshDirectories: vi.fn(),
      }),
    );

    await user.click(screen.getByRole("button", { name: "选择目标目录" }));
    await user.click(screen.getByRole("button", { name: "移动到此处" }));

    await waitFor(() => expect(onMoved).toHaveBeenCalledWith(nodes, target));
    expect(sdk.moveFileNode).toHaveBeenNthCalledWith(
      1,
      workspaceId,
      "file-a",
      "target",
      "A.txt",
    );
    expect(sdk.moveFileNode).toHaveBeenNthCalledWith(
      2,
      workspaceId,
      "file-b",
      "target",
      "B.txt",
    );
  });

  it("refreshes every source and the target after failure without reporting success", async () => {
    const user = userEvent.setup();
    const onMoved = vi.fn();
    const onRefreshDirectories = vi.fn();
    sdk.moveFileNode.mockResolvedValueOnce(undefined).mockRejectedValueOnce(new Error("conflict"));
    render(
      createElement(MoveNodesModal, {
        open: true,
        workspaceId,
        root,
        nodes,
        onClose: vi.fn(),
        onMoved,
        onRefreshDirectories,
      }),
    );

    await user.click(screen.getByRole("button", { name: "选择目标目录" }));
    await user.click(screen.getByRole("button", { name: "移动到此处" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("移动失败");
    expect(onRefreshDirectories).toHaveBeenCalledWith(["source-a", "source-b", "target"]);
    expect(onMoved).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "移动到…" })).toBeInTheDocument();
  });

  it("rejects moving a directory to any depth of its own descendants", async () => {
    const user = userEvent.setup();
    const directory = {
      ...root,
      id: "directory-a",
      parent_id: root.id,
      name: "项目",
    };
    tree.ancestorIds = new Set([root.id, directory.id, "nested-parent"]);
    render(
      createElement(MoveNodesModal, {
        open: true,
        workspaceId,
        root,
        nodes: [directory],
        onClose: vi.fn(),
        onMoved: vi.fn(),
        onRefreshDirectories: vi.fn(),
      }),
    );

    await user.click(screen.getByRole("button", { name: "选择目标目录" }));

    expect(screen.getByRole("alert")).toHaveTextContent("不能移动到子目录");
    expect(screen.getByRole("button", { name: "移动到此处" })).toBeDisabled();
    expect(sdk.moveFileNode).not.toHaveBeenCalled();
  });
});
