// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { BffError } from "@sparkops/contracts";
import { AuditWorkspace } from "./AuditWorkspace";

const sdk = vi.hoisted(() => ({
  listFileAuditEvents: vi.fn(),
  listOrganizationFileAuditEvents: vi.fn(),
}));
const drawerRenders = vi.hoisted(() => [] as string[]);
const navigation = vi.hoisted(() => ({
  replace: vi.fn(),
  searchParams: new URLSearchParams(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  listFileAuditEvents: sdk.listFileAuditEvents,
  listOrganizationFileAuditEvents: sdk.listOrganizationFileAuditEvents,
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
  }) => {
    if (open) drawerRenders.push(title);
    return open ? createElement("div", { role: "dialog", "aria-label": title }, children) : null;
  },
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
      mode: "workspace",
    }),
  );
}

describe("AuditWorkspace", () => {
  beforeEach(() => {
    sdk.listFileAuditEvents.mockReset();
    sdk.listOrganizationFileAuditEvents.mockReset();
    drawerRenders.length = 0;
    navigation.replace.mockReset();
    navigation.searchParams = new URLSearchParams("tab=audit&action=filesystem.node.rename");
    sdk.listFileAuditEvents.mockResolvedValue({ items: [event], next_cursor: "20" });
  });

  it("uses the organization audit feed for organization file administrators", async () => {
    sdk.listOrganizationFileAuditEvents.mockResolvedValue({ items: [event], next_cursor: "" });
    render(createElement(AuditWorkspace, {
      workspaceId: "00000000-0000-4000-8000-000000000001",
      organizationId: "00000000-0000-4000-8000-000000000002",
      permissions: ["org:file:admin"],
      mode: "organization",
    }));

    expect(await screen.findByText(event.action)).toBeInTheDocument();
    expect(sdk.listOrganizationFileAuditEvents).toHaveBeenCalledWith(
      expect.any(Object),
      expect.objectContaining({ organizationId: "00000000-0000-4000-8000-000000000002" }),
    );
    expect(sdk.listFileAuditEvents).not.toHaveBeenCalled();
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
    const rendered = renderWorkspace();
    await screen.findByText("filesystem.node.rename");

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));

    await waitFor(() =>
      expect(navigation.replace).toHaveBeenCalledWith(expect.stringContaining("audit_cursor=20")),
    );
    expect(screen.getByRole("button", { name: "上一页" })).toBeInTheDocument();

    navigation.searchParams = new URLSearchParams(
      "tab=audit&action=filesystem.node.rename&audit_cursor=20",
    );
    rendered.rerender(
      createElement(AuditWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        permissions: ["file:admin"],
        mode: "workspace",
      }),
    );
    navigation.searchParams = new URLSearchParams(
      "tab=audit&action=filesystem.node.rename",
    );
    rendered.rerender(
      createElement(AuditWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        permissions: ["file:admin"],
        mode: "workspace",
      }),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled(),
    );

    navigation.searchParams = new URLSearchParams(
      "tab=audit&action=filesystem.node.rename&audit_cursor=20",
    );
    rendered.rerender(
      createElement(AuditWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        permissions: ["file:admin"],
        mode: "workspace",
      }),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "上一页" })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "上一页" }));
    await waitFor(() =>
      expect(navigation.replace).toHaveBeenLastCalledWith(
        expect.not.stringContaining("audit_cursor="),
      ),
    );
  });

  it("synchronizes URL filters on history navigation", async () => {
    const rendered = renderWorkspace();
    await screen.findByText("filesystem.node.rename");

    navigation.searchParams = new URLSearchParams(
      "tab=audit&action=filesystem.node.delete&audit_cursor=40",
    );
    sdk.listFileAuditEvents.mockResolvedValueOnce({
      items: [{ ...event, action: "filesystem.node.delete" }],
      next_cursor: "",
    });
    rendered.rerender(
      createElement(AuditWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        permissions: ["file:admin"],
        mode: "workspace",
      }),
    );

    expect(screen.getByLabelText("动作")).toHaveValue("filesystem.node.delete");
    await waitFor(() =>
      expect(sdk.listFileAuditEvents).toHaveBeenLastCalledWith(
        expect.any(String),
        expect.objectContaining({ action: "filesystem.node.delete", cursor: "40" }),
        expect.any(Object),
      ),
    );
  });

  it("aborts stale requests and ignores their late responses", async () => {
    let resolveOld!: (value: unknown) => void;
    sdk.listFileAuditEvents.mockImplementationOnce(
      () => new Promise((resolve) => { resolveOld = resolve; }),
    );
    const rendered = renderWorkspace();
    await waitFor(() => expect(sdk.listFileAuditEvents).toHaveBeenCalledTimes(1));
    const oldSignal = sdk.listFileAuditEvents.mock.calls[0]?.[2]?.signal as AbortSignal;

    navigation.searchParams = new URLSearchParams("tab=audit&action=new.action");
    sdk.listFileAuditEvents.mockResolvedValueOnce({
      items: [{ ...event, action: "new.action" }],
      next_cursor: "",
    });
    rendered.rerender(
      createElement(AuditWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        permissions: ["file:admin"],
        mode: "workspace",
      }),
    );
    expect(await screen.findByText("new.action")).toBeInTheDocument();
    expect(oldSignal.aborted).toBe(true);

    resolveOld({ items: [{ ...event, action: "stale.action" }], next_cursor: "" });
    await waitFor(() => expect(screen.queryByText("stale.action")).not.toBeInTheDocument());
  });

  it("isolates visible state in the same render when workspace or URL filters change", async () => {
    const rendered = renderWorkspace();
    fireEvent.click(
      await screen.findByRole("button", {
        name: "查看审计事件 filesystem.node.rename",
      }),
    );
    expect(screen.getByRole("dialog", { name: "审计事件详情" })).toBeInTheDocument();

    let resolveFilter!: (value: unknown) => void;
    sdk.listFileAuditEvents.mockImplementationOnce(
      () => new Promise((resolve) => { resolveFilter = resolve; }),
    );
    navigation.searchParams = new URLSearchParams("tab=audit&action=new.filter");
    drawerRenders.length = 0;
    rendered.rerender(
      createElement(AuditWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        permissions: ["file:admin"],
        mode: "workspace",
      }),
    );

    expect(screen.queryByText("filesystem.node.rename")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog", { name: "审计事件详情" })).not.toBeInTheDocument();
    expect(drawerRenders).toEqual([]);
    expect(screen.getByRole("status")).toHaveTextContent("正在加载审计记录");

    resolveFilter({
      items: [{ ...event, action: "new.filter" }],
      next_cursor: "",
    });
    expect(await screen.findByText("new.filter")).toBeInTheDocument();

    sdk.listFileAuditEvents.mockImplementationOnce(() => new Promise(() => {}));
    rendered.rerender(
      createElement(AuditWorkspace, {
        workspaceId: "00000000-0000-4000-8000-000000000099",
        permissions: ["file:admin"],
        mode: "workspace",
      }),
    );

    expect(screen.queryByText("new.filter")).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("正在加载审计记录");
  });

  it("retries an audit request after an error", async () => {
    sdk.listFileAuditEvents.mockRejectedValueOnce(new Error("offline"));
    renderWorkspace();
    expect(await screen.findByRole("alert")).toHaveTextContent("加载审计记录失败");

    sdk.listFileAuditEvents.mockResolvedValueOnce({ items: [event], next_cursor: "" });
    fireEvent.click(screen.getByRole("button", { name: "重试加载审计" }));
    expect(await screen.findByText("filesystem.node.rename")).toBeInTheDocument();
  });
});
