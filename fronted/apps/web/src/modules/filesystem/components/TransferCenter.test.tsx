// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { TransferSessionVM } from "@sparkops/contracts/filesystem";
import { createTransferQueue } from "./transfer-queue";
import { TransferCenter, TransferSummary } from "./TransferCenter";

const listTransferSessionsMock = vi.hoisted(() => vi.fn());
const abortTransferMock = vi.hoisted(() => vi.fn());

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  listTransferSessions: listTransferSessionsMock,
  abortTransfer: abortTransferMock,
}));

function persisted(
  sessionId: string,
  name: string,
  workspaceId = "00000000-0000-0000-0000-000000000001",
): TransferSessionVM {
  return {
    session_id: sessionId,
    workspace_id: workspaceId,
    namespace_id: "00000000-0000-0000-0000-000000000002",
    target_parent_id: "00000000-0000-0000-0000-000000000003",
    target_name: name,
    status: "FAILED",
    protocol: "DIRECT",
    expected_size: 7,
    part_size: 7,
    total_parts: 1,
    expires_at: new Date(Date.now() + 3_600_000).toISOString(),
    parts: [],
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function emptyQueue() {
  return createTransferQueue({
    concurrency: 1,
    upload: async () => undefined,
  });
}

describe("TransferCenter", () => {
  afterEach(() => {
    cleanup();
    listTransferSessionsMock.mockReset();
    abortTransferMock.mockReset();
  });

  it("aborts an active persisted session through the backend", async () => {
    const active = {
      ...persisted("00000000-0000-0000-0000-000000000030", "活动任务.bin"),
      status: "UPLOADING" as const,
    };
    listTransferSessionsMock
      .mockResolvedValueOnce({ items: [active], next_cursor: "" })
      .mockResolvedValueOnce({ items: [], next_cursor: "" });
    abortTransferMock.mockResolvedValue(undefined);
    render(
      createElement(TransferCenter, {
        queue: emptyQueue(),
        workspaceId: active.workspace_id,
        canWrite: true,
      }),
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "取消 活动任务.bin" }),
    );

    await vi.waitFor(() =>
      expect(abortTransferMock).toHaveBeenCalledWith(
        active.session_id,
        active.workspace_id,
      ),
    );
    expect(listTransferSessionsMock).toHaveBeenCalledTimes(2);
  });

  it("requires a matching file before creating a new transfer for a failed persisted task", async () => {
    const failed = persisted(
      "00000000-0000-0000-0000-000000000031",
      "失败任务.bin",
    );
    listTransferSessionsMock.mockResolvedValue({
      items: [failed],
      next_cursor: "",
    });
    const queue = emptyQueue();
    const enqueue = vi.spyOn(queue, "enqueue");
    render(
      createElement(TransferCenter, {
        queue,
        workspaceId: failed.workspace_id,
        canWrite: true,
      }),
    );

    expect(
      await screen.findByText(/重新选择匹配文件后将新建传输/),
    ).toBeInTheDocument();
    const input = screen.getByLabelText("重新选择并新建 失败任务.bin");
    fireEvent.change(input, {
      target: { files: [new File(["wrong"], "其他.bin")] },
    });
    expect(enqueue).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("文件名称和大小必须与失败任务一致");

    fireEvent.change(input, {
      target: { files: [new File(["1234567"], "失败任务.bin")] },
    });
    expect(enqueue).toHaveBeenCalledWith(
      expect.arrayContaining([expect.any(File)]),
      failed.target_parent_id,
    );
  });

  it("hides persisted and local mutations from a read-only user", async () => {
    const active = {
      ...persisted("00000000-0000-0000-0000-000000000032", "只读活动.bin"),
      status: "UPLOADING" as const,
    };
    const failed = persisted(
      "00000000-0000-0000-0000-000000000033",
      "只读失败.bin",
    );
    listTransferSessionsMock.mockResolvedValue({
      items: [active, failed],
      next_cursor: "",
    });
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async () => {
        throw new Error("failed");
      },
    });
    queue.enqueue([new File(["local"], "本地失败.bin")], "parent-1");
    await queue.whenIdle();

    render(
      createElement(TransferCenter, {
        queue,
        workspaceId: active.workspace_id,
        canWrite: false,
      }),
    );

    expect(await screen.findByText("只读活动.bin")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "取消 只读活动.bin" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("重新选择并新建 只读失败.bin")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重试 本地失败.bin" })).not.toBeInTheDocument();
  });

  it("merges persisted sessions with local work and deduplicates by session id", async () => {
    const duplicateId = "00000000-0000-0000-0000-000000000099";
    listTransferSessionsMock.mockResolvedValue({
      items: [
        persisted(duplicateId, "远端重复.txt"),
        persisted("00000000-0000-0000-0000-000000000100", "历史任务.txt"),
      ],
      next_cursor: "",
    });
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async (_file, _parent, controls) => {
        controls.setSession({ session_id: duplicateId });
        await new Promise<void>(() => undefined);
      },
    });
    queue.enqueue([new File(["payload"], "本地任务.txt")], "parent-1");
    await vi.waitFor(() =>
      expect(queue.getSnapshot()[0]?.session?.session_id).toBe(duplicateId),
    );

    render(
      createElement(TransferCenter, {
        queue,
        workspaceId: "00000000-0000-0000-0000-000000000001",
      }),
    );

    expect(await screen.findByText("历史任务.txt")).toBeInTheDocument();
    expect(screen.getByText("本地任务.txt")).toBeInTheDocument();
    expect(screen.queryByText("远端重复.txt")).not.toBeInTheDocument();
    expect(listTransferSessionsMock).toHaveBeenCalledWith(
      "00000000-0000-0000-0000-000000000001",
      { limit: 100 },
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("hides the old workspace immediately and ignores its late first page", async () => {
    const workspaceA = "00000000-0000-0000-0000-000000000001";
    const workspaceB = "00000000-0000-0000-0000-000000000010";
    const firstA = deferred<{ items: TransferSessionVM[]; next_cursor: string }>();
    listTransferSessionsMock.mockImplementation((workspaceId: string) => {
      if (workspaceId === workspaceA) return firstA.promise;
      return Promise.resolve({
        items: [persisted("00000000-0000-0000-0000-000000000011", "B 任务", workspaceB)],
        next_cursor: "",
      });
    });
    const queue = emptyQueue();
    const { rerender } = render(
      createElement(TransferCenter, { queue, workspaceId: workspaceA }),
    );
    expect(screen.getByRole("status")).toHaveTextContent("正在加载持久传输任务");

    rerender(createElement(TransferCenter, { queue, workspaceId: workspaceB }));
    expect(screen.queryByText("A 任务")).not.toBeInTheDocument();
    expect(await screen.findByText("B 任务")).toBeInTheDocument();
    const firstSignal = listTransferSessionsMock.mock.calls[0]?.[2]?.signal as AbortSignal;
    expect(firstSignal.aborted).toBe(true);

    firstA.resolve({
      items: [persisted("00000000-0000-0000-0000-000000000012", "A 任务", workspaceA)],
      next_cursor: "stale-a",
    });
    await Promise.resolve();
    expect(screen.queryByText("A 任务")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "加载更多" })).not.toBeInTheDocument();
  });

  it("keeps only the newest request during a rapid A to B to A switch", async () => {
    const workspaceA = "00000000-0000-0000-0000-000000000001";
    const workspaceB = "00000000-0000-0000-0000-000000000010";
    const oldA = deferred<{ items: TransferSessionVM[]; next_cursor: string }>();
    const latestA = deferred<{ items: TransferSessionVM[]; next_cursor: string }>();
    let aCalls = 0;
    listTransferSessionsMock.mockImplementation((workspaceId: string) => {
      if (workspaceId === workspaceB) {
        return Promise.resolve({ items: [], next_cursor: "" });
      }
      aCalls += 1;
      return aCalls === 1 ? oldA.promise : latestA.promise;
    });
    const queue = emptyQueue();
    const { rerender } = render(
      createElement(TransferCenter, { queue, workspaceId: workspaceA }),
    );
    rerender(createElement(TransferCenter, { queue, workspaceId: workspaceB }));
    rerender(createElement(TransferCenter, { queue, workspaceId: workspaceA }));

    latestA.resolve({
      items: [persisted("00000000-0000-0000-0000-000000000013", "最新 A")],
      next_cursor: "",
    });
    expect(await screen.findByText("最新 A")).toBeInTheDocument();
    oldA.resolve({
      items: [persisted("00000000-0000-0000-0000-000000000014", "过期 A")],
      next_cursor: "",
    });
    await Promise.resolve();
    expect(screen.queryByText("过期 A")).not.toBeInTheDocument();
  });

  it("ignores a late page after switching workspace", async () => {
    const workspaceA = "00000000-0000-0000-0000-000000000001";
    const workspaceB = "00000000-0000-0000-0000-000000000010";
    const oldPage = deferred<{ items: TransferSessionVM[]; next_cursor: string }>();
    listTransferSessionsMock.mockImplementation(
      (workspaceId: string, query: { cursor?: string }) => {
        if (workspaceId === workspaceB) {
          return Promise.resolve({
            items: [persisted("00000000-0000-0000-0000-000000000015", "B 当前", workspaceB)],
            next_cursor: "",
          });
        }
        if (query.cursor) return oldPage.promise;
        return Promise.resolve({
          items: [persisted("00000000-0000-0000-0000-000000000016", "A 首屏")],
          next_cursor: "a-next",
        });
      },
    );
    const queue = emptyQueue();
    const { rerender } = render(
      createElement(TransferCenter, { queue, workspaceId: workspaceA }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "加载更多" }));
    rerender(createElement(TransferCenter, { queue, workspaceId: workspaceB }));
    expect(await screen.findByText("B 当前")).toBeInTheDocument();

    oldPage.resolve({
      items: [persisted("00000000-0000-0000-0000-000000000017", "A 迟到分页")],
      next_cursor: "",
    });
    await Promise.resolve();
    expect(screen.queryByText("A 迟到分页")).not.toBeInTheDocument();
  });

  it("deduplicates repeated pages and blocks concurrent load more", async () => {
    const page = deferred<{ items: TransferSessionVM[]; next_cursor: string }>();
    listTransferSessionsMock
      .mockResolvedValueOnce({
        items: [persisted("00000000-0000-0000-0000-000000000018", "任务一")],
        next_cursor: "next",
      })
      .mockReturnValueOnce(page.promise);
    const queue = emptyQueue();
    render(
      createElement(TransferCenter, {
        queue,
        workspaceId: "00000000-0000-0000-0000-000000000001",
      }),
    );
    const loadMore = await screen.findByRole("button", { name: "加载更多" });
    fireEvent.click(loadMore);
    fireEvent.click(loadMore);
    expect(listTransferSessionsMock).toHaveBeenCalledTimes(2);

    page.resolve({
      items: [
        persisted("00000000-0000-0000-0000-000000000018", "任务一重复"),
        persisted("00000000-0000-0000-0000-000000000019", "任务二"),
        persisted("00000000-0000-0000-0000-000000000019", "任务二重复"),
      ],
      next_cursor: "",
    });
    expect(await screen.findByText("任务二")).toBeInTheDocument();
    expect(screen.getAllByRole("row")).toHaveLength(3);
    expect(screen.queryByText("任务一重复")).not.toBeInTheDocument();
    expect(screen.queryByText("任务二重复")).not.toBeInTheDocument();
  });

  it("retries errors and refreshes while retaining current workspace data", async () => {
    const retry = deferred<{ items: TransferSessionVM[]; next_cursor: string }>();
    const refresh = deferred<{ items: TransferSessionVM[]; next_cursor: string }>();
    listTransferSessionsMock
      .mockRejectedValueOnce(new Error("offline"))
      .mockReturnValueOnce(retry.promise)
      .mockReturnValueOnce(refresh.promise);
    const queue = emptyQueue();
    render(
      createElement(TransferCenter, {
        queue,
        workspaceId: "00000000-0000-0000-0000-000000000001",
      }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("无法加载持久传输任务");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    retry.resolve({
      items: [persisted("00000000-0000-0000-0000-000000000020", "旧状态")],
      next_cursor: "",
    });
    expect(await screen.findByText("旧状态")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "刷新" }));
    expect(screen.getByText("旧状态")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("正在刷新持久传输任务");
    refresh.resolve({
      items: [persisted("00000000-0000-0000-0000-000000000021", "新状态")],
      next_cursor: "",
    });
    expect(await screen.findByText("新状态")).toBeInTheDocument();
    expect(screen.queryByText("旧状态")).not.toBeInTheDocument();
  });

  it("aborts the active request when unmounted", () => {
    listTransferSessionsMock.mockReturnValue(new Promise(() => undefined));
    const { unmount } = render(
      createElement(TransferCenter, {
        queue: emptyQueue(),
        workspaceId: "00000000-0000-0000-0000-000000000001",
      }),
    );
    const signal = listTransferSessionsMock.mock.calls[0]?.[2]?.signal as AbortSignal;
    expect(signal.aborted).toBe(false);
    unmount();
    expect(signal.aborted).toBe(true);
  });

  it("shows clear empty and transfer states with item actions", async () => {
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async () => {
        throw new Error("网络中断");
      },
    });
    const { rerender } = render(
      createElement(TransferCenter, { queue, canWrite: true }),
    );
    expect(screen.getByText("暂无传输任务")).toBeInTheDocument();

    queue.enqueue([new File(["payload"], "报告.txt")], "parent-1");
    await queue.whenIdle();
    rerender(createElement(TransferCenter, { queue, canWrite: true }));

    expect(screen.getByText("报告.txt")).toBeInTheDocument();
    expect(screen.getByText("上传失败")).toBeInTheDocument();
    expect(screen.getByText("网络中断")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重试 报告.txt" })).toBeInTheDocument();
  });

  it("offers an explicit refresh after an empty first page", async () => {
    listTransferSessionsMock.mockResolvedValue({ items: [], next_cursor: "" });
    render(
      createElement(TransferCenter, {
        queue: emptyQueue(),
        workspaceId: "00000000-0000-0000-0000-000000000001",
      }),
    );

    expect(await screen.findByText("暂无传输任务")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "刷新" })).toBeInTheDocument();
  });

  it("wires retry and cancel actions to the queue", async () => {
    let finish!: () => void;
    let attempts = 0;
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async (_file, _parent, controls) => {
        attempts += 1;
        if (attempts === 1) throw new Error("失败");
        controls.setStatus("uploading");
        await new Promise<void>((resolve) => {
          finish = resolve;
        });
      },
    });
    queue.enqueue([new File(["payload"], "报告.txt")], "parent-1");
    await queue.whenIdle();
    const { rerender } = render(
      createElement(TransferCenter, { queue, canWrite: true }),
    );

    fireEvent.click(screen.getByRole("button", { name: "重试 报告.txt" }));
    await vi.waitFor(() => expect(attempts).toBe(2));
    rerender(createElement(TransferCenter, { queue, canWrite: true }));
    fireEvent.click(screen.getByRole("button", { name: "取消 报告.txt" }));
    finish();
    await queue.whenIdle();
    rerender(createElement(TransferCenter, { queue, canWrite: true }));

    expect(screen.getByText("已取消")).toBeInTheDocument();
  });

  it("limits the file-tab summary to three active transfers", async () => {
    const queue = createTransferQueue({
      concurrency: 3,
      upload: async () => new Promise<void>(() => undefined),
    });
    queue.enqueue(
      Array.from({ length: 5 }, (_, index) => new File(["x"], `${index}.txt`)),
      "parent-1",
    );

    render(createElement(TransferSummary, { queue, onViewAll: vi.fn() }));

    expect(screen.getAllByRole("progressbar")).toHaveLength(3);
    expect(screen.getByRole("button", { name: "查看全部传输" })).toBeInTheDocument();
  });

  it("keeps verifying work visibly below completion", async () => {
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async (_file, _parent, controls) => {
        controls.setProgress({ uploadedBytes: 7, totalBytes: 7 });
        controls.setStatus("verifying");
        await new Promise<void>(() => undefined);
      },
    });
    queue.enqueue([new File(["payload"], "报告.txt")], "parent-1");
    await vi.waitFor(() => expect(queue.getSnapshot()[0]?.status).toBe("verifying"));
    const { unmount } = render(createElement(TransferCenter, { queue }));

    expect(screen.getByText("正在校验")).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "99");
    unmount();
    render(createElement(TransferSummary, { queue, onViewAll: vi.fn() }));
    expect(screen.getByText("99%")).toBeInTheDocument();
  });
});
