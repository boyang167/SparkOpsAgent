// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Drawer } from "@/components/ui/Drawer";
import { FileBrowser } from "./FileBrowser";
import { FilesystemWorkbench } from "./FilesystemWorkbench";
import { UploadPanel } from "./UploadPanel";

const navigation = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
  refresh: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/files",
  useRouter: () => navigation,
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  abortTransfer: vi.fn(),
  createDirectoryNode: vi.fn(),
  createTransferSession: vi.fn(),
  getTransferSession: vi.fn(),
  listFileAuditEvents: vi.fn(),
  listFileNodes: vi.fn(),
  listFileRevisions: vi.fn(),
  listTransferSessions: vi.fn().mockResolvedValue({ items: [], next_cursor: "" }),
  moveFileNode: vi.fn(),
  uploadTransfer: vi.fn(),
}));

vi.mock("./GovernanceWorkspace", () => ({
  GovernanceWorkspace: () => createElement("div", null, "治理工作区"),
}));

vi.mock("./AuditWorkspace", () => ({
  AuditWorkspace: () => createElement("div", null, "审计工作区"),
}));

vi.mock("./FileDetailDrawer", () => ({
  FileDetailDrawer: () => null,
}));

vi.mock("./FileExplorerPanel", () => ({
  FileExplorerPanel: () => createElement("div", null, "文件浏览器"),
}));

vi.mock("./MoveNodesModal", () => ({
  MoveNodesModal: () => null,
}));

vi.mock("./ShareFileModal", () => ({
  ShareFileModal: () => null,
}));

const namespace = {
  id: "namespace-1",
  workspace_id: "workspace-1",
  root_node_id: "root",
  status: "ACTIVE",
};

const file: FileNodeVM = {
  id: "file-1",
  namespace_id: namespace.id,
  parent_id: namespace.root_node_id,
  type: "FILE",
  name: "报告.md",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "owner-1",
};

function contrastRatio(foreground: string, background: string) {
  function luminance(hex: string) {
    const channels = hex
      .slice(1)
      .match(/.{2}/g)!
      .map((channel) => {
        const value = Number.parseInt(channel, 16) / 255;
        return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      });
    return 0.2126 * channels[0]! + 0.7152 * channels[1]! + 0.0722 * channels[2]!;
  }
  const [lighter, darker] = [luminance(foreground), luminance(background)].sort(
    (a, b) => b - a,
  );
  return (lighter! + 0.05) / (darker! + 0.05);
}

describe("FilesystemWorkbench accessibility and responsive contracts", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("exposes named tabs, directory tree, view toggles and responsive regions", () => {
    const { container } = render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [file],
        initialState: { tab: "files", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );

    expect(screen.getByRole("tablist", { name: "文件系统功能" })).toBeInTheDocument();
    expect(screen.getByRole("tree", { name: "目录" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "切换到列表视图" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "切换到网格视图" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(screen.getByRole("complementary", { name: "目录" })).toHaveClass(
      "filesystem-directory-sidebar",
    );
    expect(screen.getByTestId("filesystem-browser-layout")).toHaveClass(
      "filesystem-browser-layout",
    );
    expect(screen.getAllByRole("tab").every((tab) =>
      tab.classList.contains("filesystem-touch-target"),
    )).toBe(true);
    expect(screen.getByRole("button", { name: "切换到列表视图" })).toHaveClass(
      "filesystem-touch-target",
    );
    expect(screen.getByRole("button", { name: "切换到网格视图" })).toHaveClass(
      "filesystem-touch-target",
    );
    expect(screen.getByRole("button", { name: "打开目录" })).toHaveClass(
      "filesystem-touch-target",
    );
    expect(screen.getByRole("button", { name: "展开 全部文件" })).toHaveClass(
      "filesystem-touch-target",
    );
    expect(container.querySelector(".filesystem-browser-main")).toHaveAttribute(
      "aria-labelledby",
      "workbench-tab-files",
    );
    expect(container.querySelector(".filesystem-browser-main")?.tagName).toBe("DIV");
    expect(container.querySelector("main")).toBeNull();
  });

  it("keeps list and grid usable without horizontal page overflow and offers keyboard move", () => {
    const onMove = vi.fn();
    const common = {
      nodes: [file],
      workspaceId: namespace.workspace_id,
      selectedIds: [] as string[],
      onSelectionChange: vi.fn(),
      onOpen: vi.fn(),
      onRename: vi.fn(),
      onMove,
      canMove: true,
    };
    const rendered = render(createElement(FileBrowser, { ...common, view: "list" }));

    expect(screen.getByRole("table").parentElement).toHaveClass("filesystem-table-wrap");
    const listCheckbox = screen.getByRole("checkbox", { name: "选择 报告.md" });
    expect(listCheckbox).toHaveClass("filesystem-selection-checkbox");
    expect(listCheckbox.parentElement).toHaveClass("filesystem-selection-target");
    fireEvent.click(listCheckbox.parentElement!);
    const selectedMove = screen.getByRole("button", { name: "移动所选 1 项到…" });
    expect(selectedMove).toHaveClass("filesystem-touch-target");
    expect(screen.getByRole("button", { name: "打开 报告.md" })).toHaveClass(
      "filesystem-touch-target",
    );
    expect(screen.getByRole("button", { name: "重命名 报告.md" })).toHaveClass(
      "filesystem-touch-target",
    );
    expect(screen.getByRole("button", { name: "移动到… 报告.md" })).toHaveClass(
      "filesystem-touch-target",
    );
    fireEvent.click(selectedMove);
    expect(onMove).toHaveBeenCalledWith([file]);

    rendered.rerender(createElement(FileBrowser, { ...common, view: "grid" }));
    expect(screen.getByRole("list", { name: "文件网格" })).toHaveClass("filesystem-file-grid");
    expect(screen.getByRole("button", { name: "打开 报告.md" })).toHaveClass(
      "filesystem-touch-target",
    );
    const gridCheckbox = screen.getByRole("checkbox", { name: "选择 报告.md" });
    expect(gridCheckbox.parentElement).toHaveClass("filesystem-selection-target");
    expect(gridCheckbox).toHaveClass("filesystem-selection-checkbox");
  });

  it("keeps exactly one primary file action while an edit panel is open", () => {
    const SelectableUploadPanel = UploadPanel as unknown as React.ComponentType<{
      onFilesSelected: (files: File[]) => void;
      emphasis: "primary" | "secondary";
    }>;
    const rendered = render(
      createElement(SelectableUploadPanel, {
        onFilesSelected: vi.fn(),
        emphasis: "primary",
      }),
    );
    expect(screen.getByRole("button", { name: "上传文件" })).toHaveClass("btn-primary");
    expect(screen.getByRole("button", { name: "上传文件" })).toHaveClass(
      "filesystem-touch-target",
    );

    rendered.rerender(
      createElement(SelectableUploadPanel, {
        onFilesSelected: vi.fn(),
        emphasis: "secondary",
      }),
    );
    expect(screen.getByRole("button", { name: "上传文件" })).toHaveClass("btn-secondary");
    expect(screen.getByRole("button", { name: "上传文件" })).not.toHaveClass("btn-primary");

    const componentDirectory = resolve(
      process.cwd(),
      "apps/web/src/modules/filesystem/components",
    );
    const explorerSource = readFileSync(
      resolve(componentDirectory, "FileExplorerPanel.tsx"),
      "utf8",
    );
    const poolSource = readFileSync(resolve(componentDirectory, "StoragePoolPanel.tsx"), "utf8");
    const bindingSource = readFileSync(
      resolve(componentDirectory, "StorageBindingPanel.tsx"),
      "utf8",
    );
    expect(explorerSource).toContain('emphasis={dialog ? "secondary" : "primary"}');
    expect(explorerSource).toMatch(
      /className="btn btn-secondary filesystem-touch-target"[\s\S]{0,180}showCreateDialog\(\)/,
    );
    expect(poolSource).toMatch(/const dialogOpen\s*=/);
    expect(poolSource).toMatch(
      /providerOpen \|\| poolOpen \|\| providerEditOpen \|\| poolEditOpen \|\| deleteTarget !== null/,
    );
    expect(poolSource).toMatch(/<PanelHeader[\s\S]*?dialogOpen=\{dialogOpen\}/);
    expect(bindingSource).toContain('className={open ? "btn btn-secondary" : "btn btn-primary"}');
  });

  it("closes a Drawer with Escape", async () => {
    const onClose = vi.fn();
    render(
      createElement(Drawer, {
        open: true,
        onClose,
        title: "目录",
        children: createElement("button", { type: "button" }, "目录项"),
      }),
    );

    await screen.findByRole("dialog", { name: "目录" });
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("defines narrow-screen, touch, focus and reduced-motion safeguards with tokens", () => {
    const css = readFileSync(resolve(process.cwd(), "apps/web/app/globals.css"), "utf8");
    const progressSources = ["TransferCenter.tsx", "TransferProgress.tsx"]
      .map((fileName) =>
        readFileSync(
          resolve(
            process.cwd(),
            "apps/web/src/modules/filesystem/components",
            fileName,
          ),
          "utf8",
        ),
      )
      .join("\n");

    expect(css).toMatch(/\.filesystem-workbench\s*\{[^}]*display:\s*grid/s);
    expect(css).toMatch(
      /\.filesystem-browser-layout\s*\{[^}]*grid-template-columns:\s*minmax\(14rem,\s*18rem\)\s+minmax\(0,\s*1fr\)/s,
    );
    expect(css).toMatch(
      /@media\s*\(max-width:\s*900px\)[\s\S]*?\.filesystem-directory-sidebar\s*\{[^}]*display:\s*none/s,
    );
    expect(css).toMatch(/\.filesystem-table-wrap\s*\{[^}]*overflow-x:\s*auto/s);
    expect(css).toMatch(
      /\.filesystem-touch-target\s*\{[^}]*min-height:\s*2\.75rem[^}]*min-width:\s*2\.75rem/s,
    );
    expect(css).toMatch(
      /\.filesystem-selection-target\s*\{[^}]*min-height:\s*2\.75rem[^}]*min-width:\s*2\.75rem/s,
    );
    expect(css).toMatch(
      /\.filesystem-selection-checkbox\s*\{[^}]*width:\s*1rem[^}]*height:\s*1rem/s,
    );
    expect(css).toMatch(
      /\.filesystem-workbench\s+:focus-visible\s*\{[^}]*outline:\s*2px solid var\(--primary\)[^}]*outline-offset:\s*2px/s,
    );
    const primary = css.match(/--primary:\s*(#[\da-f]{6})/i)?.[1];
    const surface = css.match(/--surface:\s*(#[\da-f]{6})/i)?.[1];
    expect(primary).toBeDefined();
    expect(surface).toBeDefined();
    expect(contrastRatio(primary!, surface!)).toBeGreaterThanOrEqual(4.5);
    expect(css).toMatch(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.filesystem-workbench/s,
    );
    expect(css).not.toMatch(/\.filesystem-[^{]*\{[^}]*(?:#[\da-f]{3,8}|rgba?\()/i);
    expect(progressSources).not.toContain("duration-200");
    expect(progressSources).toContain("filesystem-progress-fill");
  });
});
