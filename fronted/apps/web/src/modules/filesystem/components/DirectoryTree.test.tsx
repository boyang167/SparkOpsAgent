// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import { BffError } from "@sparkops/contracts";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DirectoryTree, type DirectoryTreeProps } from "./DirectoryTree";
import {
  clearNodeDragSession,
  getActiveNodeDragPayload,
  writeNodeDragPayload,
} from "./directory-dnd";

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
const secondDirectory: FileNodeVM = {
  ...root,
  id: "directory-3",
  parent_id: root.id,
  name: "团队资料",
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function itemFor(name: string) {
  const item = screen.getByText(name).closest('[role="treeitem"]');
  if (!(item instanceof HTMLElement)) throw new Error(`treeitem not found: ${name}`);
  return item;
}

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
  });

  afterEach(() => {
    clearNodeDragSession();
    cleanup();
  });

  it("loads every page of directory children when a node expands", async () => {
    const user = userEvent.setup();
    sdk.listFileNodes
      .mockResolvedValueOnce({ items: [directory], next_cursor: "cursor-1" })
      .mockResolvedValueOnce({ items: [secondDirectory], next_cursor: "" });
    render(createElement(DirectoryTree, props()));

    expect(screen.getByRole("tree")).toBeInTheDocument();
    expect(sdk.listFileNodes).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "展开 全部文件" }));

    await waitFor(() => expect(screen.getByText("团队资料")).toBeInTheDocument());
    expect(sdk.listFileNodes).toHaveBeenNthCalledWith(1, workspaceId, {
      parentId: root.id,
      nodeType: "DIRECTORY",
    });
    expect(sdk.listFileNodes).toHaveBeenNthCalledWith(2, workspaceId, {
      parentId: root.id,
      nodeType: "DIRECTORY",
      cursor: "cursor-1",
    });
  });

  it("loads a nested directory only when it expands", async () => {
    const user = userEvent.setup();
    sdk.listFileNodes
      .mockResolvedValueOnce({ items: [directory], next_cursor: "" })
      .mockResolvedValueOnce({ items: [nestedDirectory], next_cursor: "" });
    render(createElement(DirectoryTree, props()));

    await user.click(screen.getByRole("button", { name: "展开 全部文件" }));
    await user.click(await screen.findByRole("button", { name: "展开 项目资料" }));

    await waitFor(() => {
      expect(sdk.listFileNodes).toHaveBeenLastCalledWith(workspaceId, {
        parentId: "directory-1",
        nodeType: "DIRECTORY",
      });
    });
    expect(screen.getByText("设计稿")).toBeInTheDocument();
  });

  it("implements roving focus and tree arrow-key navigation", async () => {
    const user = userEvent.setup();
    const onNavigate = vi.fn();
    sdk.listFileNodes.mockResolvedValueOnce({
      items: [directory, secondDirectory],
      next_cursor: "",
    });
    render(createElement(DirectoryTree, props({ onNavigate })));
    const rootItem = screen.getByRole("treeitem", { name: /全部文件/ });

    rootItem.focus();
    await user.keyboard("{ArrowRight}");
    expect(rootItem).toHaveAttribute("aria-expanded", "true");
    await screen.findByText("项目资料");
    expect(rootItem).toHaveAttribute("tabindex", "0");
    expect(itemFor("项目资料")).toHaveAttribute("tabindex", "-1");
    expect(screen.getByRole("button", { name: "收起 全部文件" })).toHaveAttribute(
      "tabindex",
      "-1",
    );

    await user.keyboard("{ArrowRight}");
    expect(itemFor("项目资料")).toHaveFocus();
    expect(itemFor("项目资料")).toHaveAttribute("tabindex", "0");
    expect(rootItem).toHaveAttribute("tabindex", "-1");

    await user.keyboard("{ArrowDown}");
    expect(itemFor("团队资料")).toHaveFocus();
    await user.keyboard("{ArrowUp}");
    expect(itemFor("项目资料")).toHaveFocus();

    await user.keyboard("{ArrowLeft}");
    expect(rootItem).toHaveFocus();

    await user.keyboard(" ");
    expect(onNavigate).toHaveBeenCalledWith(root);
    await user.keyboard("{Enter}");
    expect(onNavigate).toHaveBeenCalledTimes(2);
  });

  it("does not reopen a node when loading finishes after collapse", async () => {
    const user = userEvent.setup();
    const page = deferred<{ items: FileNodeVM[]; next_cursor: string }>();
    sdk.listFileNodes.mockReturnValueOnce(page.promise);
    render(createElement(DirectoryTree, props()));
    const rootItem = screen.getByRole("treeitem", { name: /全部文件/ });

    rootItem.focus();
    await user.keyboard("{ArrowRight}");
    await user.keyboard("{ArrowLeft}");
    page.resolve({ items: [directory], next_cursor: "" });

    await waitFor(() => expect(screen.queryByText("加载中…")).not.toBeInTheDocument());
    expect(rootItem).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("项目资料")).not.toBeInTheDocument();
  });

  it("shows an explicit empty state for an empty directory", async () => {
    const user = userEvent.setup();
    sdk.listFileNodes.mockResolvedValueOnce({ items: [], next_cursor: "" });
    render(createElement(DirectoryTree, props()));

    await user.click(screen.getByRole("button", { name: "展开 全部文件" }));

    expect(await screen.findByText("空目录")).toBeInTheDocument();
  });

  it("distinguishes forbidden directory loading errors", async () => {
    const user = userEvent.setup();
    sdk.listFileNodes.mockRejectedValueOnce(
      new BffError("forbidden", "FILESYSTEM_RESOURCE_FORBIDDEN", 403),
    );
    render(createElement(DirectoryTree, props()));

    await user.click(screen.getByRole("button", { name: "展开 全部文件" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("无权访问此目录");
  });

  it("activates only the clicked nested treeitem", async () => {
    const user = userEvent.setup();
    const onNavigate = vi.fn();
    sdk.listFileNodes.mockResolvedValueOnce({ items: [directory], next_cursor: "" });
    render(createElement(DirectoryTree, props({ onNavigate })));

    await user.click(screen.getByRole("button", { name: "展开 全部文件" }));
    await user.click(await screen.findByText("项目资料"));

    expect(onNavigate).toHaveBeenCalledTimes(1);
    expect(onNavigate).toHaveBeenCalledWith(directory);
  });

  it("returns roving focus to a node when it hides the focused descendant", async () => {
    const user = userEvent.setup();
    sdk.listFileNodes.mockResolvedValueOnce({ items: [directory], next_cursor: "" });
    render(createElement(DirectoryTree, props()));
    const rootItem = screen.getByRole("treeitem", { name: "全部文件" });

    rootItem.focus();
    await user.keyboard("{ArrowRight}");
    await screen.findByText("项目资料");
    await user.keyboard("{ArrowRight}");
    expect(itemFor("项目资料")).toHaveFocus();

    await user.click(screen.getByRole("button", { name: "收起 全部文件" }));

    expect(rootItem).toHaveFocus();
    expect(rootItem).toHaveAttribute("tabindex", "0");
  });

  it("does not steal focus when the tree mounts", () => {
    const outside = document.createElement("button");
    document.body.append(outside);
    outside.focus();

    render(createElement(DirectoryTree, props()));

    expect(outside).toHaveFocus();
    outside.remove();
  });

  it("expands a valid drag target after 700ms and drops the dragged nodes", async () => {
    vi.useFakeTimers();
    const onDropNodes = vi.fn();
    sdk.listFileNodes.mockResolvedValueOnce({ items: [directory], next_cursor: "" });
    render(createElement(DirectoryTree, props({ onDropNodes, canWrite: true })));
    const rootItem = screen.getByRole("treeitem", { name: "全部文件" });
    const dataTransfer = {
      getData: vi.fn().mockReturnValue(
        JSON.stringify({
          workspaceId,
          nodes: [{ id: "file-1", parentId: "source-1", type: "FILE", state: "ACTIVE" }],
        }),
      ),
      dropEffect: "none",
    };

    fireEvent.dragEnter(rootItem, { dataTransfer });
    await act(() => vi.advanceTimersByTimeAsync(699));
    expect(sdk.listFileNodes).not.toHaveBeenCalled();
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(sdk.listFileNodes).toHaveBeenCalledOnce();

    fireEvent.drop(rootItem, { dataTransfer });
    expect(onDropNodes).toHaveBeenCalledWith(["file-1"], root);
    vi.useRealTimers();
  });

  it("shows a rejected target and never drops a node onto itself", () => {
    const onDropNodes = vi.fn();
    render(createElement(DirectoryTree, props({ onDropNodes, canWrite: true })));
    const rootItem = screen.getByRole("treeitem", { name: "全部文件" });
    const dataTransfer = {
      getData: vi.fn().mockReturnValue(
        JSON.stringify({
          workspaceId,
          nodes: [{ id: root.id, parentId: null, type: "DIRECTORY", state: "ACTIVE" }],
        }),
      ),
      dropEffect: "none",
    };

    fireEvent.dragEnter(rootItem, { dataTransfer });
    expect(rootItem).toHaveAttribute("data-drop-state", "rejected");
    expect(screen.getByText("不能移动到自身")).toBeInTheDocument();
    fireEvent.drop(rootItem, { dataTransfer });
    expect(onDropNodes).not.toHaveBeenCalled();
  });

  it("clears the delayed expansion timer when unmounted", () => {
    vi.useFakeTimers();
    const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");
    const { unmount } = render(createElement(DirectoryTree, props({ canWrite: true })));
    const rootItem = screen.getByRole("treeitem", { name: "全部文件" });
    fireEvent.dragEnter(rootItem, {
      dataTransfer: {
        getData: vi.fn().mockReturnValue(
          JSON.stringify({
            workspaceId,
            nodes: [{ id: "file-1", type: "FILE", state: "ACTIVE" }],
          }),
        ),
      },
    });

    unmount();

    expect(clearTimeoutSpy).toHaveBeenCalled();
    vi.useRealTimers();
  });

  it("cancels delayed expansion on dragleave and drop", async () => {
    vi.useFakeTimers();
    const onDropNodes = vi.fn();
    render(createElement(DirectoryTree, props({ canWrite: true, onDropNodes })));
    const rootItem = screen.getByRole("treeitem", { name: "全部文件" });
    const dataTransfer = {
      getData: vi.fn().mockReturnValue(
        JSON.stringify({
          workspaceId,
          nodes: [{ id: "file-1", type: "FILE", state: "ACTIVE" }],
        }),
      ),
      dropEffect: "none",
    };

    fireEvent.dragEnter(rootItem, { dataTransfer });
    fireEvent.dragLeave(rootItem, { dataTransfer });
    await act(() => vi.advanceTimersByTimeAsync(700));
    expect(sdk.listFileNodes).not.toHaveBeenCalled();

    fireEvent.dragEnter(rootItem, { dataTransfer });
    fireEvent.drop(rootItem, { dataTransfer });
    await act(() => vi.advanceTimersByTimeAsync(700));
    expect(sdk.listFileNodes).not.toHaveBeenCalled();
    expect(onDropNodes).toHaveBeenCalledWith(["file-1"], root);
    vi.useRealTimers();
  });

  it("uses the internal drag session when DataTransfer is protected", async () => {
    vi.useFakeTimers();
    const onDropNodes = vi.fn();
    sdk.listFileNodes.mockResolvedValueOnce({ items: [], next_cursor: "" });
    render(createElement(DirectoryTree, props({ canWrite: true, onDropNodes })));
    const draggedNode: FileNodeVM = {
      ...root,
      id: "file-protected",
      parent_id: "source-1",
      type: "FILE",
      name: "受保护数据.txt",
    };
    writeNodeDragPayload(
      { setData: vi.fn(), effectAllowed: "none" } as unknown as DataTransfer,
      workspaceId,
      [draggedNode],
    );
    const protectedTransfer = {
      getData: vi.fn(() => {
        throw new Error("protected mode");
      }),
      dropEffect: "none",
    };
    const rootItem = screen.getByRole("treeitem", { name: "全部文件" });

    fireEvent.dragEnter(rootItem, { dataTransfer: protectedTransfer });
    expect(rootItem).toHaveAttribute("data-drop-state", "allowed");
    fireEvent.dragOver(rootItem, { dataTransfer: protectedTransfer });
    expect(protectedTransfer.dropEffect).toBe("move");
    expect(protectedTransfer.getData).not.toHaveBeenCalled();
    await act(() => vi.advanceTimersByTimeAsync(700));
    expect(sdk.listFileNodes).toHaveBeenCalledOnce();

    fireEvent.drop(rootItem, { dataTransfer: protectedTransfer });
    expect(onDropNodes).toHaveBeenCalledWith(["file-protected"], root);
    expect(getActiveNodeDragPayload()).toBeNull();
    vi.useRealTimers();
  });

  it("does not expand after dragend clears the internal session", async () => {
    vi.useFakeTimers();
    render(createElement(DirectoryTree, props({ canWrite: true })));
    writeNodeDragPayload(
      { setData: vi.fn(), effectAllowed: "none" } as unknown as DataTransfer,
      workspaceId,
      [
        {
          ...root,
          id: "file-ended",
          parent_id: "source-1",
          type: "FILE",
          name: "已结束拖拽.txt",
        },
      ],
    );
    const rootItem = screen.getByRole("treeitem", { name: "全部文件" });
    fireEvent.dragEnter(rootItem, {
      dataTransfer: {
        getData: vi.fn(() => {
          throw new Error("protected mode");
        }),
      },
    });

    clearNodeDragSession();
    await act(() => vi.advanceTimersByTimeAsync(700));

    expect(sdk.listFileNodes).not.toHaveBeenCalled();
    vi.useRealTimers();
  });
});
