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
  error?: string;
};

export type TransferControls = {
  signal: AbortSignal;
  setStatus: (status: "uploading" | "verifying") => void;
  setProgress: (progress: TransferProgress) => void;
  setSession: (session: TransferSessionRef) => void;
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
};

type TransferQueueOptions = {
  concurrency: number;
  upload: TransferUpload;
  abort?: (session: TransferSessionRef) => Promise<void>;
};

export function createTransferQueue({
  concurrency,
  upload,
  abort,
}: TransferQueueOptions): TransferQueue {
  let sequence = 0;
  let active = 0;
  let snapshot: TransferItem[] = [];
  const listeners = new Set<() => void>();
  const controllers = new Map<string, AbortController>();
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

  function mutateActive(
    id: string,
    change: (item: TransferItem) => TransferItem,
  ) {
    const item = snapshot.find((candidate) => candidate.id === id);
    if (!item || item.status === "canceled") return;
    update(id, change);
  }

  async function run(item: TransferItem) {
    active += 1;
    const controller = new AbortController();
    controllers.set(item.id, controller);
    update(item.id, (current) => ({ ...current, status: "creating", error: undefined }));
    try {
      await upload(item.file, item.targetParentId, {
        signal: controller.signal,
        setStatus: (status) =>
          mutateActive(item.id, (current) => ({ ...current, status })),
        setProgress: (progress) =>
          mutateActive(item.id, (current) => ({ ...current, progress })),
        setSession: (session) =>
          mutateActive(item.id, (current) => ({ ...current, session })),
      });
      mutateActive(item.id, (current) => ({
        ...current,
        status: "completed",
        progress: {
          uploadedBytes: current.file.size,
          totalBytes: current.file.size,
        },
      }));
    } catch (error) {
      mutateActive(item.id, (current) => ({
        ...current,
        status: "failed",
        error: error instanceof Error ? error.message : "上传失败",
      }));
    } finally {
      active -= 1;
      controllers.delete(item.id);
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
      const created = Array.from(files, (file) => {
        sequence += 1;
        return {
          id: `transfer-${sequence}`,
          file,
          targetParentId,
          status: "queued" as const,
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
      update(id, (current) => ({ ...current, status: "canceled" }));
      controllers.get(id)?.abort();
      if (item.session && abort) void abort(item.session).catch(() => undefined);
      resolveIdle();
    },
    retry(id) {
      const item = snapshot.find((candidate) => candidate.id === id);
      if (item?.status !== "failed") return;
      update(id, (current) => ({
        ...current,
        status: "queued",
        error: undefined,
        session: undefined,
        progress: { uploadedBytes: 0, totalBytes: current.file.size },
      }));
      pump();
    },
    getSnapshot: () => snapshot,
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    whenIdle() {
      if (isIdle()) return Promise.resolve();
      return new Promise<void>((resolve) => idleResolvers.add(resolve));
    },
  };
}
