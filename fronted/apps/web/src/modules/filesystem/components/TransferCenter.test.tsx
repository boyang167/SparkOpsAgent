// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { TransferSessionVM } from "@sparkops/contracts/filesystem";
import { createTransferQueue } from "./transfer-queue";
import { TransferCenter, TransferSummary } from "./TransferCenter";

const listTransferSessionsMock = vi.hoisted(() => vi.fn());

vi.mock("@sparkops/web-sdk/filesystem", () => ({
  listTransferSessions: listTransferSessionsMock,
}));

describe("TransferCenter", () => {
  afterEach(() => {
    cleanup();
    listTransferSessionsMock.mockReset();
  });

  it("merges persisted sessions with local work and deduplicates by session id", async () => {
    const duplicateId = "00000000-0000-0000-0000-000000000099";
    const persisted = (sessionId: string, name: string): TransferSessionVM => ({
      session_id: sessionId,
      workspace_id: "00000000-0000-0000-0000-000000000001",
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
    });
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
    );
  });

  it("shows clear empty and transfer states with item actions", async () => {
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async () => {
        throw new Error("网络中断");
      },
    });
    const { rerender } = render(createElement(TransferCenter, { queue }));
    expect(screen.getByText("暂无传输任务")).toBeInTheDocument();

    queue.enqueue([new File(["payload"], "报告.txt")], "parent-1");
    await queue.whenIdle();
    rerender(createElement(TransferCenter, { queue }));

    expect(screen.getByText("报告.txt")).toBeInTheDocument();
    expect(screen.getByText("上传失败")).toBeInTheDocument();
    expect(screen.getByText("网络中断")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重试 报告.txt" })).toBeInTheDocument();
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
    const { rerender } = render(createElement(TransferCenter, { queue }));

    fireEvent.click(screen.getByRole("button", { name: "重试 报告.txt" }));
    await vi.waitFor(() => expect(attempts).toBe(2));
    rerender(createElement(TransferCenter, { queue }));
    fireEvent.click(screen.getByRole("button", { name: "取消 报告.txt" }));
    finish();
    await queue.whenIdle();
    rerender(createElement(TransferCenter, { queue }));

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
