"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { ReleasePlanModalFooter } from "./ReleasePlanModalFooter";

interface ReleasePlanNamePromptModalProps {
  open: boolean;
  title: string;
  description?: string;
  nameLabel?: string;
  namePlaceholder?: string;
  initialName?: string;
  showDescription?: boolean;
  initialDescription?: string;
  descriptionLabel?: string;
  submitLabel: string;
  submittingLabel?: string;
  busy?: boolean;
  onClose: () => void;
  onSubmit: (values: { name: string; description?: string }) => void | Promise<void>;
}

export function ReleasePlanNamePromptModal({
  open,
  title,
  description,
  nameLabel = "名称",
  namePlaceholder,
  initialName = "",
  showDescription = false,
  initialDescription = "",
  descriptionLabel = "描述",
  submitLabel,
  submittingLabel,
  busy,
  onClose,
  onSubmit,
}: ReleasePlanNamePromptModalProps) {
  const [name, setName] = useState(initialName);
  const [detail, setDetail] = useState(initialDescription);

  useEffect(() => {
    if (!open) return;
    setName(initialName);
    setDetail(initialDescription);
  }, [open, initialName, initialDescription]);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      description={description}
      footer={
        <ReleasePlanModalFooter
          onCancel={onClose}
          busy={busy}
          disabled={!name.trim()}
          submitLabel={submitLabel}
          submittingLabel={submittingLabel}
          onSubmit={() =>
            void onSubmit({
              name: name.trim(),
              description: showDescription ? detail.trim() || undefined : undefined,
            })
          }
        />
      }
    >
      <div className="stack-md text-sm">
        <label className="stack-xs">
          <span className="text-muted-foreground">{nameLabel} *</span>
          <input
            className="input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={namePlaceholder}
            autoFocus
          />
        </label>
        {showDescription ? (
          <label className="stack-xs">
            <span className="text-muted-foreground">{descriptionLabel}</span>
            <textarea
              className="input min-h-[72px]"
              value={detail}
              onChange={(e) => setDetail(e.target.value)}
              placeholder="可选"
            />
          </label>
        ) : null}
      </div>
    </Modal>
  );
}
