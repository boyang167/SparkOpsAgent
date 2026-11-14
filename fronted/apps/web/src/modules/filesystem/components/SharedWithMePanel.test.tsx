// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SharedWithMePanel, type SharedProjectionItem } from "./SharedWithMePanel";

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
  sourcePrincipalType: SharedProjectionItem["source_principal_type"],
): SharedProjectionItem {
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
    source_principal_type: sourcePrincipalType,
    source_label: `来源 ${index}`,
    owner_label: `所有者 ${index}`,
    sharer_label: `分享者 ${index}`,
    expires_at: "2026-12-31T16:00:00.000Z",
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
      items,
      state: "ready",
    }));

    sourceTypes.forEach(([, label]) =>
      expect(screen.getByRole("heading", { name: label })).toBeInTheDocument(),
    );
    expect(screen.getByText("来源 1")).toBeInTheDocument();
    expect(screen.getByText("所有者 1 · 分享者 1")).toBeInTheDocument();
    expect(screen.getAllByText("READ、DOWNLOAD")).toHaveLength(4);
    expect(screen.getByText("/项目/共享文件 1.md")).toBeInTheDocument();
    expect(screen.queryByText(items[0]!.node.owner_id)).not.toBeInTheDocument();
  });

  it("keeps unknown principal types in a safe fallback group", () => {
    render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
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

  it.each([
    ["loading", "正在加载共享内容…"],
    ["empty", "当前没有与你共享的文件。"],
    ["error", "共享内容加载失败，请稍后重试。"],
  ] as const)("renders a distinct %s state", (state, message) => {
    render(createElement(SharedWithMePanel, {
      workspaceId: "workspace-1",
      items: [],
      state,
    }));

    expect(screen.getByText(message)).toBeInTheDocument();
  });
});
