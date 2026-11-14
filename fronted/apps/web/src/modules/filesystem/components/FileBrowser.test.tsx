// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FileBrowser, type FileBrowserProps } from "./FileBrowser";
import { clearNodeDragSession, getActiveNodeDragPayload } from "./directory-dnd";

const nodes: FileNodeVM[] = [
  {
    id: "00000000-0000-4000-8000-000000000011",
    namespace_id: "00000000-0000-4000-8000-000000000001",
    parent_id: "00000000-0000-4000-8000-000000000002",
    type: "FILE",
    name: "规格说明.md",
    current_revision_id: null,
    state: "ACTIVE",
    owner_id: "00000000-0000-4000-8000-000000000003",
  },
  {
    id: "00000000-0000-4000-8000-000000000012",
    namespace_id: "00000000-0000-4000-8000-000000000001",
    parent_id: "00000000-0000-4000-8000-000000000002",
    type: "DIRECTORY",
    name: "资料",
    current_revision_id: null,
    state: "ACTIVE",
    owner_id: "00000000-0000-4000-8000-000000000003",
  },
  {
    id: "00000000-0000-4000-8000-000000000013",
    namespace_id: "00000000-0000-4000-8000-000000000001",
    parent_id: "00000000-0000-4000-8000-000000000002",
    type: "FILE",
    name: "阿尔法.txt",
    current_revision_id: null,
    state: "ACTIVE",
    owner_id: "00000000-0000-4000-8000-000000000003",
  },
];

function createProps(): FileBrowserProps {
  return {
    nodes,
    view: "list",
    selectedIds: [],
    onSelectionChange: vi.fn(),
    onOpen: vi.fn(),
    onRename: vi.fn(),
    onMove: vi.fn(),
  };
}

describe("FileBrowser", () => {
  afterEach(() => {
    clearNodeDragSession();
    cleanup();
  });

  it("preserves selection when switching from list to grid", async () => {
    const user = userEvent.setup();
    const props = createProps();
    const { rerender } = render(createElement(FileBrowser, { ...props, view: "list" }));

    await user.click(screen.getByRole("checkbox", { name: "选择 规格说明.md" }));
    rerender(createElement(FileBrowser, { ...props, view: "grid" }));

    expect(screen.getByRole("checkbox", { name: "选择 规格说明.md" })).toBeChecked();
    expect(props.onSelectionChange).toHaveBeenCalledWith([nodes[0].id]);
  });

  it("reflects selection updates supplied by its owner", () => {
    const props = createProps();
    const { rerender } = render(createElement(FileBrowser, props));

    rerender(createElement(FileBrowser, { ...props, selectedIds: [nodes[1].id] }));

    expect(screen.getByRole("checkbox", { name: "选择 资料" })).toBeChecked();
  });

  it("sorts directories first and names in Chinese ascending order", () => {
    render(createElement(FileBrowser, { ...createProps(), view: "list" }));

    const body = screen.getAllByRole("rowgroup")[1];
    const names = within(body)
      .getAllByRole("button", { name: /^打开 / })
      .map((button) => button.textContent);

    expect(names).toEqual(["资料", "阿尔法.txt", "规格说明.md"]);
  });

  it("routes grid actions through the supplied callbacks", async () => {
    const user = userEvent.setup();
    const props = createProps();
    render(createElement(FileBrowser, { ...props, view: "grid" }));

    await user.click(screen.getByRole("button", { name: "打开 资料" }));
    await user.click(screen.getByRole("button", { name: "重命名 资料" }));
    await user.click(screen.getByRole("button", { name: "移动到… 资料" }));

    expect(props.onOpen).toHaveBeenCalledWith(nodes[1]);
    expect(props.onRename).toHaveBeenCalledWith(nodes[1]);
    expect(props.onMove).toHaveBeenCalledWith([nodes[1]]);
  });

  it("hides move actions when the compatibility adapter disables the capability", () => {
    const props = createProps();
    render(createElement(FileBrowser, { ...props, canMove: false }));

    expect(screen.queryByRole("button", { name: /^移动到… / })).not.toBeInTheDocument();
  });

  it("offers one keyboard-accessible move action for the current selection", async () => {
    const user = userEvent.setup();
    const props = createProps();
    render(createElement(FileBrowser, props));

    await user.click(screen.getByRole("checkbox", { name: "选择 规格说明.md" }));
    await user.click(screen.getByRole("checkbox", { name: "选择 资料" }));
    await user.click(screen.getByRole("button", { name: "移动所选 2 项到…" }));

    expect(props.onMove).toHaveBeenCalledWith([nodes[0], nodes[1]]);
  });

  it("writes the selected active nodes to the internal drag payload", async () => {
    const user = userEvent.setup();
    const props = { ...createProps(), workspaceId: "workspace-1" };
    render(createElement(FileBrowser, props));
    await user.click(screen.getByRole("checkbox", { name: "选择 规格说明.md" }));
    await user.click(screen.getByRole("checkbox", { name: "选择 资料" }));
    const dataTransfer = { setData: vi.fn(), effectAllowed: "none" };

    fireEvent.dragStart(screen.getByRole("row", { name: /规格说明\.md/ }), { dataTransfer });

    expect(dataTransfer.setData).toHaveBeenCalledWith(
      "application/x-sparkops-file-nodes",
      expect.stringContaining('"workspaceId":"workspace-1"'),
    );
    expect(dataTransfer.setData.mock.calls[0]?.[1]).toContain(nodes[1].id);
  });

  it("converges selection to active nodes that still exist after refresh", async () => {
    const onSelectionChange = vi.fn();
    const onMove = vi.fn();
    const { rerender } = render(
      createElement(FileBrowser, {
        ...createProps(),
        selectedIds: [nodes[0].id, nodes[1].id],
        onSelectionChange,
        onMove,
      }),
    );

    rerender(
      createElement(FileBrowser, {
        ...createProps(),
        nodes: [{ ...nodes[0] }, { ...nodes[1], state: "TRASHED" }],
        selectedIds: [nodes[0].id, nodes[1].id],
        onSelectionChange,
        onMove,
      }),
    );

    expect(await screen.findByText("已选择 1 项")).toBeInTheDocument();
    expect(onSelectionChange).toHaveBeenLastCalledWith([nodes[0].id]);
    await userEvent.click(screen.getByRole("button", { name: "移动所选 1 项到…" }));
    expect(onMove).toHaveBeenCalledWith([{ ...nodes[0] }]);
  });

  it("clears the internal drag session on dragend", () => {
    render(
      createElement(FileBrowser, {
        ...createProps(),
        workspaceId: "workspace-1",
      }),
    );
    const row = screen.getByRole("row", { name: /规格说明\.md/ });
    const dataTransfer = { setData: vi.fn(), effectAllowed: "none" };

    fireEvent.dragStart(row, { dataTransfer });
    expect(getActiveNodeDragPayload()?.nodes[0]?.id).toBe(nodes[0].id);

    fireEvent.dragEnd(row, { dataTransfer });
    expect(getActiveNodeDragPayload()).toBeNull();
  });
});
