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
    projectionDetails,
  }: {
    open: boolean;
    node: { id: string; name: string } | null;
    projectionOnly?: boolean;
    projectionDetails?: Array<{ label: string; value: string }>;
  }) => {
    if (!open || !node) return null;
    detail.nodeId = node.id;
    detail.projectionOnly = Boolean(projectionOnly);
    return createElement(
      "div",
      { role: "dialog", "aria-label": "文件详情" },
      node.name,
      projectionDetails?.map((entry) =>
        createElement("span", { key: entry.label }, entry.value),
      ),
    );
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
      items: [
        item(5, "SERVICE_ACCOUNT"),
        item(6, "ROLE"),
        item(12, "FUTURE_PRINCIPAL"),
      ],
      state: "ready",
    }));

    expect(screen.getByRole("heading", { name: "其他授权来源" })).toBeInTheDocument();
    expect(screen.getByText("共享文件 5.md")).toBeInTheDocument();
    expect(screen.getByText("共享文件 6.md")).toBeInTheDocument();
    expect(screen.getByText("共享文件 12.md")).toBeInTheDocument();
  });

  it("uses stable source priority, shows every source type, and renders a node once", () => {
    const shared = item(13, "USER");
    shared.sources = [
      shared.sources[0]!,
      {
        grant_id: "30000000-0000-4000-8000-000000000099",
        principal_type: "ORGANIZATION",
        principal_id: "40000000-0000-4000-8000-000000000099",
        expires_at: "2027-01-31T16:00:00.000Z",
      },
    ];
    const rendered = render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [shared],
      state: "ready",
    }));

    expect(screen.getByRole("heading", { name: "租户发布" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "其他用户共享" })).not.toBeInTheDocument();
    expect(screen.getByText("租户、其他用户")).toBeInTheDocument();
    expect(screen.getAllByText("共享文件 13.md")).toHaveLength(1);

    rendered.rerender(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [{
        ...shared,
        sources: [...shared.sources].reverse().map((entry, index) => ({
          ...entry,
          grant_id: `60000000-0000-4000-8000-0000000000${index + 1}`,
        })),
      }],
      state: "ready",
    }));
    expect(screen.getByRole("heading", { name: "租户发布" })).toBeInTheDocument();
    expect(screen.getByText("租户、其他用户")).toBeInTheDocument();
  });

  it("uses the active source filter as the primary group", () => {
    const shared = item(14, "ORGANIZATION");
    shared.sources.push({
      grant_id: "30000000-0000-4000-8000-000000000098",
      principal_type: "USER",
      principal_id: "40000000-0000-4000-8000-000000000098",
      expires_at: null,
    });
    render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      source: "user",
      items: [shared],
      state: "ready",
    }));

    expect(screen.getByRole("heading", { name: "其他用户共享" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "租户发布" })).not.toBeInTheDocument();
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

  it("derives an open detail from the latest item projection", () => {
    const shared = item(15, "USER");
    const rendered = render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [shared],
      state: "ready",
    }));
    fireEvent.click(screen.getByRole("button", { name: "打开 共享文件 15.md" }));

    const updated = {
      ...shared,
      node: { ...shared.node, name: "更新后的共享文件.md" },
      effective_actions: ["READ"] as SharedItemVM["effective_actions"],
      original_path: "/更新/共享文件.md",
      sources: [{
        ...shared.sources[0]!,
        principal_type: "AGENT",
      }],
    };
    rendered.rerender(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [updated],
      state: "ready",
    }));

    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent(
      "更新后的共享文件.md",
    );
    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent("READ");
    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent(
      "/更新/共享文件.md",
    );
    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent(
      "Agent / Workflow",
    );
  });

  it("formats valid expiry deterministically and safely handles invalid values", () => {
    const missingExpiry = item(16, "USER");
    delete (missingExpiry.sources[0] as { expires_at?: string | null }).expires_at;
    render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      items: [
        item(9, "USER"),
        item(10, "USER", "not-a-date"),
        item(11, "USER", null),
        missingExpiry,
      ],
      state: "ready",
    }));

    const time = screen.getByText("2026/12/31 16:00 UTC");
    expect(time.tagName).toBe("TIME");
    expect(time).toHaveAttribute("datetime", "2026-12-31T16:00:00.000Z");
    expect(screen.getByText("永久有效")).toBeInTheDocument();
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
