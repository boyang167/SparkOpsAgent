// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createElement, StrictMode } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FilesystemWorkbench } from "./FilesystemWorkbench";

const sdk = vi.hoisted(() => ({
  moveFileNode: vi.fn(),
  abortTransfer: vi.fn(),
  createTransferSession: vi.fn(),
  getTransferSession: vi.fn(),
  listTransferSessions: vi.fn(),
  uploadTransfer: vi.fn(),
}));

const navigation = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
  refresh: vi.fn(),
  searchParams: new URLSearchParams(),
}));

const explorer = vi.hoisted(() => ({
  views: [] as string[],
  canMoves: [] as boolean[],
  canWrites: [] as boolean[],
  uploadEnabled: [] as boolean[],
  parentIds: [] as string[],
  onOpen: undefined as undefined | ((node: FileNodeVM) => void),
}));

const directoryTree = vi.hoisted(() => ({
  roots: [] as string[],
  namespaceIds: [] as string[],
  canWrites: [] as boolean[],
  onNavigate: undefined as undefined | ((node: { id: string }) => void),
  onDropNodes: undefined as
    | undefined
    | ((nodeIds: string[], target: FileNodeVM) => void),
}));

const auditWorkspace = vi.hoisted(() => ({
  modes: [] as string[],
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  moveFileNode: sdk.moveFileNode,
  abortTransfer: sdk.abortTransfer,
  createTransferSession: sdk.createTransferSession,
  getTransferSession: sdk.getTransferSession,
  listTransferSessions: sdk.listTransferSessions,
  uploadTransfer: sdk.uploadTransfer,
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/files",
  useRouter: () => ({
    replace: navigation.replace,
    push: navigation.push,
    refresh: navigation.refresh,
  }),
  useSearchParams: () => navigation.searchParams,
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
  }) =>
    open
      ? createElement("div", { role: "dialog", "aria-label": title }, children)
      : null,
}));

vi.mock("./MoveNodesModal", () => ({
  MoveNodesModal: () => null,
}));

vi.mock("./ShareFileModal", () => ({
  ShareFileModal: () => null,
}));
vi.mock("./ShareApprovalPanel", () => ({
  ShareApprovalPanel: () => createElement("div", null, "共享审批面板"),
}));

vi.mock("./GovernanceWorkspace", () => ({
  GovernanceWorkspace: () => createElement("div", null, "治理工作区"),
}));

vi.mock("./AuditWorkspace", () => ({
  AuditWorkspace: ({ mode }: { mode: string }) => {
    auditWorkspace.modes.push(mode);
    return createElement("div", null, `审计工作区 ${mode}`);
  },
}));

vi.mock("./FileDetailDrawer", () => ({
  FileDetailDrawer: ({
    open,
    node,
    loadError,
    onClose,
    onRetry,
  }: {
    open: boolean;
    node: FileNodeVM | null;
    loadError?: "forbidden" | "not-found" | "error";
    onClose: () => void;
    onRetry?: () => void;
  }) =>
    open && (node || loadError)
      ? createElement(
          "div",
          { role: "dialog", "aria-label": "文件详情" },
          node?.name ?? loadError,
          createElement("button", { type: "button", onClick: onClose }, "关闭文件详情"),
          onRetry
            ? createElement("button", { type: "button", onClick: onRetry }, "重试文件详情")
            : null,
        )
      : null,
}));

vi.mock("./FileExplorerPanel", () => ({
  FileExplorerPanel: ({
    view,
    canMove,
    canWrite,
    parentId,
    onOpen,
    onFilesSelected,
  }: {
    view: string;
    canMove: boolean;
    canWrite: boolean;
    parentId: string;
    onOpen?: (node: FileNodeVM) => void;
    onFilesSelected?: (files: File[]) => void;
  }) => {
    explorer.views.push(view);
    explorer.canMoves.push(canMove);
    explorer.canWrites.push(canWrite);
    explorer.uploadEnabled.push(Boolean(onFilesSelected));
    explorer.parentIds.push(parentId);
    explorer.onOpen = onOpen;
    return null;
  },
}));

vi.mock("./DirectoryTree", () => ({
  DirectoryTree: ({
    rootNode,
    namespaceId,
    canWrite,
    onNavigate,
    onDropNodes,
  }: {
    rootNode: { id: string };
    namespaceId: string;
    canWrite: boolean;
    onNavigate: (node: { id: string }) => void;
    onDropNodes: (nodeIds: string[], target: FileNodeVM) => void;
  }) => {
    directoryTree.roots.push(rootNode.id);
    directoryTree.namespaceIds.push(namespaceId);
    directoryTree.canWrites.push(canWrite);
    directoryTree.onNavigate = onNavigate;
    directoryTree.onDropNodes = onDropNodes;
    return createElement("div", { role: "tree" });
  },
}));

const namespace = {
  id: "00000000-0000-4000-8000-000000000002",
  workspace_id: "00000000-0000-4000-8000-000000000001",
  root_node_id: "00000000-0000-4000-8000-000000000003",
  status: "ACTIVE",
};

const namespaceB = {
  id: "00000000-0000-4000-8000-000000000022",
  workspace_id: "00000000-0000-4000-8000-000000000021",
  root_node_id: "00000000-0000-4000-8000-000000000023",
  status: "ACTIVE",
};

function namespaceSummary(
  namespaceType: "WORKSPACE" | "ORGANIZATION",
  capabilities: Array<"READ" | "WRITE">,
) {
  return {
    id: namespace.id,
    namespace_type: namespaceType,
    organization_id: "00000000-0000-4000-8000-000000000004",
    workspace_id: namespaceType === "WORKSPACE" ? namespace.workspace_id : null,
    owner_user_id: null,
    root_node_id: namespace.root_node_id,
    status: "ACTIVE",
    capabilities,
    entry_node_ids: [],
    entry_nodes_truncated: false,
  };
}

const movableNode: FileNodeVM = {
  id: "00000000-0000-4000-8000-000000000011",
  namespace_id: namespace.id,
  parent_id: namespace.root_node_id,
  type: "FILE",
  name: "报告.md",
  current_revision_id: null,
  state: "ACTIVE",
  owner_id: "00000000-0000-4000-8000-000000000012",
};

function transferSession(
  sessionId: string,
  workspaceId = namespace.workspace_id,
  namespaceId = namespace.id,
) {
  return {
    session_id: sessionId,
    workspace_id: workspaceId,
    namespace_id: namespaceId,
    status: "UPLOADING",
    protocol: "DIRECT",
    target_name: "报告.txt",
    expected_size: 7,
    part_size: 7,
    total_parts: 1,
    expires_at: new Date(Date.now() + 60_000).toISOString(),
    upload_url: "https://gateway.example/upload",
    capability_token: "token",
    parts: [],
  };
}

function renderWorkbench(
  permissions: string[] = ["file:read"],
  initialNodes: FileNodeVM[] = [],
) {
  return render(
    createElement(FilesystemWorkbench, {
      workspaceId: "00000000-0000-4000-8000-000000000001",
      namespace,
      initialNodes,
      initialState: { tab: "files", view: "list", query: "" },
      permissions,
    }),
  );
}

describe("FilesystemWorkbench", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  beforeEach(() => {
    navigation.replace.mockReset();
    navigation.push.mockReset();
    navigation.refresh.mockReset();
    sdk.moveFileNode.mockReset();
    sdk.abortTransfer.mockReset();
    sdk.createTransferSession.mockReset();
    sdk.getTransferSession.mockReset();
    sdk.listTransferSessions.mockReset();
    sdk.listTransferSessions.mockResolvedValue({ items: [], next_cursor: "" });
    sdk.uploadTransfer.mockReset();
    navigation.searchParams = new URLSearchParams();
    explorer.views = [];
    explorer.canMoves = [];
    explorer.canWrites = [];
    explorer.uploadEnabled = [];
    explorer.parentIds = [];
    explorer.onOpen = undefined;
    directoryTree.roots = [];
    directoryTree.namespaceIds = [];
    directoryTree.canWrites = [];
    directoryTree.onNavigate = undefined;
    directoryTree.onDropNodes = undefined;
    auditWorkspace.modes = [];
  });

  it("selects files and hides governance without storage read permission", () => {
    renderWorkbench();

    expect(screen.getByRole("tab", { name: "文件" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("tab", { name: "存储治理" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "共享审批" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "审计" })).not.toBeInTheDocument();
  });

  it("shows approval management only with authoritative organization permission", () => {
    renderWorkbench(["file:read", "file:admin"]);
    expect(screen.queryByRole("tab", { name: "共享审批" })).not.toBeInTheDocument();

    cleanup();
    renderWorkbench(["file:read", "org:file:admin"]);
    expect(screen.getByRole("tab", { name: "共享审批" })).toBeInTheDocument();
  });

  it("shows workspace audit for workspace file administrators", () => {
    render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "audit", view: "list", query: "" },
        permissions: ["file:read", "file:admin"],
      }),
    );

    expect(screen.getByRole("tab", { name: "审计" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "审计" })).toHaveAttribute("aria-selected", "true");
    expect(auditWorkspace.modes).toEqual(["workspace"]);
  });

  it("keeps organization owners on the URL-selected organization audit tab", () => {
    navigation.searchParams = new URLSearchParams("tab=audit");
    render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "audit", view: "list", query: "" },
        permissions: ["file:read", "org:file:admin"],
        organizationId: "00000000-0000-4000-8000-000000000004",
      }),
    );

    expect(screen.getByRole("tab", { name: "审计" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("审计工作区 organization")).toBeInTheDocument();
    expect(auditWorkspace.modes).toEqual(["organization"]);
    expect(navigation.replace).not.toHaveBeenCalled();
  });

  it("preserves existing query parameters when selecting a tab", () => {
    navigation.searchParams = new URLSearchParams("q=report&view=grid");
    renderWorkbench();

    fireEvent.click(screen.getByRole("tab", { name: "传输" }));

    expect(navigation.replace).toHaveBeenCalledWith("/files?q=report&view=grid&tab=transfers");
  });

  it("opens a file drawer from the browser and keeps selection in the URL", () => {
    navigation.searchParams = new URLSearchParams("tab=files&view=grid");
    renderWorkbench(["file:read"], [movableNode]);

    act(() => explorer.onOpen?.(movableNode));

    expect(navigation.replace).toHaveBeenCalledWith(
      `/files?tab=files&view=grid&node=${movableNode.id}`,
    );
    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent("报告.md");
    fireEvent.click(screen.getByRole("button", { name: "关闭文件详情" }));
    expect(navigation.replace).toHaveBeenLastCalledWith("/files?tab=files&view=grid");
  });

  it("restores a selected file drawer from URL-backed server data", () => {
    render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialSelectedNode: movableNode,
        initialState: { tab: "files", view: "list", query: "", node: movableNode.id },
        permissions: ["file:read"],
      }),
    );

    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent("报告.md");
  });

  it("synchronizes selected node across URL history and workspace changes without stale content", () => {
    const nodeB = {
      ...movableNode,
      id: "00000000-0000-4000-8000-000000000031",
      namespace_id: namespaceB.id,
      parent_id: namespaceB.root_node_id,
      name: "B.md",
    };
    navigation.searchParams = new URLSearchParams(`node=${movableNode.id}`);
    const rendered = render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialSelectedNode: movableNode,
        initialState: { tab: "files", view: "list", query: "", node: movableNode.id },
        permissions: ["file:read"],
      }),
    );
    expect(screen.getByText("报告.md")).toBeInTheDocument();

    navigation.searchParams = new URLSearchParams();
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespaceB.workspace_id,
        namespace: namespaceB,
        initialNodes: [],
        initialState: { tab: "files", view: "list", query: "" },
        permissions: ["file:read"],
      }),
    );
    expect(screen.queryByText("报告.md")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog", { name: "文件详情" })).not.toBeInTheDocument();

    navigation.searchParams = new URLSearchParams(`node=${nodeB.id}`);
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespaceB.workspace_id,
        namespace: namespaceB,
        initialNodes: [],
        initialSelectedNode: nodeB,
        initialState: { tab: "files", view: "list", query: "", node: nodeB.id },
        permissions: ["file:read"],
      }),
    );
    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent("B.md");
  });

  it("preserves server-confirmed parent node and cursor across history-like namespace changes", () => {
    const nodeB = {
      ...movableNode,
      id: "00000000-0000-4000-8000-000000000031",
      namespace_id: namespaceB.id,
      parent_id: namespaceB.root_node_id,
      name: "B.md",
    };
    navigation.searchParams = new URLSearchParams(
      `parent=${namespace.root_node_id}&node=${movableNode.id}&cursor=old-cursor&view=grid`,
    );
    const rendered = render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [movableNode],
        initialSelectedNode: movableNode,
        initialState: {
          tab: "files",
          view: "grid",
          query: "",
          parent: namespace.root_node_id,
          node: movableNode.id,
          cursor: "old-cursor",
        },
        permissions: ["file:read"],
      }),
    );

    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespaceB.workspace_id,
        namespace: namespaceB,
        initialNodes: [nodeB],
        initialSelectedNode: nodeB,
        initialState: {
          tab: "files",
          view: "grid",
          query: "",
          parent: namespaceB.root_node_id,
          node: nodeB.id,
          cursor: "new-cursor",
        },
        permissions: ["file:read"],
      }),
    );

    expect(explorer.parentIds.at(-1)).toBe(namespaceB.root_node_id);
    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent("B.md");
    expect(navigation.replace).not.toHaveBeenCalled();
  });

  it.each([
    ["forbidden", "forbidden"],
    ["not-found", "not-found"],
    ["error", "error"],
  ] as const)("renders selected-node %s inside the drawer and retries through refresh", (loadError, text) => {
    navigation.searchParams = new URLSearchParams(`node=${movableNode.id}`);
    render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialSelectedNodeError: loadError,
        initialState: { tab: "files", view: "list", query: "", node: movableNode.id },
        permissions: ["file:read"],
      }),
    );

    expect(screen.getByRole("dialog", { name: "文件详情" })).toHaveTextContent(text);
    fireEvent.click(screen.getByRole("button", { name: "重试文件详情" }));
    expect(navigation.refresh).toHaveBeenCalled();
  });

  it("connects the selected tab to its panel", () => {
    renderWorkbench();

    const filesTab = screen.getByRole("tab", { name: "文件" });
    const transfersTab = screen.getByRole("tab", { name: "传输" });
    expect(filesTab).toHaveAttribute("tabindex", "0");
    expect(filesTab).toHaveAttribute("aria-controls", "workbench-panel-files");
    expect(transfersTab).toHaveAttribute("tabindex", "-1");
    expect(transfersTab).toHaveAttribute("aria-controls", "workbench-panel-transfers");
    expect(document.getElementById("workbench-panel-files")).toHaveAttribute(
      "aria-labelledby",
      "workbench-tab-files",
    );
    expect(document.getElementById("workbench-panel-files")).not.toHaveAttribute("hidden");
    expect(document.getElementById("workbench-panel-transfers")).toHaveAttribute(
      "aria-labelledby",
      "workbench-tab-transfers",
    );
    expect(document.getElementById("workbench-panel-transfers")).toHaveAttribute("hidden");
  });

  it("changes only the view query parameter from the files toolbar", () => {
    navigation.searchParams = new URLSearchParams("q=report&tab=files");
    const replaceState = vi.spyOn(window.history, "replaceState");
    renderWorkbench();

    expect(screen.getByRole("button", { name: "切换到列表视图" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    const gridButton = screen.getByRole("button", { name: "切换到网格视图" });
    expect(gridButton).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(gridButton);

    expect(gridButton).toHaveAttribute("aria-pressed", "true");
    expect(explorer.views.at(-1)).toBe("grid");
    expect(replaceState).toHaveBeenCalledWith(
      null,
      "",
      "/files?q=report&tab=files&view=grid",
    );
    expect(navigation.replace).not.toHaveBeenCalled();
  });

  it("synchronizes the view when browser history changes the URL", () => {
    const rendered = renderWorkbench();
    navigation.searchParams = new URLSearchParams("view=grid");

    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "files", view: "grid", query: "" },
        permissions: ["file:read"],
      }),
    );

    expect(screen.getByRole("button", { name: "切换到网格视图" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(explorer.views.at(-1)).toBe("grid");
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

  it("reuses the directory tree in the desktop sidebar and narrow-screen drawer", () => {
    renderWorkbench();

    expect(screen.getByRole("complementary", { name: "目录" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "打开目录" }));

    expect(screen.getByRole("dialog", { name: "目录" })).toBeInTheDocument();
    expect(screen.getAllByRole("tree")).toHaveLength(2);
    expect(directoryTree.roots.every((rootId) => rootId === namespace.root_node_id)).toBe(true);
    expect(directoryTree.namespaceIds.every((id) => id === namespace.id)).toBe(true);
  });

  it("isolates equal root node ids when switching namespaces in one workspace", () => {
    const namespaceWithEqualRoot = {
      ...namespace,
      id: "00000000-0000-4000-8000-000000000099",
    };
    const rendered = renderWorkbench();

    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace: namespaceWithEqualRoot,
        initialNodes: [],
        initialState: { tab: "files", view: "list", query: "" },
        permissions: ["file:read"],
      }),
    );

    expect(directoryTree.namespaceIds.at(-1)).toBe(namespaceWithEqualRoot.id);
    expect(directoryTree.roots.at(-1)).toBe(namespace.root_node_id);
  });

  it("navigates a directory through the supported parent query", () => {
    navigation.searchParams = new URLSearchParams("cursor=old-page&q=report");
    renderWorkbench();

    directoryTree.onNavigate?.({ id: "directory-1" });

    expect(navigation.push).toHaveBeenCalledWith("/files?q=report&parent=directory-1");
  });

  it("submits backend search and navigates stable cursor pages", () => {
    navigation.searchParams = new URLSearchParams("q=report&cursor=cursor-1");
    const rendered = render(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialNextCursor: "cursor-2",
        initialState: {
          tab: "files",
          view: "list",
          query: "report",
          cursor: "cursor-1",
        },
        permissions: ["file:read"],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    expect(navigation.push).toHaveBeenCalledWith("/files?q=report&cursor=cursor-2");

    navigation.searchParams = new URLSearchParams("q=report&cursor=cursor-2");
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialNextCursor: "",
        initialState: {
          tab: "files",
          view: "list",
          query: "report",
          cursor: "cursor-2",
        },
        permissions: ["file:read"],
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "上一页" }));
    expect(navigation.push).toHaveBeenLastCalledWith(
      "/files?q=report&cursor=cursor-1",
    );

    fireEvent.change(screen.getByRole("searchbox", { name: "搜索文件" }), {
      target: { value: "design" },
    });
    fireEvent.submit(screen.getByRole("search", { name: "文件搜索" }));
    expect(navigation.push).toHaveBeenLastCalledWith("/files?q=design");
  });

  it("gates drag and keyboard move entry points with file write permission", () => {
    const { rerender } = renderWorkbench();

    expect(explorer.canMoves.at(-1)).toBe(false);
    expect(directoryTree.canWrites.every((canWrite) => !canWrite)).toBe(true);

    rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: "00000000-0000-4000-8000-000000000001",
        namespace,
        initialNodes: [],
        initialState: { tab: "files", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );

    expect(explorer.canMoves.at(-1)).toBe(true);
    expect(directoryTree.canWrites.at(-1)).toBe(true);
  });

  it.each([
    ["organization", namespaceSummary("ORGANIZATION", ["READ", "WRITE"]), false],
    ["workspace", namespaceSummary("WORKSPACE", ["READ"]), false],
    ["workspace", namespaceSummary("WORKSPACE", ["READ", "WRITE"]), true],
  ] as const)(
    "gates legacy writes for %s space with namespace capabilities",
    (fileSpace, summary, expected) => {
      render(
        createElement(FilesystemWorkbench, {
          workspaceId: namespace.workspace_id,
          namespace,
          namespaceSummary: summary,
          fileSpace,
          organizationLabel: "星火科技",
          workspaceLabel: "研发平台",
          initialNodes: [],
          initialState: { tab: "files", view: "list", query: "", space: fileSpace },
          permissions: ["file:read", "file:write"],
        }),
      );

      expect(explorer.canWrites.at(-1)).toBe(expected);
      expect(explorer.canMoves.at(-1)).toBe(expected);
      expect(explorer.uploadEnabled.at(-1)).toBe(expected);
      expect(directoryTree.canWrites.at(-1)).toBe(expected);
    },
  );

  it("executes a tree drop and refreshes the workbench", async () => {
    sdk.moveFileNode.mockResolvedValue(undefined);
    renderWorkbench(["file:read", "file:write"], [movableNode]);
    const target = {
      ...movableNode,
      id: "00000000-0000-4000-8000-000000000013",
      type: "DIRECTORY" as const,
      name: "归档",
    };

    directoryTree.onDropNodes?.([movableNode.id], target);

    await waitFor(() =>
      expect(sdk.moveFileNode).toHaveBeenCalledWith(
        "00000000-0000-4000-8000-000000000001",
        movableNode.id,
        target.id,
        movableNode.name,
      ),
    );
    expect(navigation.refresh).toHaveBeenCalled();
  });

  it("rejects a cross-namespace drop in the mutation handler", async () => {
    renderWorkbench(["file:read", "file:write"], [movableNode]);
    const target = {
      ...movableNode,
      id: "00000000-0000-4000-8000-000000000014",
      namespace_id: "00000000-0000-4000-8000-000000000099",
      type: "DIRECTORY" as const,
      name: "其他空间",
    };

    directoryTree.onDropNodes?.([movableNode.id], target);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "跨空间请使用复制、转存或采纳",
    );
    expect(sdk.moveFileNode).not.toHaveBeenCalled();
  });

  it("keeps an upload running when switching from files to transfers", async () => {
    let finishUpload!: () => void;
    sdk.createTransferSession.mockResolvedValue(
      transferSession("00000000-0000-4000-8000-000000000099"),
    );
    sdk.uploadTransfer.mockImplementation(
      async (_file: File, options: { onProgress?: (value: never) => void }) => {
        options.onProgress?.({
          uploadedBytes: 3,
          totalBytes: 7,
          completedParts: 0,
          totalParts: 1,
        } as never);
        await new Promise<void>((resolve) => {
          finishUpload = resolve;
        });
      },
    );
    sdk.getTransferSession.mockResolvedValue({ status: "COMPLETED" });
    const rendered = renderWorkbench(["file:read", "file:write"]);
    const file = new File(["payload"], "报告.txt");

    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: { files: [file], types: ["Files"], items: [] },
    });
    await waitFor(() => expect(sdk.uploadTransfer).toHaveBeenCalled());

    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );
    expect(screen.getByText("报告.txt")).toBeInTheDocument();
    expect(screen.getByText("正在上传")).toBeInTheDocument();

    finishUpload();
    await waitFor(() => expect(screen.getByText("已完成")).toBeInTheDocument());
    expect(navigation.refresh).toHaveBeenCalled();
  });

  it("scopes local queues by workspace and restores an active queue when returning", async () => {
    const sharedSessionID = "00000000-0000-4000-8000-000000000090";
    let finishA!: () => void;
    let finishB!: () => void;
    let signalA: AbortSignal | undefined;
    sdk.createTransferSession.mockImplementation(
      async (
        workspaceId: string,
        input: { namespace_id: string; name: string },
      ) =>
        transferSession(
          sharedSessionID,
          workspaceId,
          input.namespace_id,
        ),
    );
    sdk.uploadTransfer.mockImplementation(
      async (file: File, options: { signal?: AbortSignal }) => {
        if (file.name === "A.txt") {
          signalA = options.signal;
          await new Promise<void>((resolve) => {
            finishA = resolve;
          });
          return;
        }
        await new Promise<void>((resolve) => {
          finishB = resolve;
        });
      },
    );
    sdk.getTransferSession.mockResolvedValue({ status: "COMPLETED" });
    sdk.listTransferSessions.mockImplementation(async (workspaceId: string) => ({
      items:
        workspaceId === namespaceB.workspace_id
          ? [
              {
                ...transferSession(
                  sharedSessionID,
                  namespaceB.workspace_id,
                  namespaceB.id,
                ),
                target_name: "B 持久任务",
              },
            ]
          : [],
      next_cursor: "",
    }));
    const rendered = renderWorkbench(["file:read", "file:write"]);

    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["a"], "A.txt")],
        types: ["Files"],
        items: [],
      },
    });
    await waitFor(() => expect(sdk.uploadTransfer).toHaveBeenCalledTimes(1));

    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespaceB.workspace_id,
        namespace: namespaceB,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );
    expect(signalA?.aborted).toBe(false);
    expect(screen.queryByText("A.txt")).not.toBeInTheDocument();
    expect(await screen.findByText("B 持久任务")).toBeInTheDocument();

    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespaceB.workspace_id,
        namespace: namespaceB,
        initialNodes: [],
        initialState: { tab: "files", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["b"], "B.txt")],
        types: ["Files"],
        items: [],
      },
    });
    await waitFor(() => expect(sdk.createTransferSession).toHaveBeenLastCalledWith(
      namespaceB.workspace_id,
      expect.objectContaining({ namespace_id: namespaceB.id, name: "B.txt" }),
      expect.anything(),
    ));

    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );
    expect(screen.getByText("A.txt")).toBeInTheDocument();
    expect(screen.queryByText("B.txt")).not.toBeInTheDocument();
    expect(screen.getByText("正在上传")).toBeInTheDocument();

    finishA();
    finishB();
    await waitFor(() => expect(screen.getByText("已完成")).toBeInTheDocument());
  });

  it("disposes scoped queues only when the workbench unmounts", async () => {
    let uploadSignal: AbortSignal | undefined;
    sdk.createTransferSession.mockResolvedValue(
      transferSession("00000000-0000-4000-8000-000000000091"),
    );
    sdk.uploadTransfer.mockImplementation(
      async (_file: File, options: { signal?: AbortSignal }) => {
        uploadSignal = options.signal;
        await new Promise<void>(() => undefined);
      },
    );
    sdk.abortTransfer.mockResolvedValue(undefined);
    const rendered = renderWorkbench(["file:read", "file:write"]);
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["a"], "A.txt")],
        types: ["Files"],
        items: [],
      },
    });
    await waitFor(() => expect(uploadSignal).toBeDefined());

    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespaceB.workspace_id,
        namespace: namespaceB,
        initialNodes: [],
        initialState: { tab: "files", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );
    expect(uploadSignal?.aborted).toBe(false);

    rendered.unmount();
    await waitFor(() => expect(uploadSignal?.aborted).toBe(true));
    await waitFor(() =>
      expect(sdk.abortTransfer).toHaveBeenCalledWith(
        "00000000-0000-4000-8000-000000000091",
        namespace.workspace_id,
      ),
    );
  });

  it("keeps the scoped queue usable through StrictMode effect replay", async () => {
    sdk.createTransferSession.mockResolvedValue(
      transferSession("00000000-0000-4000-8000-000000000092"),
    );
    sdk.uploadTransfer.mockResolvedValue(undefined);
    sdk.getTransferSession.mockResolvedValue({ status: "COMPLETED" });

    render(
      createElement(
        StrictMode,
        null,
        createElement(FilesystemWorkbench, {
          workspaceId: namespace.workspace_id,
          namespace,
          initialNodes: [],
          initialState: { tab: "files", view: "list", query: "" },
          permissions: ["file:read", "file:write"],
        }),
      ),
    );
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["a"], "strict.txt")],
        types: ["Files"],
        items: [],
      },
    });

    await waitFor(() =>
      expect(sdk.createTransferSession).toHaveBeenCalledWith(
        namespace.workspace_id,
        expect.objectContaining({ namespace_id: namespace.id, name: "strict.txt" }),
        expect.anything(),
      ),
    );
  });

  it("aborts a late session when canceled during session creation", async () => {
    let resolveSession!: (session: ReturnType<typeof transferSession>) => void;
    let createSignal: AbortSignal | undefined;
    sdk.createTransferSession.mockImplementation(
      (
        _workspaceId: string,
        _input: unknown,
        options?: { signal?: AbortSignal },
      ) => {
        createSignal = options?.signal;
        return new Promise((resolve) => {
          resolveSession = resolve;
        });
      },
    );
    sdk.abortTransfer.mockResolvedValue(undefined);
    sdk.uploadTransfer.mockImplementation(
      async (_file: File, options: { signal?: AbortSignal }) => {
        if (options.signal?.aborted) throw new DOMException("Aborted", "AbortError");
      },
    );
    const rendered = renderWorkbench(["file:read", "file:write"]);
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["payload"], "报告.txt")],
        types: ["Files"],
        items: [],
      },
    });
    await waitFor(() => expect(sdk.createTransferSession).toHaveBeenCalled());
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "取消 报告.txt" }));
    expect(createSignal?.aborted).toBe(true);
    resolveSession(transferSession("00000000-0000-4000-8000-000000000098"));

    await waitFor(() =>
      expect(sdk.abortTransfer).toHaveBeenCalledWith(
        "00000000-0000-4000-8000-000000000098",
        namespace.workspace_id,
      ),
    );
    expect(screen.getByText("已取消")).toBeInTheDocument();
  });

  it("recovers an unknown create outcome with the same key before aborting", async () => {
    sdk.createTransferSession
      .mockImplementationOnce(
        (
          _workspaceId: string,
          _input: unknown,
          options?: { signal?: AbortSignal },
        ) =>
          new Promise((_resolve, reject) => {
            options?.signal?.addEventListener(
              "abort",
              () => reject(new Error("network outcome unknown")),
              { once: true },
            );
          }),
      )
      .mockResolvedValueOnce(
        transferSession("00000000-0000-4000-8000-000000000095"),
      );
    sdk.abortTransfer.mockResolvedValue(undefined);
    const rendered = renderWorkbench(["file:read", "file:write"]);
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["payload"], "报告.txt")],
        types: ["Files"],
        items: [],
      },
    });
    await waitFor(() => expect(sdk.createTransferSession).toHaveBeenCalledTimes(1));
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "取消 报告.txt" }));

    await waitFor(() => expect(sdk.createTransferSession).toHaveBeenCalledTimes(2));
    expect(sdk.createTransferSession.mock.calls[0]?.[2].idempotencyKey).toBe(
      sdk.createTransferSession.mock.calls[1]?.[2].idempotencyKey,
    );
    expect(sdk.createTransferSession.mock.calls[1]?.[2].signal).toBeUndefined();
    await waitFor(() =>
      expect(sdk.abortTransfer).toHaveBeenCalledWith(
        "00000000-0000-4000-8000-000000000095",
        namespace.workspace_id,
      ),
    );
  });

  it("reconciles completion when backend abort loses the cancel race", async () => {
    sdk.createTransferSession.mockResolvedValue(
      transferSession("00000000-0000-4000-8000-000000000097"),
    );
    sdk.uploadTransfer.mockImplementation(
      async (_file: File, options: { signal?: AbortSignal }) =>
        new Promise((_resolve, reject) => {
          options.signal?.addEventListener(
            "abort",
            () => reject(new DOMException("Aborted", "AbortError")),
            { once: true },
          );
        }),
    );
    sdk.abortTransfer.mockRejectedValue(new Error("already completed"));
    sdk.getTransferSession.mockResolvedValue({
      ...transferSession("00000000-0000-4000-8000-000000000097"),
      status: "COMPLETED",
    });
    const rendered = renderWorkbench(["file:read", "file:write"]);
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["payload"], "报告.txt")],
        types: ["Files"],
        items: [],
      },
    });
    await waitFor(() => expect(sdk.uploadTransfer).toHaveBeenCalled());
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "取消 报告.txt" }));

    await waitFor(() => expect(screen.getByText("已完成")).toBeInTheDocument());
    expect(sdk.getTransferSession).toHaveBeenCalledWith(
      "00000000-0000-4000-8000-000000000097",
      namespace.workspace_id,
    );
  });

  it("clears the polling timer immediately when canceled", async () => {
    vi.useFakeTimers();
    sdk.createTransferSession.mockResolvedValue(
      transferSession("00000000-0000-4000-8000-000000000096"),
    );
    sdk.uploadTransfer.mockResolvedValue(undefined);
    sdk.getTransferSession.mockResolvedValue({
      ...transferSession("00000000-0000-4000-8000-000000000096"),
      status: "VERIFYING",
    });
    sdk.abortTransfer.mockResolvedValue(undefined);
    const rendered = renderWorkbench(["file:read", "file:write"]);
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["payload"], "报告.txt")],
        types: ["Files"],
        items: [],
      },
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(sdk.getTransferSession).toHaveBeenCalled();
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "取消 报告.txt" }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(screen.getByText("已取消")).toBeInTheDocument();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("reuses session-1 after polling network failure without creating or uploading again", async () => {
    const session = transferSession("00000000-0000-4000-8000-000000000094");
    sdk.createTransferSession.mockResolvedValue(session);
    sdk.uploadTransfer.mockResolvedValue(undefined);
    sdk.getTransferSession
      .mockRejectedValueOnce(new Error("poll offline"))
      .mockResolvedValueOnce({ ...session, status: "COMPLETED" });
    const rendered = renderWorkbench(["file:read", "file:write"]);
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["payload"], "报告.txt")],
        types: ["Files"],
        items: [],
      },
    });
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "重试 报告.txt" })).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "重试 报告.txt" }));
    await waitFor(() => expect(screen.getByText("已完成")).toBeInTheDocument());

    expect(sdk.createTransferSession).toHaveBeenCalledTimes(1);
    expect(sdk.uploadTransfer).toHaveBeenCalledTimes(1);
    expect(sdk.getTransferSession).toHaveBeenLastCalledWith(
      session.session_id,
      namespace.workspace_id,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("queries the old session before retrying an abort failure", async () => {
    const session = transferSession("00000000-0000-4000-8000-000000000093");
    sdk.createTransferSession.mockResolvedValue(session);
    sdk.uploadTransfer.mockImplementationOnce(
      async (_file: File, options: { signal?: AbortSignal }) =>
        new Promise((_resolve, reject) => {
          options.signal?.addEventListener(
            "abort",
            () => reject(new DOMException("Aborted", "AbortError")),
            { once: true },
          );
        }),
    );
    sdk.abortTransfer.mockRejectedValue(new Error("abort unavailable"));
    sdk.getTransferSession
      .mockResolvedValueOnce({ ...session, status: "UPLOADING" })
      .mockResolvedValueOnce({ ...session, status: "COMPLETED" });
    const rendered = renderWorkbench(["file:read", "file:write"]);
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["payload"], "报告.txt")],
        types: ["Files"],
        items: [],
      },
    });
    await waitFor(() => expect(sdk.uploadTransfer).toHaveBeenCalledTimes(1));
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "取消 报告.txt" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "重试 报告.txt" })).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "重试 报告.txt" }));
    await waitFor(() => expect(screen.getByText("已完成")).toBeInTheDocument());

    expect(sdk.createTransferSession).toHaveBeenCalledTimes(1);
    expect(sdk.uploadTransfer).toHaveBeenCalledTimes(1);
    expect(sdk.getTransferSession).toHaveBeenNthCalledWith(
      2,
      session.session_id,
      namespace.workspace_id,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("creates session-2 only after session-1 is authoritatively terminal", async () => {
    const session1 = transferSession("00000000-0000-4000-8000-000000000092");
    const session2 = transferSession("00000000-0000-4000-8000-000000000091");
    sdk.createTransferSession
      .mockResolvedValueOnce(session1)
      .mockResolvedValueOnce(session2);
    sdk.uploadTransfer.mockResolvedValue(undefined);
    sdk.getTransferSession
      .mockRejectedValueOnce(new Error("poll offline"))
      .mockResolvedValueOnce({ ...session1, status: "FAILED" })
      .mockResolvedValueOnce({ ...session2, status: "COMPLETED" });
    const rendered = renderWorkbench(["file:read", "file:write"]);
    fireEvent.drop(screen.getByTestId("file-drop-zone"), {
      dataTransfer: {
        files: [new File(["payload"], "报告.txt")],
        types: ["Files"],
        items: [],
      },
    });
    rendered.rerender(
      createElement(FilesystemWorkbench, {
        workspaceId: namespace.workspace_id,
        namespace,
        initialNodes: [],
        initialState: { tab: "transfers", view: "list", query: "" },
        permissions: ["file:read", "file:write"],
      }),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "重试 报告.txt" })).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "重试 报告.txt" }));
    await waitFor(() => expect(screen.getByText("已完成")).toBeInTheDocument());

    expect(sdk.createTransferSession).toHaveBeenCalledTimes(2);
    expect(sdk.uploadTransfer).toHaveBeenCalledTimes(2);
    expect(sdk.createTransferSession.mock.calls[0]?.[2].idempotencyKey).not.toBe(
      sdk.createTransferSession.mock.calls[1]?.[2].idempotencyKey,
    );
  });
});
