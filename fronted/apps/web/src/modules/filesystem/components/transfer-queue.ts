export type TransferStatus =
  | "queued"
  | "creating"
  | "uploading"
  | "verifying"
  | "completed"
  | "failed"
  | "canceled";

export type TransferProgress = {
  uploadedBytes: number;
  totalBytes: number;
};

export type TransferSessionRef = {
  session_id: string;
};

export type TransferItem = {
  id: string;
  file: File;
  targetParentId: string;
  status: TransferStatus;
  progress: TransferProgress;
  session?: TransferSessionRef;
  idempotencyKey?: string;
  attempt: number;
  error?: string;
  cancelRequested?: boolean;
};

export type TransferControls = {
  signal: AbortSignal;
  session?: TransferSessionRef;
  idempotencyKey?: string;
  retrying: boolean;
  setStatus: (status: "uploading" | "verifying") => void;
  setProgress: (progress: TransferProgress) => void;
  setSession: (session: TransferSessionRef | undefined) => void;
  setIdempotencyKey: (idempotencyKey: string) => void;
};

export type TransferUpload = (
  file: File,
  targetParentId: string,
  controls: TransferControls,
) => Promise<void>;

export type TransferQueue = {
  enqueue: (files: Iterable<File>, targetParentId: string) => string[];
  cancel: (id: string) => void;
  retry: (id: string) => void;
  getSnapshot: () => readonly TransferItem[];
  subscribe: (listener: () => void) => () => void;
  whenIdle: () => Promise<void>;
  dispose: () => void;
};

type TransferQueueOptions = {
  concurrency: number;
  upload: TransferUpload;
  abort?: (
    session: TransferSessionRef,
  ) => Promise<"canceled" | "completed" | void>;
};

type ActiveTransfer = {
  controller: AbortController;
  cancelRequested: boolean;
  abortPromise?: Promise<"canceled" | "completed">;
};

function isAbortError(error: unknown) {
  return (
    typeof error === "object" &&
    error !== null &&
    "name" in error &&
    error.name === "AbortError"
  );
}

export function createTransferQueue({
  concurrency,
  upload,
  abort,
}: TransferQueueOptions): TransferQueue {
  let sequence = 0;
  let active = 0;
  let disposed = false;
  let snapshot: TransferItem[] = [];
  const listeners = new Set<() => void>();
  const activeTransfers = new Map<string, ActiveTransfer>();
  const idleResolvers = new Set<() => void>();

  function publish() {
    snapshot = [...snapshot];
    listeners.forEach((listener) => listener());
  }

  function update(id: string, change: (item: TransferItem) => TransferItem) {
    snapshot = snapshot.map((item) => (item.id === id ? change(item) : item));
    publish();
  }

  function isIdle() {
    return active === 0 && !snapshot.some((item) => item.status === "queued");
  }

  function resolveIdle() {
    if (!isIdle()) return;
    idleResolvers.forEach((resolve) => resolve());
    idleResolvers.clear();
  }

  function mutateActive(id: string, change: (item: TransferItem) => TransferItem) {
    const item = snapshot.find((candidate) => candidate.id === id);
    if (!item || ["completed", "failed", "canceled"].includes(item.status)) return;
    update(id, change);
  }

  function startAbort(
    session: TransferSessionRef,
    transfer: ActiveTransfer,
  ) {
    if (!transfer.abortPromise) {
      transfer.abortPromise = abort
        ? abort(session).then((outcome) => outcome ?? "canceled")
        : Promise.resolve("canceled");
    }
    return transfer.abortPromise;
  }

  async function finishCancellation(
    id: string,
    transfer: ActiveTransfer,
    cause?: unknown,
  ) {
    const item = snapshot.find((candidate) => candidate.id === id);
    try {
      if (!item?.session && cause && !isAbortError(cause)) throw cause;
      const outcome = item?.session
        ? await startAbort(item.session, transfer)
        : "canceled";
      update(id, (current) =>
        outcome === "completed"
          ? {
              ...current,
              status: "completed",
              cancelRequested: false,
              error: undefined,
              progress: {
                uploadedBytes: current.file.size,
                totalBytes: current.file.size,
              },
            }
          : {
              ...current,
              status: "canceled",
              cancelRequested: false,
              error: undefined,
            },
      );
    } catch (error) {
      update(id, (current) => ({
        ...current,
        status: "failed",
        cancelRequested: false,
        error: `取消失败：${error instanceof Error ? error.message : "后端状态未知"}`,
      }));
    }
  }

  async function run(item: TransferItem) {
    active += 1;
    const controller = new AbortController();
    const transfer: ActiveTransfer = { controller, cancelRequested: false };
    activeTransfers.set(item.id, transfer);
    update(item.id, (current) => ({ ...current, status: "creating", error: undefined }));
    try {
      await upload(item.file, item.targetParentId, {
        signal: controller.signal,
        session: item.session,
        idempotencyKey: item.idempotencyKey,
        retrying: item.attempt > 0,
        setStatus: (status) =>
          !transfer.cancelRequested &&
          mutateActive(item.id, (current) => ({ ...current, status })),
        setProgress: (progress) =>
          !transfer.cancelRequested &&
          mutateActive(item.id, (current) => ({ ...current, progress })),
        setSession: (session) => {
          mutateActive(item.id, (current) => ({ ...current, session }));
          if (session && transfer.cancelRequested) {
            void startAbort(session, transfer).catch(() => undefined);
          }
        },
        setIdempotencyKey: (idempotencyKey) =>
          mutateActive(item.id, (current) => ({ ...current, idempotencyKey })),
      });
      if (transfer.cancelRequested) {
        await finishCancellation(item.id, transfer);
      } else {
        mutateActive(item.id, (current) => ({
          ...current,
          status: "completed",
          progress: {
            uploadedBytes: current.file.size,
            totalBytes: current.file.size,
          },
        }));
      }
    } catch (error) {
      if (transfer.cancelRequested) {
        await finishCancellation(item.id, transfer, error);
      } else {
        mutateActive(item.id, (current) => ({
          ...current,
          status: "failed",
          error:
            isAbortError(error)
              ? "上传已中止，请重试"
              : error instanceof Error
                ? error.message
                : "上传失败",
        }));
      }
    } finally {
      active -= 1;
      activeTransfers.delete(item.id);
      pump();
      resolveIdle();
    }
  }

  function pump() {
    while (active < Math.max(1, concurrency)) {
      const next = snapshot.find((item) => item.status === "queued");
      if (!next) break;
      void run(next);
    }
  }

  return {
    enqueue(files, targetParentId) {
      if (disposed) return [];
      const created = Array.from(files, (file) => {
        sequence += 1;
        return {
          id: `transfer-${sequence}`,
          file,
          targetParentId,
          status: "queued" as const,
          attempt: 0,
          progress: { uploadedBytes: 0, totalBytes: file.size },
        };
      });
      snapshot = [...snapshot, ...created];
      publish();
      pump();
      return created.map((item) => item.id);
    },
    cancel(id) {
      const item = snapshot.find((candidate) => candidate.id === id);
      if (!item || ["completed", "failed", "canceled"].includes(item.status)) return;
      const transfer = activeTransfers.get(id);
      if (!transfer) {
        update(id, (current) => ({ ...current, status: "canceled" }));
        resolveIdle();
        return;
      }
      if (transfer.cancelRequested) return;
      transfer.cancelRequested = true;
      update(id, (current) => ({
        ...current,
        cancelRequested: true,
        error: undefined,
      }));
      transfer.controller.abort();
      if (item.session) {
        void startAbort(item.session, transfer).catch(() => undefined);
      }
    },
    retry(id) {
      if (disposed) return;
      const item = snapshot.find((candidate) => candidate.id === id);
      if (item?.status !== "failed") return;
      update(id, (current) => ({
        ...current,
        status: "queued",
        error: undefined,
        cancelRequested: false,
        attempt: current.attempt + 1,
      }));
      pump();
    },
    getSnapshot: () => snapshot,
    subscribe(listener) {
      if (disposed) return () => undefined;
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    whenIdle() {
      if (isIdle()) return Promise.resolve();
      return new Promise<void>((resolve) => idleResolvers.add(resolve));
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      for (const item of snapshot) {
        if (!["completed", "failed", "canceled"].includes(item.status)) {
          this.cancel(item.id);
        }
      }
      listeners.clear();
      resolveIdle();
    },
  };
}
