// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { createElement } from "react";
import { describe, expect, it, vi } from "vitest";
import { FilesystemWorkbench } from "./FilesystemWorkbench";

vi.mock("next/navigation", () => ({
  usePathname: () => "/files",
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
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

describe("FilesystemWorkbench", () => {
  it("selects files and hides governance without storage read permission", () => {
    render(
      createElement(FilesystemWorkbench, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        namespace,
        initialNodes: [],
        initialState: { tab: "files", view: "list", query: "" },
        permissions: ["file:read"],
      }),
    );

    expect(screen.getByRole("tab", { name: "文件" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("tab", { name: "存储治理" })).not.toBeInTheDocument();
  });
});
