"use client";

import { useEffect, useState } from "react";
import type { LinkedWorkItemVM, ReleasePlanVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";

interface SprintWorkItemsImportModalProps {
  open: boolean;
  plan: ReleasePlanVM;
  busy?: boolean;
  onClose: () => void;
  onSubmit: (workItemIds: string[]) => void | Promise<void>;
}

function workItemKey(item: LinkedWorkItemVM) {
  return item.pm_links?.find((link) => link.external_key)?.external_key ?? null;
}

export function SprintWorkItemsImportModal({ open, plan, busy, onClose, onSubmit }: SprintWorkItemsImportModalProps) {
  const items = plan.linked_work_items ?? [];
  const [selected, setSelected] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (open) setSelected(new Set());
  }, [open]);

  const allSelected = items.length > 0 && selected.size === items.length;

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="从迭代需求导入"
      description={`从迭代「${plan.sprint?.name ?? "未命名迭代"}」选择需求，每条需求生成一个检查项。`}
      footer={
        <ReleasePlanModalFooter
          onCancel={onClose}
          submitLabel={selected.size > 0 ? `导入 ${selected.size} 项` : "导入"}
          submittingLabel="导入中…"
          busy={busy}
          disabled={selected.size === 0}
          onSubmit={() => void onSubmit([...selected])}
        />
      }
    >
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">该迭代暂无需求。</p>
      ) : (
        <div className="sprint-import">
          <label className="sprint-import-all">
            <input
              type="checkbox"
              checked={allSelected}
              onChange={() => setSelected(allSelected ? new Set() : new Set(items.map((item) => item.id)))}
            />
            <span>全选</span>
            <span className="sprint-import-count">共 {items.length} 条</span>
          </label>
          <ul className="sprint-import-list">
            {items.map((item) => {
              const key = workItemKey(item);
              return (
                <li key={item.id}>
                  <label className="sprint-import-item">
                    <input type="checkbox" checked={selected.has(item.id)} onChange={() => toggle(item.id)} />
                    <span className="sprint-import-title">{item.title}</span>
                    {key ? <span className="sprint-import-key">{key}</span> : null}
                  </label>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </Modal>
  );
}
