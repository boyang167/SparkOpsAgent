// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FilesystemWorkbench } from "./FilesystemWorkbench";

const navigation = vi.hoisted(() => ({
  replace: vi.fn(),
  searchParams: new URLSearchParams(),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/files",
  useRouter: () => ({ replace: navigation.replace }),
  useSearchParams: () => navigation.searchParams,
}));

vi.mock("./FileExplorerPanel", () => ({
  FileExplorerPanel: () => null,
}));

const namespace = {
  id: "00000000-0000-4000-8000-000000000002",
  workspace_id: "00000000-0000-4000-8000-000000000001",
  root_node_id: "00000000-0000-4000-8000-000000000003",
  status: "ACTIVE",
};

function renderWorkbench(permissions: string[] = ["file:read"]) {
  return render(
    createElement(FilesystemWorkbench, {
      workspaceId: "00000000-0000-4000-8000-000000000001",
      namespace,
      initialNodes: [],
      initialState: { tab: "files", view: "list", query: "" },
      permissions,
    }),
  );
}

describe("FilesystemWorkbench", () => {
  afterEach(cleanup);

  beforeEach(() => {
    navigation.replace.mockReset();
    navigation.searchParams = new URLSearchParams();
  });

  it("selects files and hides governance without storage read permission", () => {
    renderWorkbench();

    expect(screen.getByRole("tab", { name: "文件" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("tab", { name: "存储治理" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "审计" })).not.toBeInTheDocument();
  });

  it("shows audit only with file admin permission", () => {
    renderWorkbench(["file:read", "file:admin"]);

    expect(screen.getByRole("tab", { name: "审计" })).toBeInTheDocument();
  });

  it("preserves existing query parameters when selecting a tab", () => {
    navigation.searchParams = new URLSearchParams("q=report&view=grid");
    renderWorkbench();

    fireEvent.click(screen.getByRole("tab", { name: "传输" }));

    expect(navigation.replace).toHaveBeenCalledWith("/files?q=report&view=grid&tab=transfers");
  });

  it("connects the selected tab to its panel", () => {
    renderWorkbench();

    const filesTab = screen.getByRole("tab", { name: "文件" });
    expect(filesTab).toHaveAttribute("tabindex", "0");
    expect(filesTab).toHaveAttribute("aria-controls", "workbench-panel-files");
    expect(screen.getByRole("tabpanel")).toHaveAttribute("id", "workbench-panel-files");
    expect(screen.getByRole("tabpanel")).toHaveAttribute("aria-labelledby", "workbench-tab-files");
    expect(screen.getByRole("tab", { name: "传输" })).toHaveAttribute("tabindex", "-1");
    expect(screen.getByRole("tab", { name: "传输" })).not.toHaveAttribute("aria-controls");
  });

  it("changes only the view query parameter from the files toolbar", () => {
    navigation.searchParams = new URLSearchParams("q=report&tab=files");
    renderWorkbench();

    expect(screen.getByRole("button", { name: "列表视图" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    const gridButton = screen.getByRole("button", { name: "网格视图" });
    expect(gridButton).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(gridButton);

    expect(navigation.replace).toHaveBeenCalledWith("/files?q=report&tab=files&view=grid");
  });

  it("selects the next visible tab with ArrowRight", () => {
    renderWorkbench();

    fireEvent.keyDown(screen.getByRole("tab", { name: "文件" }), { key: "ArrowRight" });

    expect(navigation.replace).toHaveBeenCalledWith("/files?tab=transfers");
  });

  it("wraps to the previous visible tab with ArrowLeft", () => {
    renderWorkbench();

    fireEvent.keyDown(screen.getByRole("tab", { name: "文件" }), { key: "ArrowLeft" });

    expect(navigation.replace).toHaveBeenCalledWith("/files?tab=transfers");
  });
});
