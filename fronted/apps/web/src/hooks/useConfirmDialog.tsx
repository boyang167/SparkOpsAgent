"use client";

import { useState, useCallback, useRef, type ReactNode } from "react";
import { Modal } from "@/components/ui/Modal";

interface ConfirmOptions {
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

/**
 * Promise-based confirm dialog hook.
 *
 * Usage:
 * ```tsx
 * const { confirm, ConfirmDialog } = useConfirmDialog();
 *
 * async function handleDelete() {
 *   const ok = await confirm({ title: "确定删除？", danger: true });
 *   if (!ok) return;
 *   // do delete...
 * }
 *
 * return (
 *   <>
 *     ...
 *     <ConfirmDialog />
 *   </>
 * );
 * ```
 */
export function useConfirmDialog() {
  const [state, setState] = useState<{
    open: boolean;
    options: ConfirmOptions | null;
  }>({ open: false, options: null });
  const resolveRef = useRef<((v: boolean) => void) | null>(null);

  const confirm = useCallback((options: ConfirmOptions): Promise<boolean> => {
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve;
      setState({ open: true, options });
    });
  }, []);

  const close = useCallback((result: boolean) => {
    setState({ open: false, options: null });
    resolveRef.current?.(result);
    resolveRef.current = null;
  }, []);

  function ConfirmDialog() {
    if (!state.open || !state.options) return null;
    const { title, description, confirmLabel = "确定", cancelLabel = "取消", danger = false } = state.options;
    return (
      <Modal open onClose={() => close(false)} title={title} description={description} size="sm">
        <footer className="modal-footer">
          <button type="button" className="btn btn-secondary" onClick={() => close(false)}>
            {cancelLabel}
          </button>
          <button
            type="button"
            className={danger ? "btn btn-danger-solid" : "btn btn-primary"}
            onClick={() => close(true)}
          >
            {confirmLabel}
          </button>
        </footer>
      </Modal>
    );
  }

  return { confirm, ConfirmDialog } as const;
}
