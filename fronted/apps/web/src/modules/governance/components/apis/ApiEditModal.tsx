"use client";

import type { RegisteredAPI } from "@sparkops/contracts/governance";
import { Modal } from "@/components/ui/Modal";

interface ApiEditModalProps {
  open: boolean;
  api: RegisteredAPI | null;
  apiGroup: string;
  description: string;
  saving: boolean;
  onClose: () => void;
  onChangeGroup: (value: string) => void;
  onChangeDescription: (value: string) => void;
  onSubmit: (e: React.FormEvent) => void;
}

export function ApiEditModal({
  open,
  api,
  apiGroup,
  description,
  saving,
  onClose,
  onChangeGroup,
  onChangeDescription,
  onSubmit,
}: ApiEditModalProps) {
  return (
    <Modal open={open} onClose={onClose} title="编辑 API">
      <form onSubmit={onSubmit} className="stack-md">
        <div className="grid gap-3">
          <label className="stack-xs text-sm">
            <span className="text-muted">请求方式</span>
            <input className="input" value={api?.method ?? ""} readOnly disabled />
          </label>
          <label className="stack-xs text-sm">
            <span className="text-muted">API 路径</span>
            <input className="input font-mono text-xs" value={api?.path ?? ""} readOnly disabled />
          </label>
          <label className="stack-xs text-sm">
            <span className="text-muted">API 分组</span>
            <input
              className="input"
              placeholder="例如：用户管理"
              value={apiGroup}
              required
              onChange={(e) => onChangeGroup(e.target.value)}
            />
          </label>
          <label className="stack-xs text-sm">
            <span className="text-muted">API 介绍</span>
            <textarea
              className="input min-h-[5rem]"
              placeholder="简要说明该接口用途"
              value={description}
              onChange={(e) => onChangeDescription(e.target.value)}
            />
          </label>
        </div>
        <div className="flex justify-end gap-2">
          <button type="button" className="btn-secondary" onClick={onClose}>
            取消
          </button>
          <button type="submit" className="btn" disabled={saving || !apiGroup.trim()}>
            {saving ? "保存中…" : "确定"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
