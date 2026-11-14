// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { SharedItemVM } from "@sparkops/contracts/filesystem";
import { SharedWithMePanel } from "./SharedWithMePanel";

const detail = vi.hoisted(() => ({
  nodeId: "",
  projectionOnly: false,
}));

vi.mock("./FileDetailDrawer", () => ({
  FileDetailDrawer: ({
    open,
    node,
    projectionOnly,
  }: {
    open: boolean;
    node: { id: string; name: string } | null;
    projectionOnly?: boolean;
  }) => {
    if (!open || !node) return null;
    detail.nodeId = node.id;
    detail.projectionOnly = Boolean(projectionOnly);
    return createElement("div", { role: "dialog", "aria-label": "文件详情" }, node.name);
  },
}));

const sourceTypes = [
  ["ORGANIZATION", "租户发布"],
  ["USER", "其他用户共享"],
  ["WORKSPACE", "工作区共享"],
  ["AGENT", "Agent / Workflow 产物"],
] as const;

function item(
  index: number,
  sourcePrincipalType: SharedItemVM["sources"][number]["principal_type"],
  expiresAt: string | null = "2026-12-31T16:00:00.000Z",
): SharedItemVM {
  return {
    node: {
      id: `00000000-0000-4000-8000-0000000000${index}`,
      namespace_id: `10000000-0000-4000-8000-0000000000${index}`,
      parent_id: null,
      type: "FILE",
      name: `共享文件 ${index}.md`,
      current_revision_id: null,
      state: "ACTIVE",
      owner_id: `20000000-0000-4000-8000-0000000000${index}`,
    },
    effective_actions: ["READ", "DOWNLOAD"],
    source_grant_ids: [`30000000-0000-4000-8000-0000000000${index}`],
    sources: [{
      grant_id: `30000000-0000-4000-8000-0000000000${index}`,
      principal_type: sourcePrincipalType,
      principal_id: `40000000-0000-4000-8000-0000000000${index}`,
      expires_at: expiresAt,
    }],
    source_namespace: {
      namespace_type: index === 2 ? "USER" : "WORKSPACE",
      owner_user_id: index === 2
        ? `50000000-0000-4000-8000-0000000000${index}`
        : null,
    },
    original_path: `/项目/共享文件 ${index}.md`,
  };
}

describe("SharedWithMePanel", () => {
  afterEach(() => {
    cleanup();
    detail.nodeId = "";
    detail.projectionOnly = false;
  });

  it("groups shared projections by source and shows authorized metadata", () => {
    const items = sourceTypes.map(([type], index) => item(index + 1, type));
    render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items,
      state: "ready",
    }));

    sourceTypes.forEach(([, label]) =>
      expect(screen.getByRole("heading", { name: label })).toBeInTheDocument(),
    );
    expect(screen.getByText("星火科技")).toBeInTheDocument();
    expect(screen.getAllByText("研发平台 · 共享者信息不可用")).toHaveLength(3);
    expect(screen.getAllByText("READ、DOWNLOAD")).toHaveLength(4);
    expect(screen.getByText("/项目/共享文件 1.md")).toBeInTheDocument();
    expect(screen.queryByText(items[0]!.node.owner_id)).not.toBeInTheDocument();
  });

  it("keeps unknown principal types in a safe fallback group", () => {
    render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [item(5, "SERVICE_ACCOUNT"), item(6, "ROLE")],
      state: "ready",
    }));

    expect(screen.getByRole("heading", { name: "其他授权来源" })).toBeInTheDocument();
    expect(screen.getByText("共享文件 5.md")).toBeInTheDocument();
    expect(screen.getByText("共享文件 6.md")).toBeInTheDocument();
  });

  it("opens projection-only detail by original node id without mutation targets", () => {
    const shared = item(7, "USER");
    const { container } = render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [shared],
      state: "ready",
    }));

    fireEvent.click(screen.getByRole("button", { name: "打开 共享文件 7.md" }));

    expect(detail.nodeId).toBe(shared.node.id);
    expect(detail.projectionOnly).toBe(true);
    expect(screen.getByRole("dialog", { name: "文件详情" })).toBeInTheDocument();
    expect(container.querySelector("[draggable='true']")).toBeNull();
    expect(screen.queryByText(/上传|新建目录|移动|重命名|删除/)).not.toBeInTheDocument();
  });

  it("closes stale details when workspace scope or items change", () => {
    const shared = item(8, "USER");
    const rendered = render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [shared],
      state: "ready",
    }));
    fireEvent.click(screen.getByRole("button", { name: "打开 共享文件 8.md" }));
    expect(screen.getByRole("dialog", { name: "文件详情" })).toBeInTheDocument();

    rendered.rerender(createElement(SharedWithMePanel, {
      workspaceId: "workspace-2",
      organizationLabel: "星火科技",
      workspaceLabel: "数据平台",
      items: [shared],
      state: "ready",
    }));
    expect(screen.queryByRole("dialog", { name: "文件详情" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "打开 共享文件 8.md" }));
    rendered.rerender(createElement(SharedWithMePanel, {
      workspaceId: "workspace-2",
      organizationLabel: "星火科技",
      workspaceLabel: "数据平台",
      items: [],
      state: "empty",
    }));
    expect(screen.queryByRole("dialog", { name: "文件详情" })).not.toBeInTheDocument();
  });

  it("formats valid expiry deterministically and safely handles invalid values", () => {
    render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [item(9, "USER"), item(10, "USER", "not-a-date"), item(11, "USER", null)],
      state: "ready",
    }));

    const time = screen.getByText("2026/12/31 16:00 UTC");
    expect(time.tagName).toBe("TIME");
    expect(time).toHaveAttribute("datetime", "2026-12-31T16:00:00.000Z");
    expect(screen.getAllByText("到期信息不可用")).toHaveLength(2);
    expect(screen.queryByText("Invalid Date")).not.toBeInTheDocument();
  });

  it.each([
    ["loading", "正在加载共享内容…"],
    ["empty", "当前没有与你共享的文件。"],
    ["error", "共享内容加载失败，请稍后重试。"],
  ] as const)("renders a distinct %s state", (state, message) => {
    render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [],
      state,
    }));

    expect(screen.getByText(message)).toBeInTheDocument();
  });
});
