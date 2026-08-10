"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { slugify, isValidSlug, slugPreview } from "@/modules/tenant/lib/slug-utils";

export interface CreatedOrganization {
  id: string;
  name: string;
  slug: string;
}

interface CreateOrganizationDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated?: (org: CreatedOrganization) => void;
  onContinueToWorkspace?: (org: CreatedOrganization) => void;
  /** 创建成功后是否引导继续创建工作区 */
  suggestWorkspace?: boolean;
}

type DialogStep = "form" | "success";

export function CreateOrganizationDialog({
  open,
  onClose,
  onCreated,
  onContinueToWorkspace,
  suggestWorkspace = true,
}: CreateOrganizationDialogProps) {
  const router = useRouter();
  const [step, setStep] = useState<DialogStep>("form");
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [created, setCreated] = useState<CreatedOrganization | null>(null);

  useEffect(() => {
    if (!open) return;
    setStep("form");
    setName("");
    setSlug("");
    setSlugTouched(false);
    setError(null);
    setLoading(false);
    setCreated(null);
  }, [open]);

  function handleNameChange(value: string) {
    setName(value);
    if (!slugTouched) {
      setSlug(slugify(value));
    }
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!name.trim()) {
      setError("请填写组织名称");
      return;
    }
    if (!slug.trim()) {
      setError("请填写唯一标识");
      return;
    }
    if (!isValidSlug(slug)) {
      setError("唯一标识仅支持小写英文、数字和连字符，例如 acme-corp");
      return;
    }

    setLoading(true);
    try {
      const org = await tenant.createOrganization({ name: name.trim(), slug: slug.trim() });
      setCreated(org);
      setStep("success");
      onCreated?.(org);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建组织失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  }

  function handleClose() {
    onClose();
  }

  function handleDone() {
    handleClose();
  }

  function handleContinueWorkspace() {
    if (!created) return;
    if (onContinueToWorkspace) {
      onContinueToWorkspace(created);
    } else {
      onCreated?.(created);
      handleClose();
    }
  }

  if (step === "success" && created) {
    return (
      <Modal
        open={open}
        onClose={handleClose}
        title="组织已创建"
        description="您的租户顶层容器已就绪，可继续配置工作区。"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={handleDone}>
              稍后配置
            </button>
            {suggestWorkspace ? (
              <button type="button" className="btn" onClick={handleContinueWorkspace}>
                继续创建工作区
              </button>
            ) : (
              <button type="button" className="btn" onClick={handleDone}>
                完成
              </button>
            )}
          </div>
        }
      >
        <div className="create-success">
          <div className="create-success-icon" aria-hidden>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M20 6 9 17l-5-5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <p className="create-success-name">{created.name}</p>
          <p className="create-success-meta">
            唯一标识 <code>{created.slug}</code>
          </p>
          <p className="create-success-path">路径预览 {slugPreview(created.slug, "org")}</p>
        </div>
      </Modal>
    );
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="新建组织"
      description="组织是企业级租户容器，用于隔离计费、成员与资源边界。"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" onClick={handleClose} disabled={loading}>
            取消
          </button>
          <button type="submit" form="create-org-dialog-form" className="btn" disabled={loading}>
            {loading ? "创建中..." : "创建组织"}
          </button>
        </div>
      }
    >
      <form id="create-org-dialog-form" className="create-dialog-form" onSubmit={onSubmit} noValidate>
        <div className="create-field">
          <label className="label" htmlFor="dialog-org-name">
            组织名称 <span className="label-required">*</span>
          </label>
          <input
            id="dialog-org-name"
            className="input"
            placeholder="例如：星火科技"
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
            autoComplete="organization"
            autoFocus
          />
          <p className="field-hint">在平台中展示给成员看的名称，支持中文。</p>
        </div>

        <div className="create-field">
          <div className="create-field-label-row">
            <label className="label" htmlFor="dialog-org-slug">
              唯一标识 <span className="label-required">*</span>
            </label>
            {!slugTouched && slug ? <span className="create-auto-badge">自动生成</span> : null}
          </div>
          <input
            id="dialog-org-slug"
            className="input input-mono"
            placeholder="例如：spark-tech"
            value={slug}
            onChange={(e) => {
              setSlugTouched(true);
              setSlug(e.target.value);
            }}
            autoComplete="off"
          />
          <p className="field-hint">
            系统内部使用的英文 ID，仅小写字母、数字与连字符。创建后不建议修改。
          </p>
          {slug ? (
            <p className="create-preview">
              预览 <code>{slugPreview(slug, "org")}</code>
            </p>
          ) : null}
        </div>

        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      </form>
    </Modal>
  );
}
