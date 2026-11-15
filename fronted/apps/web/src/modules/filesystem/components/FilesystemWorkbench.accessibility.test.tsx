// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createElement } from "react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Drawer } from "@/components/ui/Drawer";
import { FileBrowser } from "./FileBrowser";
import { FileDetailDrawer } from "./FileDetailDrawer";
import { FileSpaceTabList } from "./FileSpaceTabList";
import { FilesystemWorkbench } from "./FilesystemWorkbench";
import { ShareApprovalPanel } from "./ShareApprovalPanel";
import { ShareFileModal } from "./ShareFileModal";
import { SharedWithMePanel } from "./SharedWithMePanel";
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

vi.mock("@/components/ui/InlineAlert", () => ({
  InlineAlert: ({ tone, children }: { tone: string; children: React.ReactNode }) =>
    createElement("div", { className: `inline-alert inline-alert-${tone}` }, children),
}));

vi.mock("@/components/ui/Modal", () => ({
  Modal: ({
    open,
    title,
    children,
    footer,
  }: {
    open: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    open
      ? createElement(
          "div",
          { role: "dialog", "aria-label": title, "aria-modal": "true", tabIndex: -1, autoFocus: true },
          children,
          footer,
        )
      : null,
}));

const sdk = vi.hoisted(() => ({
  abortTransfer: vi.fn(),
  approveShareRequest: vi.fn(),
  createDirectoryNode: vi.fn(),
  createGovernedGrant: vi.fn(),
  createShareRequest: vi.fn(),
  createTransferSession: vi.fn(),
  getEffectiveAccess: vi.fn(),
  getTransferSession: vi.fn(),
  listFileAuditEvents: vi.fn(),
  listFileNodes: vi.fn(),
  listFileRevisions: vi.fn(),
  listGovernedGrants: vi.fn(),
  listOrganizationShareRequests: vi.fn(),
  listShareRequests: vi.fn(),
  listTransferSessions: vi.fn().mockResolvedValue({ items: [], next_cursor: "" }),
  moveFileNode: vi.fn(),
  previewGovernedShare: vi.fn(),
  rejectShareRequest: vi.fn(),
  uploadTransfer: vi.fn(),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => sdk);

vi.mock("./GovernanceWorkspace", () => ({
  GovernanceWorkspace: () => createElement("div", null, "治理工作区"),
}));

vi.mock("./AuditWorkspace", () => ({
  AuditWorkspace: ({ mode }: { mode: string }) =>
    createElement("div", { "data-testid": "audit-mode" }, `审计工作区 ${mode}`),
}));

vi.mock("./FileExplorerPanel", () => ({
  FileExplorerPanel: () => createElement("div", null, "文件浏览器"),
}));

vi.mock("./MoveNodesModal", () => ({
  MoveNodesModal: () => null,
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

  it("exposes organization audit as the selected accessible tab for tenant owners", () => {
    render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "audit", view: "list", query: "" },
        permissions: ["file:read", "org:file:admin"],
        organizationId: "organization-1",
      }),
    );

    const auditTab = screen.getByRole("tab", { name: "审计" });
    expect(auditTab).toHaveAttribute("aria-selected", "true");
    expect(auditTab).toHaveAttribute("aria-controls", "workbench-panel-audit");
    expect(screen.getByRole("tabpanel")).toHaveAttribute(
      "aria-labelledby",
      "workbench-tab-audit",
    );
    expect(screen.getByTestId("audit-mode")).toHaveTextContent("organization");
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
    const uploadTrigger = () => screen.getByText("上传文件").closest("label");
    expect(uploadTrigger()).toHaveClass("btn-primary");
    expect(uploadTrigger()).toHaveClass("filesystem-touch-target");

    rendered.rerender(
      createElement(SelectableUploadPanel, {
        onFilesSelected: vi.fn(),
        emphasis: "secondary",
      }),
    );
    expect(uploadTrigger()).toHaveClass("btn-secondary");
    expect(uploadTrigger()).not.toHaveClass("btn-primary");

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

  it("renders all four file spaces and shared panel states with accessible landmarks", () => {
    const rendered = render(
      createElement(FileSpaceTabList, {
        activeSpace: "shared",
        view: "list",
      }),
    );
    const tablist = screen.getByRole("tablist", { name: "文件空间" });
    expect(tablist).toBeInTheDocument();
    expect(screen.getAllByRole("tab")).toHaveLength(4);
    expect(screen.getByRole("tab", { name: "与我共享" })).toHaveAttribute(
      "aria-current",
      "page",
    );

    rendered.rerender(
      createElement(SharedWithMePanel, {
        workspaceId: "workspace-1",
        organizationId: "organization-1",
        organizationLabel: "星火科技",
        workspaceLabel: "研发平台",
        items: [],
        state: "loading",
      }),
    );
    expect(screen.getByRole("status")).toHaveTextContent("正在加载共享内容");
    rendered.rerender(
      createElement(SharedWithMePanel, {
        workspaceId: "workspace-1",
        organizationId: "organization-1",
        organizationLabel: "星火科技",
        workspaceLabel: "研发平台",
        items: [],
        state: "error",
      }),
    );
    expect(screen.getByRole("alert")).toHaveTextContent("共享内容加载失败");
  });

  it("renders the two-step share modal with labelled controls and managed focus", async () => {
    const user = userEvent.setup();
    sdk.previewGovernedShare.mockResolvedValue({
      outcome: "ACTIVE",
      reason: "POLICY_ACTIVE",
    });
    render(
      createElement(ShareFileModal, {
        open: true,
        node: { ...file, type: "DIRECTORY", classification: "INTERNAL" },
        organizationId: "00000000-0000-4000-8000-000000000101",
        organizationLabel: "星火组织",
        workspaceId: "00000000-0000-4000-8000-000000000102",
        workspaceLabel: "研发空间",
        members: [{
          member_id: "00000000-0000-4000-8000-000000000103",
          email: "member@example.com",
          display_name: "王小明",
          roles: [],
        }],
        effectiveActions: ["SHARE"],
        onClose: vi.fn(),
      }),
    );

    const dialog = await screen.findByRole("dialog", { name: "共享“报告.md”" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    await user.click(screen.getByRole("radio", { name: /王小明/ }));
    await user.click(screen.getByRole("button", { name: "下一步" }));
    expect(screen.getByRole("checkbox", { name: "READ" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /应用到子项/ })).toBeChecked();
    expect(await screen.findByRole("button", { name: "立即共享" })).toBeInTheDocument();
  });

  it("renders approval loading, actions and a labelled rejection modal", async () => {
    const user = userEvent.setup();
    sdk.listOrganizationShareRequests.mockResolvedValue({
      items: [{
        id: "00000000-0000-4000-8000-000000000011",
        organization_id: "00000000-0000-4000-8000-000000000001",
        namespace_id: "00000000-0000-4000-8000-000000000012",
        node_id: "00000000-0000-4000-8000-000000000013",
        requested_by: "00000000-0000-4000-8000-000000000014",
        principal_type: "ORGANIZATION",
        principal_ids: ["00000000-0000-4000-8000-000000000001"],
        actions: ["READ"],
        classification: "INTERNAL",
        effect: "ALLOW",
        status: "PENDING",
        created_at: "2026-11-15T00:00:00Z",
      }],
      next_cursor: "",
      truncated: false,
      total_count: 1,
    });
    render(
      createElement(ShareApprovalPanel, {
        organizationId: "00000000-0000-4000-8000-000000000001",
        workspaceId: "00000000-0000-4000-8000-000000000002",
      }),
    );

    expect(screen.getByRole("status")).toHaveTextContent("正在加载待审批申请");
    expect(await screen.findByText("待审批 1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "批准共享申请" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "拒绝共享申请" }));
    const rejectionDialog = await screen.findByRole("dialog", { name: "拒绝共享申请" });
    expect(rejectionDialog).toHaveAttribute("aria-modal", "true");
    expect(screen.getByLabelText("拒绝原因")).toHaveAttribute("maxLength", "1000");
  });

  it("renders drawer tabs, loading and error states with dialog focus semantics", async () => {
    const user = userEvent.setup();
    sdk.listFileRevisions.mockRejectedValue(new Error("network"));
    sdk.listFileAuditEvents.mockResolvedValue({ items: [], next_cursor: "" });
    sdk.getEffectiveAccess.mockResolvedValue({
      decision: "ALLOW",
      effective_actions: ["READ"],
      reason_code: "owner",
      source_grant_ids: [],
      policy_version: 1,
      cache_key: {
        organization_id: "organization-1",
        workspace_id: "workspace-1",
        actor_id: "owner-1",
        node_id: file.id,
        action: "READ",
        policy_version: 1,
      },
    });
    sdk.listGovernedGrants.mockResolvedValue({
      items: [],
      next_cursor: "",
      truncated: false,
    });
    sdk.listShareRequests.mockResolvedValue({
      items: [],
      next_cursor: "",
      truncated: false,
    });
    render(
      createElement(FileDetailDrawer, {
        open: true,
        node: file,
        workspaceId: "workspace-1",
        organizationId: "organization-1",
        onClose: vi.fn(),
      }),
    );

    const dialog = await screen.findByRole("dialog", { name: "报告.md" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    await waitFor(() => expect(dialog).toContainElement(document.activeElement as HTMLElement));
    expect(screen.getByRole("tablist", { name: "文件详情" })).toBeInTheDocument();
    expect(screen.getAllByRole("tab")).toHaveLength(4);
    await user.click(screen.getByRole("tab", { name: "版本" }));
    expect(screen.getByRole("tabpanel")).toHaveAttribute(
      "aria-labelledby",
      expect.stringContaining("revisions"),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("加载版本失败");
    expect(screen.getByRole("button", { name: "重试加载版本" })).toBeInTheDocument();
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
