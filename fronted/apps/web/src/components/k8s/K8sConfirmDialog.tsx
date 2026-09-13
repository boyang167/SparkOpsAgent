"use client";

import { Modal } from "@/components/ui/Modal";

interface K8sConfirmDialogProps {
  open: boolean;
  title: string;
  description: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  loading?: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

export function K8sConfirmDialog({
  open,
  title,
  description,
  confirmLabel = "确认",
  cancelLabel = "取消",
  danger = false,
  loading = false,
  onConfirm,
  onClose,
}: K8sConfirmDialogProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      size="md"
      description={description}
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" onClick={onClose} disabled={loading}>
            {cancelLabel}
          </button>
          <button
            type="button"
            className={danger ? "btn-primary project-service-danger" : "btn-primary"}
            onClick={onConfirm}
            disabled={loading}
          >
            {loading ? "处理中…" : confirmLabel}
          </button>
        </div>
      }
    >
      {null}
    </Modal>
  );
}
