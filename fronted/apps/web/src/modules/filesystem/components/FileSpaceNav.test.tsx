// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FileSpaceNav } from "./FileSpaceNav";

vi.mock("./DirectoryTree", () => ({
  DirectoryTree: () => createElement("div", { role: "tree", "aria-label": "目录" }),
}));

const namespace = {
  id: "namespace-1",
  namespace_type: "WORKSPACE" as const,
  organization_id: "org-1",
  workspace_id: "workspace-1",
  owner_user_id: null,
  root_node_id: "root-1",
  status: "ACTIVE",
  capabilities: ["READ" as const],
  entry_node_ids: [],
  entry_nodes_truncated: false,
};

describe("FileSpaceNav", () => {
  afterEach(cleanup);

  it("renders context and directory tree for non-shared spaces", () => {
    render(createElement(FileSpaceNav, {
      activeSpace: "workspace",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      namespace,
      workspaceId: "workspace-1",
    }));
    expect(screen.getByText("星火科技 · 研发平台")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "文件夹" })).toBeInTheDocument();
    expect(screen.getByRole("tree", { name: "目录" })).toBeInTheDocument();
  });

  it("does not render a directory tree for shared", () => {
    render(createElement(FileSpaceNav, {
      activeSpace: "shared",
      organizationLabel: "星火科技",
      workspaceLabel: "研发平台",
      workspaceId: "workspace-1",
    }));
    expect(screen.queryByRole("tree")).not.toBeInTheDocument();
  });
});