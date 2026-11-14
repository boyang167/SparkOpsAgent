// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { BffError } from "@sparkops/contracts";
import { AuditWorkspace } from "./AuditWorkspace";

const sdk = vi.hoisted(() => ({ listFileAuditEvents: vi.fn() }));
const navigation = vi.hoisted(() => ({
  replace: vi.fn(),
  searchParams: new URLSearchParams(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  listFileAuditEvents: sdk.listFileAuditEvents,
}));
vi.mock("@/components/ui/Drawer", () => ({
  Drawer: ({
    open,
    title,
    children,
  }: {
    open: boolean;
    title: string;
    children: React.ReactNode;
  }) => open ? createElement("div", { role: "dialog", "aria-label": title }, children) : null,
}));
vi.mock("@/components/ui/InlineAlert", () => ({
  InlineAlert: ({ children }: { children: React.ReactNode }) =>
    createElement("div", null, children),
}));
vi.mock("next/navigation", () => ({
  usePathname: () => "/files",
  useRouter: () => ({ replace: navigation.replace }),
  useSearchParams: () => navigation.searchParams,
}));

const event = {
  id: "00000000-0000-4000-8000-000000000031",
  actor_id: "00000000-0000-4000-8000-000000000032",
  action: "filesystem.node.rename",
  resource_type: "filesystem",
  resource_id: "00000000-0000-4000-8000-000000000033",
  result: "ALLOW",
  trace_id: "trace-7",
  created_at: "2026-11-14T04:00:00Z",
  detail: { workspace_id: "00000000-0000-4000-8000-000000000001", previous_name: "旧名" },
};

function renderWorkspace() {
  return render(
    createElement(AuditWorkspace, {
      workspaceId: "00000000-0000-4000-8000-000000000001",
      permissions: ["file:admin"],
    }),
  );
}

describe("AuditWorkspace", () => {
  beforeEach(() => {
    sdk.listFileAuditEvents.mockReset();
    navigation.replace.mockReset();
    navigation.searchParams = new URLSearchParams("tab=audit&action=filesystem.node.rename");
    sdk.listFileAuditEvents.mockResolvedValue({ items: [event], next_cursor: "20" });
  });

  afterEach(() => cleanup());

  it("loads URL filters, keeps advanced filters collapsed and writes filters back to URL", async () => {
    const user = userEvent.setup();
    renderWorkspace();

    expect(await screen.findByText("filesystem.node.rename")).toBeInTheDocument();
    expect(sdk.listFileAuditEvents).toHaveBeenCalledWith(
      "00000000-0000-4000-8000-000000000001",
      expect.objectContaining({ action: "filesystem.node.rename", limit: 20 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(screen.queryByLabelText("操作者 ID")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "高级筛选" }));
    await user.type(screen.getByLabelText("资源 ID"), "resource-7");
    await user.click(screen.getByRole("button", { name: "查询" }));

    expect(navigation.replace).toHaveBeenCalledWith(
      expect.stringContaining("resource_id=resource-7"),
    );
    expect(navigation.replace).toHaveBeenCalledWith(
      expect.stringContaining("tab=audit"),
    );
  });

  it("opens an event detail drawer with actor, resource, decision, trace and detail JSON", async () => {
    renderWorkspace();

    fireEvent.click(await screen.findByRole("button", { name: "查看审计事件 filesystem.node.rename" }));

    const detail = within(screen.getByRole("dialog", { name: "审计事件详情" }));
    expect(detail.getByText(event.actor_id)).toBeInTheDocument();
    expect(detail.getByText(event.resource_id)).toBeInTheDocument();
    expect(detail.getByText("ALLOW")).toBeInTheDocument();
    expect(detail.getByText("trace-7")).toBeInTheDocument();
    expect(detail.getByText(/previous_name/)).toBeInTheDocument();
  });

  it("keeps forbidden separate from loading, error and empty states", async () => {
    sdk.listFileAuditEvents.mockRejectedValue(
      new BffError("forbidden", "FORBIDDEN", 403),
    );
    renderWorkspace();

    expect(screen.getByRole("status")).toHaveTextContent("正在加载审计记录");
    expect(await screen.findByText("无权查看文件审计。")).toBeInTheDocument();
  });

  it("uses the backend cursor for pagination and persists it in the URL", async () => {
    renderWorkspace();
    await screen.findByText("filesystem.node.rename");

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));

    await waitFor(() =>
      expect(navigation.replace).toHaveBeenCalledWith(expect.stringContaining("audit_cursor=20")),
    );
  });
});
