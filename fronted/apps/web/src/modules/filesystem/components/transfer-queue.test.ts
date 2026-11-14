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
});
