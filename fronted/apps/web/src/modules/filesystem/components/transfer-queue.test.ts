import { describe, expect, it, vi } from "vitest";
import {
  createTransferQueue,
  type TransferUpload,
} from "./transfer-queue";

function files(count: number) {
  return Array.from(
    { length: count },
    (_, index) => new File([`file-${index}`], `file-${index}.txt`),
  );
}

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function abortError() {
  return new DOMException("Aborted", "AbortError");
}

function waitForAbort(signal: AbortSignal) {
  if (signal.aborted) return Promise.reject(abortError());
  return new Promise<never>((_resolve, reject) => {
    signal.addEventListener("abort", () => reject(abortError()), { once: true });
  });
}

describe("createTransferQueue", () => {
  it("runs at most three uploads", async () => {
    let active = 0;
    let peak = 0;
    const releases: Array<() => void> = [];
    const upload: TransferUpload = async () => {
      active += 1;
      peak = Math.max(peak, active);
      const gate = deferred();
      releases.push(gate.resolve);
      await gate.promise;
      active -= 1;
    };
    const queue = createTransferQueue({ concurrency: 3, upload });

    queue.enqueue(files(6), "parent-1");
    await vi.waitFor(() => expect(releases).toHaveLength(3));
    releases.splice(0).forEach((release) => release());
    await vi.waitFor(() => expect(releases).toHaveLength(3));
    releases.splice(0).forEach((release) => release());
    await queue.whenIdle();

    expect(peak).toBe(3);
    expect(queue.getSnapshot().every((item) => item.status === "completed")).toBe(true);
  });

  it("reports per-item session, state, and progress", async () => {
    const seenProgress: Array<{ uploadedBytes: number; totalBytes: number }> = [];
    const queue = createTransferQueue({
      concurrency: 3,
      upload: async (_file, _parentId, controls) => {
        controls.setSession({ session_id: "session-1" });
        controls.setStatus("uploading");
        controls.setProgress({ uploadedBytes: 4, totalBytes: 8 });
        controls.setStatus("verifying");
      },
    });
    queue.subscribe(() => {
      const progress = queue.getSnapshot()[0]?.progress;
      if (progress) seenProgress.push(progress);
    });

    queue.enqueue(files(1), "parent-1");
    await queue.whenIdle();

    expect(seenProgress).toContainEqual({ uploadedBytes: 4, totalBytes: 8 });
    expect(queue.getSnapshot()[0]).toMatchObject({
      targetParentId: "parent-1",
      status: "completed",
      progress: { uploadedBytes: 6, totalBytes: 6 },
      session: { session_id: "session-1" },
    });
  });

  it("cancels an active session and does not complete it later", async () => {
    const gate = deferred();
    const abort = vi.fn().mockResolvedValue(undefined);
    const queue = createTransferQueue({
      concurrency: 1,
      abort,
      upload: async (_file, _parentId, controls) => {
        controls.setSession({ session_id: "session-1" });
        controls.setStatus("uploading");
        await gate.promise;
      },
    });
    const [id] = queue.enqueue(files(1), "parent-1");
    await vi.waitFor(() =>
      expect(queue.getSnapshot()[0]?.status).toBe("uploading"),
    );

    queue.cancel(id!);
    await vi.waitFor(() =>
      expect(abort).toHaveBeenCalledWith(
        expect.objectContaining({ session_id: "session-1" }),
      ),
    );
    gate.resolve();
    await queue.whenIdle();

    expect(queue.getSnapshot()[0]?.status).toBe("canceled");
  });

  it("retries only failed items", async () => {
    let attempts = 0;
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async () => {
        attempts += 1;
        if (attempts === 1) throw new Error("network down");
      },
    });
    const [id] = queue.enqueue(files(1), "parent-1");
    await queue.whenIdle();
    expect(queue.getSnapshot()[0]).toMatchObject({
      status: "failed",
      error: "network down",
    });

    queue.retry(id!);
    await queue.whenIdle();
    expect(queue.getSnapshot()[0]).toMatchObject({
      status: "completed",
      error: undefined,
    });
    expect(attempts).toBe(2);
  });

  it("aborts a session returned after cancellation during creation", async () => {
    const created = deferred();
    const abort = vi.fn().mockResolvedValue("canceled");
    const queue = createTransferQueue({
      concurrency: 1,
      abort,
      upload: async (_file, _parentId, controls) => {
        await created.promise;
        controls.setSession({ session_id: "late-session" });
        await waitForAbort(controls.signal);
      },
    });
    const [id] = queue.enqueue(files(1), "parent-1");
    await vi.waitFor(() => expect(queue.getSnapshot()[0]?.status).toBe("creating"));

    queue.cancel(id!);
    created.resolve();
    await queue.whenIdle();

    expect(abort).toHaveBeenCalledWith({ session_id: "late-session" });
    expect(queue.getSnapshot()[0]).toMatchObject({
      status: "canceled",
      session: { session_id: "late-session" },
    });
  });

  it("fails cancellation when a canceled create has an unknown outcome", async () => {
    const creating = deferred();
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async () => {
        await creating.promise;
        throw new Error("create outcome unknown");
      },
    });
    const [id] = queue.enqueue(files(1), "parent-1");
    await vi.waitFor(() => expect(queue.getSnapshot()[0]?.status).toBe("creating"));

    queue.cancel(id!);
    creating.resolve();
    await queue.whenIdle();

    expect(queue.getSnapshot()[0]).toMatchObject({
      status: "failed",
      error: "取消失败：create outcome unknown",
    });
  });

  it("reports a failed cancellation when backend abort fails", async () => {
    const abort = vi.fn().mockRejectedValue(new Error("abort unavailable"));
    let attempt = 0;
    const signals: AbortSignal[] = [];
    const queue = createTransferQueue({
      concurrency: 1,
      abort,
      upload: async (_file, _parentId, controls) => {
        attempt += 1;
        signals.push(controls.signal);
        if (attempt > 1) return;
        controls.setSession({ session_id: "session-1" });
        controls.setStatus("uploading");
        await waitForAbort(controls.signal);
      },
    });
    const [id] = queue.enqueue(files(1), "parent-1");
    await vi.waitFor(() => expect(queue.getSnapshot()[0]?.status).toBe("uploading"));

    queue.cancel(id!);
    await queue.whenIdle();

    expect(queue.getSnapshot()[0]).toMatchObject({
      status: "failed",
      error: "取消失败：abort unavailable",
    });

    queue.retry(id!);
    await queue.whenIdle();
    expect(queue.getSnapshot()[0]).toMatchObject({
      status: "completed",
      session: { session_id: "session-1" },
    });
    expect(signals).toHaveLength(2);
    expect(signals[0]?.aborted).toBe(true);
    expect(signals[1]?.aborted).toBe(false);
    expect(signals[1]).not.toBe(signals[0]);
  });

  it("uses the reconciled completed state when cancel races with completion", async () => {
    const abort = vi.fn().mockResolvedValue("completed");
    const queue = createTransferQueue({
      concurrency: 1,
      abort,
      upload: async (_file, _parentId, controls) => {
        controls.setSession({ session_id: "session-1" });
        controls.setStatus("verifying");
        await waitForAbort(controls.signal);
      },
    });
    const completedSnapshots: string[] = [];
    queue.subscribe(() => {
      if (queue.getSnapshot()[0]?.status === "completed") {
        completedSnapshots.push("completed");
      }
    });
    const [id] = queue.enqueue(files(1), "parent-1");
    await vi.waitFor(() => expect(queue.getSnapshot()[0]?.status).toBe("verifying"));

    queue.cancel(id!);
    await queue.whenIdle();

    expect(queue.getSnapshot()[0]?.status).toBe("completed");
    expect(completedSnapshots).toHaveLength(1);
  });

  it("retry after polling failure preserves the old session", async () => {
    let attempt = 0;
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async (_file, _parentId, controls) => {
        attempt += 1;
        if (attempt === 1) {
          controls.setSession({ session_id: "session-1" });
        }
        controls.setStatus("verifying");
        if (attempt === 1) throw new Error("poll failed");
      },
    });
    const [id] = queue.enqueue(files(1), "parent-1");
    await queue.whenIdle();

    queue.retry(id!);
    await queue.whenIdle();

    expect(queue.getSnapshot()[0]).toMatchObject({
      status: "completed",
      session: { session_id: "session-1" },
    });
  });

  it("classifies an unexpected AbortError as retryable failure", async () => {
    const namedAbortError = new Error("Aborted");
    namedAbortError.name = "AbortError";
    const queue = createTransferQueue({
      concurrency: 1,
      upload: async () => {
        throw namedAbortError;
      },
    });

    queue.enqueue(files(1), "parent-1");
    await queue.whenIdle();

    expect(queue.getSnapshot()[0]).toMatchObject({
      status: "failed",
      error: "上传已中止，请重试",
    });
  });
});
