// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createElement } from "react";
import type { FileNodeVM } from "@sparkops/contracts/filesystem";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FilesystemWorkbench } from "./FilesystemWorkbench";

const sdk = vi.hoisted(() => ({
  moveFileNode: vi.fn(),
  abortTransfer: vi.fn(),
  createTransferSession: vi.fn(),
  getTransferSession: vi.fn(),
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
}));

const directoryTree = vi.hoisted(() => ({
  roots: [] as string[],
  canWrites: [] as boolean[],
  onNavigate: undefined as undefined | ((node: { id: string }) => void),
  onDropNodes: undefined as
    | undefined
    | ((nodeIds: string[], target: FileNodeVM) => void),
}));

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  moveFileNode: sdk.moveFileNode,
  abortTransfer: sdk.abortTransfer,
  createTransferSession: sdk.createTransferSession,
  getTransferSession: sdk.getTransferSession,
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

vi.mock("./FileExplorerPanel", () => ({
  FileExplorerPanel: ({ view, canMove }: { view: string; canMove: boolean }) => {
    explorer.views.push(view);
    explorer.canMoves.push(canMove);
    return null;
  },
}));

vi.mock("./DirectoryTree", () => ({
  DirectoryTree: ({
    root,
    canWrite,
    onNavigate,
    onDropNodes,
  }: {
    root: { id: string };
    canWrite: boolean;
    onNavigate: (node: { id: string }) => void;
    onDropNodes: (nodeIds: string[], target: FileNodeVM) => void;
  }) => {
    directoryTree.roots.push(root.id);
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

function transferSession(sessionId: string) {
  return {
    session_id: sessionId,
    workspace_id: namespace.workspace_id,
    namespace_id: namespace.id,
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
    sdk.uploadTransfer.mockReset();
    navigation.searchParams = new URLSearchParams();
    explorer.views = [];
    explorer.canMoves = [];
    directoryTree.roots = [];
    directoryTree.canWrites = [];
    directoryTree.onNavigate = undefined;
    directoryTree.onDropNodes = undefined;
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

    expect(screen.getByRole("button", { name: "列表视图" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    const gridButton = screen.getByRole("button", { name: "网格视图" });
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
  });

  it("navigates a directory through the supported parent query", () => {
    renderWorkbench();

    directoryTree.onNavigate?.({ id: "directory-1" });

    expect(navigation.push).toHaveBeenCalledWith("/files?parent=directory-1");
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
});
