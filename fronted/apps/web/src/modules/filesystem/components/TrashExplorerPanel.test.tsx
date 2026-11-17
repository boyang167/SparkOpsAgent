// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import type { TrashItemVM } from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TrashExplorerPanel } from "./TrashExplorerPanel";

const refresh = vi.hoisted(() => vi.fn());
const listTrashItems = vi.hoisted(() => vi.fn());
const restoreFileNode = vi.hoisted(() => vi.fn());
const purgeFileNode = vi.hoisted(() => vi.fn());
const emptyTrash = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh }),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  listTrashItems,
  restoreFileNode,
  purgeFileNode,
  emptyTrash,
}));

vi.mock("@/components/ui/InlineAlert", () => ({
  InlineAlert: ({ children }: { children: React.ReactNode }) =>
    createElement("div", { role: "alert" }, children),
}));

vi.mock("@/components/ui/Modal", () => ({
  Modal: ({
    open,
    title,
    description,
    footer,
    children,
  }: {
    open: boolean;
    title: string;
    description?: string;
    footer?: React.ReactNode;
    children?: React.ReactNode;
  }) =>
    open
      ? createElement(
          "div",
          { role: "dialog", "aria-label": title },
          description ? createElement("p", null, description) : null,
          children,
          footer,
        )
      : null,
}));

function trashItem(id: string, name: string): TrashItemVM {
  return {
    node: {
      id,
      namespace_id: "00000000-0000-4000-8000-000000000099",
      parent_id: "00000000-0000-4000-8000-000000000098",
      type: "FILE",
      name,
      current_revision_id: null,
      state: "TRASHED",
      owner_id: "00000000-0000-4000-8000-000000000097",
    },
    original_path: `/项目/${name}`,
    deleted_at: "2026-01-15T08:00:00.000Z",
    can_restore: true,
    can_permanently_delete: true,
  };
}

describe("TrashExplorerPanel", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows the empty state", () => {
    render(
      createElement(TrashExplorerPanel, {
        workspaceId: "workspace-1",
        namespaceId: "00000000-0000-4000-8000-000000000099",
        initialItems: [],
      }),
    );
    expect(screen.getByText("回收站为空。")).toBeInTheDocument();
  });

  it("retries after a load error", async () => {
    const user = userEvent.setup();
    listTrashItems.mockResolvedValue({
      items: [trashItem("00000000-0000-4000-8000-000000000011", "a.txt")],
      next_cursor: "",
    });

    render(
      createElement(TrashExplorerPanel, {
        workspaceId: "workspace-1",
        namespaceId: "00000000-0000-4000-8000-000000000099",
        initialItems: [],
        initialLoadError: true,
      }),
    );

    await user.click(screen.getByRole("button", { name: "重试" }));
    expect(listTrashItems).toHaveBeenCalledWith("workspace-1", {
      namespace_id: "00000000-0000-4000-8000-000000000099",
    });
    expect(screen.getByText("a.txt")).toBeInTheDocument();
  });

  it("restores a single item immediately", async () => {
    const user = userEvent.setup();
    restoreFileNode.mockResolvedValue(undefined);

    render(
      createElement(TrashExplorerPanel, {
        workspaceId: "workspace-1",
        namespaceId: "00000000-0000-4000-8000-000000000099",
        initialItems: [trashItem("00000000-0000-4000-8000-000000000011", "a.txt")],
      }),
    );

    await user.click(screen.getByRole("button", { name: "恢复" }));
    expect(restoreFileNode).toHaveBeenCalledWith(
      "workspace-1",
      "00000000-0000-4000-8000-000000000011",
      { namespaceId: "00000000-0000-4000-8000-000000000099" },
    );
    expect(refresh).toHaveBeenCalled();
    expect(screen.queryByText("a.txt")).not.toBeInTheDocument();
  });

  it("shows restore conflict copy for a failed batch item", async () => {
    const user = userEvent.setup();
    restoreFileNode
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(
        new BffError("conflict", "FILESYSTEM_RESTORE_NAME_CONFLICT", 409),
      );

    render(
      createElement(TrashExplorerPanel, {
        workspaceId: "workspace-1",
        namespaceId: "00000000-0000-4000-8000-000000000099",
        initialItems: [
          trashItem("00000000-0000-4000-8000-000000000011", "a.txt"),
          trashItem("00000000-0000-4000-8000-000000000012", "b.txt"),
        ],
      }),
    );

    await user.click(screen.getByRole("checkbox", { name: "选择 a.txt" }));
    await user.click(screen.getByRole("checkbox", { name: "选择 b.txt" }));
    await user.click(
      within(screen.getByRole("toolbar", { name: "回收站所选操作" })).getByRole("button", {
        name: "恢复",
      }),
    );
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "恢复" }));

    expect(screen.getByRole("alert")).toHaveTextContent("原目录已有同名项目");
    expect(screen.getByText("b.txt")).toBeInTheDocument();
    expect(screen.queryByText("a.txt")).not.toBeInTheDocument();
  });

  it("requires confirmation before permanent delete", async () => {
    const user = userEvent.setup();
    purgeFileNode.mockResolvedValue(undefined);

    render(
      createElement(TrashExplorerPanel, {
        workspaceId: "workspace-1",
        namespaceId: "00000000-0000-4000-8000-000000000099",
        initialItems: [trashItem("00000000-0000-4000-8000-000000000011", "a.txt")],
      }),
    );

    await user.click(screen.getByRole("button", { name: "彻底删除" }));
    expect(purgeFileNode).not.toHaveBeenCalled();
    const dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "彻底删除" }));
    expect(purgeFileNode).toHaveBeenCalled();
  });

  it("requires confirmation before emptying trash", async () => {
    const user = userEvent.setup();
    emptyTrash.mockResolvedValue({ items: [{ node_id: "00000000-0000-4000-8000-000000000011", succeeded: true }] });

    render(
      createElement(TrashExplorerPanel, {
        workspaceId: "workspace-1",
        namespaceId: "00000000-0000-4000-8000-000000000099",
        initialItems: [trashItem("00000000-0000-4000-8000-000000000011", "a.txt")],
      }),
    );

    await user.click(screen.getByRole("button", { name: "清空回收站" }));
    expect(emptyTrash).not.toHaveBeenCalled();
    const dialog = screen.getByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "清空回收站" }));
    expect(emptyTrash).toHaveBeenCalledWith("workspace-1", {
      namespaceId: "00000000-0000-4000-8000-000000000099",
    });
  });

  it("shows trash metadata and disables browse-only affordances", () => {
    render(
      createElement(TrashExplorerPanel, {
        workspaceId: "workspace-1",
        namespaceId: "00000000-0000-4000-8000-000000000099",
        initialItems: [trashItem("00000000-0000-4000-8000-000000000011", "a.txt")],
      }),
    );

    expect(screen.getByText("/项目/a.txt")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "打开 a.txt" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^重命名 / })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^移动到… / })).not.toBeInTheDocument();
  });
});