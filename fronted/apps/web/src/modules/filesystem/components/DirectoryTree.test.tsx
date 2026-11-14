// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DirectoryTree, type DirectoryTreeProps } from "./DirectoryTree";

const sdk = vi.hoisted(() => ({
  listFileNodes: vi.fn(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  listFileNodes: sdk.listFileNodes,
}));

const workspaceId = "00000000-0000-4000-8000-000000000001";
const root: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000002",
  namespace_id: "00000000-0000-4000-8000-000000000003",
  parent_id: null,
  type: "DIRECTORY",
  name: "全部文件",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000004",
};
const directory: FileNodeVM = {
  ...root,
  id: "directory-1",
  parent_id: root.id,
  name: "项目资料",
};
const nestedDirectory: FileNodeVM = {
  ...root,
  id: "directory-2",
  parent_id: directory.id,
  name: "设计稿",
};

function props(overrides: Partial<DirectoryTreeProps> = {}): DirectoryTreeProps {
  return {
    workspaceId,
    root,
    currentId: root.id,
    onNavigate: vi.fn(),
    onDropNodes: vi.fn(),
    ...overrides,
  };
}

describe("DirectoryTree", () => {
  beforeEach(() => {
    sdk.listFileNodes.mockReset();
    sdk.listFileNodes
      .mockResolvedValueOnce({ items: [directory], next_cursor: "" })
      .mockResolvedValueOnce({ items: [nestedDirectory], next_cursor: "" });
  });

  afterEach(cleanup);

  it("loads directory children only when their parent expands", async () => {
    const user = userEvent.setup();
    render(createElement(DirectoryTree, props()));

    expect(screen.getByRole("tree")).toBeInTheDocument();
    expect(sdk.listFileNodes).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "展开 全部文件" }));
    await user.click(await screen.findByRole("button", { name: "展开 项目资料" }));

    await waitFor(() =>
      expect(sdk.listFileNodes).toHaveBeenLastCalledWith(workspaceId, {
        parentId: "directory-1",
        nodeType: "DIRECTORY",
      }),
    );
    expect(screen.getByRole("button", { name: "设计稿" })).toBeInTheDocument();
  });

  it("supports ArrowRight, ArrowLeft, and Enter", async () => {
    const user = userEvent.setup();
    const onNavigate = vi.fn();
    render(createElement(DirectoryTree, props({ onNavigate })));
    const rootItem = screen.getByRole("treeitem", { name: /全部文件/ });

    rootItem.focus();
    await user.keyboard("{ArrowRight}");
    expect(rootItem).toHaveAttribute("aria-expanded", "true");
    expect(await screen.findByRole("button", { name: "项目资料" })).toBeInTheDocument();

    await user.keyboard("{ArrowLeft}");
    expect(rootItem).toHaveAttribute("aria-expanded", "false");

    await user.keyboard("{Enter}");
    expect(onNavigate).toHaveBeenCalledWith(root);
  });
});
