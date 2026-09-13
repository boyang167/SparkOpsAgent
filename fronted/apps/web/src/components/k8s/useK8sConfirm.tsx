"use client";

import { useCallback, useRef, useState } from "react";
import { K8sConfirmDialog } from "./K8sConfirmDialog";

export type K8sConfirmOptions = {
  title?: string;
  description: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
};

type PendingConfirm = K8sConfirmOptions & {
  resolve: (confirmed: boolean) => void;
  run?: () => void | Promise<void>;
};

export function useK8sConfirm() {
  const [pending, setPending] = useState<PendingConfirm | null>(null);
  const [loading, setLoading] = useState(false);
  const pendingRef = useRef<PendingConfirm | null>(null);

  const close = useCallback((confirmed: boolean) => {
    pendingRef.current?.resolve(confirmed);
    pendingRef.current = null;
    setPending(null);
    setLoading(false);
  }, []);

  /** Returns true when user confirms; false when cancelled. */
  const ask = useCallback((options: K8sConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      const next: PendingConfirm = { ...options, resolve };
      pendingRef.current = next;
      setPending(next);
    });
  }, []);

  const onConfirm = useCallback(async () => {
    const current = pendingRef.current;
    if (!current?.run) {
      close(true);
      return;
    }
    setLoading(true);
    try {
      await current.run();
      close(true);
    } catch {
      setLoading(false);
    }
  }, [close]);

  const dialog = pending ? (
    <K8sConfirmDialog
      open
      title={pending.title ?? "确认操作"}
      description={pending.description}
      confirmLabel={pending.confirmLabel}
      cancelLabel={pending.cancelLabel}
      danger={pending.danger ?? true}
      loading={loading}
      onConfirm={onConfirm}
      onClose={() => close(false)}
    />
  ) : null;

  return { ask, dialog };
}
