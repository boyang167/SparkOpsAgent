// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement, useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import { FileDetailDrawer } from "./FileDetailDrawer";

const sdk = vi.hoisted(() => ({
  listFileRevisions: vi.fn(),
  listFileAuditEvents: vi.fn(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  listFileRevisions: sdk.listFileRevisions,
  listFileAuditEvents: sdk.listFileAuditEvents,
}));

vi.mock("@/components/ui/Drawer", () => ({
  Drawer: ({
    open,
    title,
    tabs,
    children,
    onClose,
  }: {
    open: boolean;
    title: string;
    tabs?: React.ReactNode;
    children: React.ReactNode;
    onClose: () => void;
  }) => {
    useEffect(() => {
      if (!open) return;
      const listener = (event: KeyboardEvent) => {
        if (event.key === "Escape") onClose();
      };
      document.addEventListener("keydown", listener);
      return () => document.removeEventListener("keydown", listener);
    }, [onClose, open]);
    return open
      ? createElement("div", { role: "dialog", "aria-label": title }, tabs, children)
      : null;
  },
}));
vi.mock("@/components/ui/InlineAlert", () => ({
  InlineAlert: ({ children }: { children: React.ReactNode }) =>
    createElement("div", null, children),
}));

const file: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000011",
  namespace_id: "00000000-0000-4000-8000-000000000002",
  parent_id: "00000000-0000-4000-8000-000000000003",
  type: "FILE",
  name: "规格说明.md",
  current_revision_id: "00000000-0000-4000-8000-000000000012",
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000013",
};

function drawer(
  open = true,
  onClose = vi.fn(),
  node: FileNodeVM = file,
  workspaceId = "00000000-0000-4000-8000-000000000001",
) {
  return createElement(FileDetailDrawer, {
    open,
    node,
    workspaceId,
    onClose,
  });
}

describe("FileDetailDrawer", () => {
  beforeEach(() => {
    sdk.listFileRevisions.mockReset();
    sdk.listFileAuditEvents.mockReset();
    sdk.listFileRevisions.mockResolvedValue({
      items: [
        {
          id: file.current_revision_id,
          node_id: file.id,
          revision_no: 2,
          size: 42,
          mime_type: "text/markdown",
          created_by: file.owner_id,
        },
      ],
      next_cursor: "",
    });
    sdk.listFileAuditEvents.mockResolvedValue({ items: [], next_cursor: "" });
  });

  afterEach(() => cleanup());

  it("loads real revisions when opened and closes with Escape", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();

    render(drawer(true, onClose));

    await user.click(screen.getByRole("tab", { name: "版本" }));
    expect(await screen.findByText("版本 #2")).toBeInTheDocument();
    expect(sdk.listFileRevisions).toHaveBeenCalledWith(
      "00000000-0000-4000-8000-000000000001",
      file.id,
      { limit: 50 },
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );

    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });

  it("shows preview and download only when backed by a real capability", async () => {
    render(drawer());

    expect(screen.getByText("当前后端未提供可直接展示的预览内容。")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "下载" })).not.toBeInTheDocument();
  });

  it("separates loading, empty, error and forbidden revision states", async () => {
    let reject!: (error: Error) => void;
    sdk.listFileRevisions.mockImplementationOnce(
      () => new Promise((_resolve, nextReject) => { reject = nextReject; }),
    );
    const rendered = render(drawer());
    await userEvent.click(screen.getByRole("tab", { name: "版本" }));
    expect(screen.getByRole("status")).toHaveTextContent("正在加载版本");
    reject(new BffError("forbidden", "FORBIDDEN", 403));
    expect(await screen.findByText("无权查看该文件的版本。")).toBeInTheDocument();

    sdk.listFileRevisions.mockResolvedValueOnce({ items: [], next_cursor: "" });
    rendered.rerender(drawer(false));
    rendered.rerender(drawer());
    await userEvent.click(screen.getByRole("tab", { name: "版本" }));
    expect(await screen.findByText("暂无版本记录。")).toBeInTheDocument();

    sdk.listFileRevisions.mockRejectedValueOnce(new Error("offline"));
    rendered.rerender(drawer(false));
    rendered.rerender(drawer());
    await userEvent.click(screen.getByRole("tab", { name: "版本" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("加载版本失败"));
    sdk.listFileRevisions.mockResolvedValueOnce({ items: [], next_cursor: "" });
    await userEvent.click(screen.getByRole("button", { name: "重试加载版本" }));
    expect(await screen.findByText("暂无版本记录。")).toBeInTheDocument();
  });

  it("clears stale node data and ignores late revision responses after a node switch", async () => {
    let resolveOld!: (value: unknown) => void;
    const oldRequest = new Promise((resolve) => { resolveOld = resolve; });
    const nextFile: FileNodeVM = {
      ...file,
      id: "00000000-0000-4000-8000-000000000021",
      name: "新文件.md",
      current_revision_id: "00000000-0000-4000-8000-000000000022",
    };
    sdk.listFileRevisions
      .mockReturnValueOnce(oldRequest)
      .mockResolvedValueOnce({
        items: [{
          id: nextFile.current_revision_id,
          node_id: nextFile.id,
          revision_no: 9,
          size: 9,
          created_by: nextFile.owner_id,
        }],
        next_cursor: "",
      });

    const rendered = render(drawer());
    await userEvent.click(screen.getByRole("tab", { name: "版本" }));
    await waitFor(() => expect(sdk.listFileRevisions).toHaveBeenCalledTimes(1));
    const oldSignal = sdk.listFileRevisions.mock.calls[0]?.[3]?.signal as AbortSignal;

    rendered.rerender(drawer(true, vi.fn(), nextFile));
    expect(screen.queryByText("版本 #2")).not.toBeInTheDocument();
    expect(await screen.findByText("版本 #9")).toBeInTheDocument();
    expect(oldSignal.aborted).toBe(true);

    resolveOld({
      items: [{
        id: file.current_revision_id,
        node_id: file.id,
        revision_no: 2,
        size: 42,
        created_by: file.owner_id,
      }],
      next_cursor: "",
    });
    await waitFor(() => expect(screen.queryByText("版本 #2")).not.toBeInTheDocument());
  });

  it("reloads audits for a new node, supports retry and ignores late audit responses", async () => {
    let resolveOld!: (value: unknown) => void;
    sdk.listFileAuditEvents.mockImplementationOnce(
      () => new Promise((resolve) => { resolveOld = resolve; }),
    );
    const nextFile: FileNodeVM = {
      ...file,
      id: "00000000-0000-4000-8000-000000000041",
      name: "审计目标.md",
    };
    sdk.listFileAuditEvents.mockRejectedValueOnce(new Error("offline"));

    const rendered = render(drawer());
    await userEvent.click(screen.getByRole("tab", { name: "审计" }));
    await waitFor(() => expect(sdk.listFileAuditEvents).toHaveBeenCalledTimes(1));
    const oldSignal = sdk.listFileAuditEvents.mock.calls[0]?.[2]?.signal as AbortSignal;
    rendered.rerender(drawer(true, vi.fn(), nextFile));

    expect(oldSignal.aborted).toBe(true);
    expect(await screen.findByRole("alert")).toHaveTextContent("加载审计失败");
    sdk.listFileAuditEvents.mockResolvedValueOnce({ items: [], next_cursor: "" });
    await userEvent.click(screen.getByRole("button", { name: "重试加载审计" }));
    expect(await screen.findByText("暂无相关审计记录。")).toBeInTheDocument();

    resolveOld({
      items: [{
        id: "00000000-0000-4000-8000-000000000051",
        actor_id: file.owner_id,
        action: "stale.audit",
        resource_type: "filesystem",
        resource_id: file.id,
        result: "ALLOW",
        created_at: "2026-11-14T04:00:00Z",
        detail: {},
      }],
      next_cursor: "",
    });
    await waitFor(() => expect(screen.queryByText("stale.audit")).not.toBeInTheDocument());
    expect(sdk.listFileAuditEvents).toHaveBeenLastCalledWith(
      expect.any(String),
      expect.objectContaining({ resource_id: nextFile.id }),
      expect.any(Object),
    );
  });

  it("implements roving tabs with arrow keys and linked tabpanels", async () => {
    render(drawer());
    const detailTab = screen.getByRole("tab", { name: "详情" });
    const revisionTab = screen.getByRole("tab", { name: "版本" });
    expect(screen.getAllByRole("tab")).toHaveLength(3);
    expect(
      screen
        .getAllByRole("tab")
        .every((tab) => tab.classList.contains("filesystem-touch-target")),
    ).toBe(true);
    expect(detailTab).toHaveAttribute("tabindex", "0");
    expect(revisionTab).toHaveAttribute("tabindex", "-1");
    expect(detailTab).toHaveAttribute("aria-controls");
    expect(screen.getByRole("tabpanel")).toHaveAttribute(
      "aria-labelledby",
      detailTab.getAttribute("id"),
    );

    detailTab.focus();
    fireEvent.keyDown(detailTab, { key: "ArrowRight" });
    expect(revisionTab).toHaveFocus();
    expect(revisionTab).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("tabpanel")).toHaveAttribute(
      "aria-labelledby",
      revisionTab.getAttribute("id"),
    );
  });
});
