"use client";

import { useEffect, useId, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { slugify } from "@/modules/tenant/lib/slug-utils";

interface CreateProductDialogProps {
  open: boolean;
  workspaceId: string;
  busy?: boolean;
  onClose: () => void;
  onCreated: (productId: string) => void;
}

const FORM_ID = "create-product-dialog-form";

export function CreateProductDialog({
  open,
  workspaceId,
  busy: busyProp = false,
  onClose,
  onCreated,
}: CreateProductDialogProps) {
  const nameFieldId = useId();
  const displayNameFieldId = useId();
  const descriptionFieldId = useId();

  const [name, setName] = useState("");
  const [nameTouched, setNameTouched] = useState(false);
  const [displayName, setDisplayName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const busy = busyProp || loading;
  const trimmedName = name.trim();
  const nameValid = trimmedName.length >= 3 && trimmedName.length <= 100;

  useEffect(() => {
    if (!open) return;
    setName("");
    setNameTouched(false);
    setDisplayName("");
    setDescription("");
    setError(null);
    setLoading(false);
  }, [open]);

  function handleDisplayNameChange(value: string) {
    setDisplayName(value);
    if (!nameTouched) {
      setName(slugify(value));
    }
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (trimmedName.length < 3) {
      setError("产品标识至少 3 个字符");
      return;
    }
    if (trimmedName.length > 100) {
      setError("产品标识不能超过 100 个字符");
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const created = await product.createProduct(workspaceId, {
        name: trimmedName,
        display_name: displayName.trim() || undefined,
        description: description.trim() || undefined,
      });
      onCreated(created.id);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="新建产品"
      panelClassName="product-create-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" disabled={busy} onClick={onClose}>
            取消
          </button>
          <button type="submit" form={FORM_ID} className="btn btn-primary" disabled={busy || !nameValid}>
            {busy ? "创建中…" : "创建"}
          </button>
        </div>
      }
    >
      <form id={FORM_ID} className="product-create-form" onSubmit={onSubmit} noValidate>
        <div className="form-field">
          <label className="label" htmlFor={displayNameFieldId}>
            产品名称
          </label>
          <input
            id={displayNameFieldId}
            className="input"
            value={displayName}
            onChange={(e) => handleDisplayNameChange(e.target.value)}
            placeholder="产品名称"
            autoComplete="off"
            autoFocus
          />
        </div>

        <div className="form-field">
          <label className="label" htmlFor={nameFieldId}>
            产品标识 <span className="label-required">*</span>
          </label>
          <input
            id={nameFieldId}
            className="input input-mono"
            value={name}
            onChange={(e) => {
              setNameTouched(true);
              setName(e.target.value);
            }}
            placeholder="产品标识"
            autoComplete="off"
            maxLength={100}
          />
          <p className="field-hint">创建后不可修改，建议小写英文与连字符</p>
        </div>

        <div className="form-field">
          <label className="label" htmlFor={descriptionFieldId}>
            描述
          </label>
          <textarea
            id={descriptionFieldId}
            className="input product-create-textarea"
            value={description}
            onChange={(e) => setDescription(e.target.value.slice(0, 500))}
            placeholder="可选"
            rows={2}
          />
        </div>

        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      </form>
    </Modal>
  );
}
