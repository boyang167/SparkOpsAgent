// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
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

function drawer(open = true, onClose = vi.fn()) {
  return createElement(FileDetailDrawer, {
    open,
    node: file,
    workspaceId: "00000000-0000-4000-8000-000000000001",
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
  });
});
