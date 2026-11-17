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
  getEffectiveAccess: vi.fn(),
  listGovernedGrants: vi.fn(),
  listShareRequests: vi.fn(),
  getFileNode: vi.fn(),
  getFileStorage: vi.fn(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  listFileRevisions: sdk.listFileRevisions,
  listFileAuditEvents: sdk.listFileAuditEvents,
  getEffectiveAccess: sdk.getEffectiveAccess,
  listGovernedGrants: sdk.listGovernedGrants,
  listShareRequests: sdk.listShareRequests,
  getFileNode: sdk.getFileNode,
  getFileStorage: sdk.getFileStorage,
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
vi.mock("./FilePreviewPane", () => ({
  FilePreviewPane: () => createElement("div", null, "文件预览区域"),
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
    organizationId: "00000000-0000-4000-8000-000000000071",
    onClose,
  });
}

describe("FileDetailDrawer", () => {
  beforeEach(() => {
    sdk.getFileStorage.mockReset();
    sdk.getFileStorage.mockResolvedValue({
      revision_id: file.current_revision_id ?? "00000000-0000-4000-8000-000000000099",
      blob_id: "00000000-0000-4000-8000-000000000098",
      provider_id: "00000000-0000-4000-8000-000000000097",
      provider_type: "LOCAL",
      provider_root: "A:\\21-Agent\\SparkOpsAgent\\logs",
      object_key: "transfers/demo/content",
      content_sha256: "ed2b89b68f11227164ecdd887a0b3ea122098cc0a66713c7ca9165ee7f35d3fb",
      local_object_file:
        "A:\\21-Agent\\SparkOpsAgent\\logs\\e60c18b8d64a19712695ab520548264e6f7f16bceae2c7aa1110ee46ef1fc7e8.object",
    });
    sdk.getFileNode.mockReset();
    sdk.getFileNode.mockResolvedValue({
      id: file.parent_id!,
      namespace_id: file.namespace_id,
      parent_id: null,
      type: "DIRECTORY",
      name: "test",
      current_revision_id: null,
      state: "ACTIVE",
      owner_id: file.owner_id,
    });
    sdk.listFileRevisions.mockReset();
    sdk.listFileAuditEvents.mockReset();
    sdk.getEffectiveAccess.mockReset();
    sdk.listGovernedGrants.mockReset();
    sdk.listShareRequests.mockReset();
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
    sdk.getEffectiveAccess.mockResolvedValue({
      decision: "ALLOW",
      effective_actions: ["READ", "DOWNLOAD", "SHARE"],
      reason_code: "resource_grant",
      source_grant_ids: ["00000000-0000-4000-8000-000000000061"],
      policy_version: 2,
      cache_key: {
        organization_id: "00000000-0000-4000-8000-000000000071",
        workspace_id: "00000000-0000-4000-8000-000000000001",
        actor_id: "00000000-0000-4000-8000-000000000072",
        node_id: file.id,
        action: "READ",
        policy_version: 2,
      },
    });
    sdk.listGovernedGrants.mockResolvedValue({
      items: [{
        id: "00000000-0000-4000-8000-000000000061",
        node_id: file.id,
        grantee_id: "00000000-0000-4000-8000-000000000073",
        principal_type: "WORKSPACE",
        principal_id: "00000000-0000-4000-8000-000000000073",
        permission: "READ",
        effect: "ALLOW",
        inherit_to_children: false,
        source: "DIRECT",
        expires_at: "2026-12-31T16:00:00Z",
      }, {
        id: "00000000-0000-4000-8000-000000000062",
        node_id: "00000000-0000-4000-8000-000000000003",
        grantee_id: "00000000-0000-4000-8000-000000000074",
        principal_type: "USER",
        principal_id: "00000000-0000-4000-8000-000000000074",
        permission: "DELETE",
        effect: "DENY",
        inherit_to_children: true,
        source: "INHERITED",
      }],
      next_cursor: "",
      truncated: false,
    });
    sdk.listShareRequests.mockResolvedValue({
      items: [{
        id: "00000000-0000-4000-8000-000000000081",
        organization_id: "00000000-0000-4000-8000-000000000071",
        namespace_id: file.namespace_id,
        node_id: file.id,
        requested_by: file.owner_id,
        principal_type: "WORKSPACE",
        principal_ids: ["00000000-0000-4000-8000-000000000073"],
        actions: ["WRITE"],
        inherit_to_children: false,
        classification: "CONFIDENTIAL",
        effect: "ALLOW",
        status: "PENDING",
        created_at: "2026-11-15T00:00:00Z",
      }],
      next_cursor: "",
      truncated: false,
    });
  });

  it("renders shared projection details without workspace-scoped reads or raw ids", () => {
    render(createElement(FileDetailDrawer, {
      open: true,
      node: file,
      workspaceId: "workspace-that-does-not-own-source",
      projectionOnly: true,
      projectionDetails: [
        { label: "来源", value: "研发共享" },
        { label: "原路径", value: "/项目/规格说明.md" },
      ],
      onClose: vi.fn(),
    }));

    expect(screen.getByText("研发共享")).toBeInTheDocument();
    expect(screen.getByText("/项目/规格说明.md")).toBeInTheDocument();
    expect(screen.queryByText(file.id)).not.toBeInTheDocument();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(sdk.listFileRevisions).not.toHaveBeenCalled();
    expect(sdk.listFileAuditEvents).not.toHaveBeenCalled();
  });

  afterEach(() => cleanup());

  it("shows underlying storage metadata on the details tab", async () => {
    render(drawer());
    expect(await screen.findByText("存储对象键")).toBeInTheDocument();
    expect(screen.getByText("transfers/demo/content")).toBeInTheDocument();
    expect(screen.getByText("物理对象文件")).toBeInTheDocument();
    expect(
      screen.getByText(
        "A:\\21-Agent\\SparkOpsAgent\\logs\\e60c18b8d64a19712695ab520548264e6f7f16bceae2c7aa1110ee46ef1fc7e8.object",
      ),
    ).toBeInTheDocument();
  });

  it("shows owner name, timestamps, and owner id on the details tab", () => {
    render(
      createElement(FileDetailDrawer, {
        open: true,
        node: {
          ...file,
          created_at: "2026-11-15T08:00:00Z",
          updated_at: "2026-11-17T10:30:00Z",
        },
        workspaceId: "00000000-0000-4000-8000-000000000001",
        members: [
          {
            member_id: file.owner_id,
            display_name: "王五",
            email: "wang@example.com",
            roles: [{ id: "role-1", name: "Member", permissions: [] }],
          },
        ],
        onClose: vi.fn(),
      }),
    );

    expect(screen.getByText("王五")).toBeInTheDocument();
    expect(screen.getByText(file.owner_id)).toBeInTheDocument();
    expect(screen.getByText("创建时间")).toBeInTheDocument();
    expect(screen.getByText("更新时间")).toBeInTheDocument();
  });

  it("shows the file path on the details tab", () => {
    render(
      createElement(FileDetailDrawer, {
        open: true,
        node: file,
        workspaceId: "00000000-0000-4000-8000-000000000001",
        displayPath: "/test/K8s.md",
        onClose: vi.fn(),
      }),
    );

    expect(screen.getByText("路径")).toBeInTheDocument();
    expect(screen.getByText("/test/K8s.md")).toBeInTheDocument();
  });

  it("loads real revisions when opened and closes with Escape", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();

    render(drawer(true, onClose));

    await user.click(screen.getByRole("tab", { name: "版本" }));
    expect(await screen.findByText("版本 #2")).toBeInTheDocument();
    expect(sdk.listFileRevisions).toHaveBeenCalledWith(
      "00000000-0000-4000-8000-000000000001",
      file.id,
      { limit: 50, namespace_id: file.namespace_id },
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );

    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });

  it("renders the file preview pane for file nodes", () => {
    render(drawer());
    expect(screen.getByText("文件预览区域")).toBeInTheDocument();
  });

  it("shows effective access, grant source, audience, expiry, pending approval and deny", async () => {
    render(drawer());
    await userEvent.click(screen.getByRole("tab", { name: "共享与权限" }));

    expect(await screen.findByText("READ、DOWNLOAD、SHARE")).toBeInTheDocument();
    expect(screen.getByText("直接授权")).toBeInTheDocument();
    expect(screen.getByText("继承授权")).toBeInTheDocument();
    expect(screen.getByText("WORKSPACE")).toBeInTheDocument();
    expect(screen.getByText(/2026-12-31/)).toBeInTheDocument();
    expect(screen.getByText("等待审批")).toBeInTheDocument();
    expect(screen.getByText("显式拒绝 · DELETE")).toBeInTheDocument();
  });

  it("loads subsequent governance pages without duplicating records", async () => {
    const firstGrant = {
      id: "00000000-0000-4000-8000-000000000061",
      node_id: file.id,
      grantee_id: "00000000-0000-4000-8000-000000000073",
      principal_type: "WORKSPACE" as const,
      principal_id: "00000000-0000-4000-8000-000000000073",
      permission: "READ" as const,
      effect: "ALLOW" as const,
      inherit_to_children: false,
      source: "DIRECT" as const,
    };
    const secondGrant = {
      ...firstGrant,
      id: "00000000-0000-4000-8000-000000000063",
      permission: "SHARE" as const,
    };
    sdk.listGovernedGrants
      .mockResolvedValueOnce({
        items: [firstGrant],
        next_cursor: "grant-page-2",
        truncated: true,
      })
      .mockResolvedValueOnce({
        items: [firstGrant, secondGrant],
        next_cursor: "",
        truncated: false,
      });
    render(drawer());
    await userEvent.click(screen.getByRole("tab", { name: "共享与权限" }));

    await userEvent.click(await screen.findByRole("button", { name: "加载更多" }));
    expect(await screen.findByText("SHARE")).toBeInTheDocument();
    expect(screen.getAllByText("READ")).toHaveLength(1);
    expect(sdk.listGovernedGrants).toHaveBeenLastCalledWith(
      file.id,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
      { cursor: "grant-page-2" },
    );
    expect(screen.queryByRole("button", { name: "加载更多" })).not.toBeInTheDocument();
  });

  it("shows governance metadata as restricted on 403 without leaking grants", async () => {
    sdk.listGovernedGrants.mockRejectedValue(
      new BffError("forbidden", "FILESYSTEM_RESOURCE_FORBIDDEN", 403),
    );
    sdk.listShareRequests.mockRejectedValue(
      new BffError("forbidden", "FILESYSTEM_RESOURCE_FORBIDDEN", 403),
    );
    render(drawer());
    await userEvent.click(screen.getByRole("tab", { name: "共享与权限" }));

    expect(await screen.findByText("治理元数据受限")).toBeInTheDocument();
    expect(screen.queryByText("00000000-0000-4000-8000-000000000073")).not.toBeInTheDocument();
    expect(screen.getByText("READ、DOWNLOAD、SHARE")).toBeInTheDocument();
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
    const accessTab = screen.getByRole("tab", { name: "共享与权限" });
    expect(screen.getAllByRole("tab")).toHaveLength(4);
    expect(
      screen
        .getAllByRole("tab")
        .every((tab) => tab.classList.contains("filesystem-touch-target")),
    ).toBe(true);
    expect(detailTab).toHaveAttribute("tabindex", "0");
    expect(accessTab).toHaveAttribute("tabindex", "-1");
    expect(detailTab).toHaveAttribute("aria-controls");
    expect(screen.getByRole("tabpanel")).toHaveAttribute(
      "aria-labelledby",
      detailTab.getAttribute("id"),
    );

    detailTab.focus();
    fireEvent.keyDown(detailTab, { key: "ArrowRight" });
    expect(accessTab).toHaveFocus();
    expect(accessTab).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("tabpanel")).toHaveAttribute(
      "aria-labelledby",
      accessTab.getAttribute("id"),
    );
  });
});
