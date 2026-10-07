"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { slugify, isValidSlug, slugPreview } from "@/modules/tenant/lib/slug-utils";
import type { OrgMeta } from "@/modules/tenant/lib/org-display";

interface CreateWorkspaceDialogProps {
  open: boolean;
  onClose: () => void;
  organizations: OrgMeta[];
  defaultOrgId?: string;
  onCreated?: (workspace: { id: string; name: string; slug: string }) => void;
}

type DialogStep = "form" | "success";

export function CreateWorkspaceDialog({
  open,
  onClose,
  organizations,
  defaultOrgId,
  onCreated,
}: CreateWorkspaceDialogProps) {
  const router = useRouter();
  const [step, setStep] = useState<DialogStep>("form");
  const [orgId, setOrgId] = useState(defaultOrgId ?? organizations[0]?.id ?? "");
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [created, setCreated] = useState<{ id: string; name: string; slug: string } | null>(null);

  useEffect(() => {
    if (!open) return;
    setStep("form");
    setOrgId(defaultOrgId ?? organizations[0]?.id ?? "");
    setName("");
    setSlug("");
    setSlugTouched(false);
    setError(null);
    setLoading(false);
    setCreated(null);
  }, [open, defaultOrgId, organizations]);

  function handleNameChange(value: string) {
    setName(value);
    if (!slugTouched) {
      setSlug(slugify(value));
    }
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!orgId) {
      setError("请选择所属组织");
      return;
    }
    if (!name.trim()) {
      setError("请填写工作区名称");
      return;
    }
    if (!slug.trim()) {
      setError("请填写唯一标识");
      return;
    }
    if (!isValidSlug(slug)) {
      setError("唯一标识仅支持小写英文、数字和连字符，例如 dev-team");
      return;
    }

    setLoading(true);
    try {
      const ws = await tenant.createWorkspace(orgId, { name: name.trim(), slug: slug.trim() });
      await tenant.selectWorkspace(ws.id);
      setCreated(ws);
      setStep("success");
      onCreated?.(ws);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建工作区失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  }

  function handleClose() {
    onClose();
  }

  if (step === "success" && created) {
    return (
      <Modal
        open={open}
        onClose={handleClose}
        title="工作区已创建"
        description="已自动切换为当前工作区，您可以立即开始使用。"
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={handleClose}>
              留在此页
            </button>
            <Link href="/dashboard" className="btn" onClick={handleClose}>
              进入控制台
            </Link>
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
          <p className="create-success-path">路径预览 {slugPreview(created.slug, "workspace")}</p>
        </div>
      </Modal>
    );
  }

  const noOrgs = organizations.length === 0;
  const selectedOrg = organizations.find((org) => org.id === orgId);
  const trimmedSlug = slug.trim();
  const slugValid = isValidSlug(trimmedSlug);
  const canSubmit = Boolean(orgId && name.trim() && slugValid);
  const useOrgMenu = organizations.length > 5;

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="新建工作区"
      footer={
        noOrgs ? (
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={handleClose}>
              关闭
            </button>
            <Link href="/settings/tenant" className="btn" onClick={handleClose}>
              去创建组织
            </Link>
          </div>
        ) : (
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={handleClose} disabled={loading}>
              取消
            </button>
            <button type="submit" form="create-ws-dialog-form" className="btn" disabled={loading || !canSubmit}>
              {loading ? "创建中..." : "创建工作区"}
            </button>
          </div>
        )
      }
    >
      {noOrgs ? (
        <InlineAlert tone="warning">
          尚未创建任何组织。请先创建组织，再在其下添加工作区。
        </InlineAlert>
      ) : (
        <form id="create-ws-dialog-form" className="create-dialog-form" onSubmit={onSubmit} noValidate>
          <div className="create-field">
            <span className="label" id="dialog-ws-org-label">
              所属组织 <span className="label-required">*</span>
            </span>
            {organizations.length === 1 && selectedOrg ? (
              <p className="ws-create-org-current">{selectedOrg.name}</p>
            ) : useOrgMenu ? (
              <select
                className="ws-create-org-select"
                aria-labelledby="dialog-ws-org-label"
                value={orgId}
                onChange={(event) => setOrgId(event.target.value)}
              >
                {organizations.map((org) => (
                  <option key={org.id} value={org.id}>
                    {org.name}
                  </option>
                ))}
              </select>
            ) : (
              <div className="ws-create-org-list" role="radiogroup" aria-labelledby="dialog-ws-org-label">
                {organizations.map((org) => {
                  const selected = orgId === org.id;
                  return (
                    <label key={org.id} className={selected ? "is-active" : ""}>
                      <input
                        type="radio"
                        name="workspace-org"
                        value={org.id}
                        checked={selected}
                        onChange={() => setOrgId(org.id)}
                        className="sr-only"
                      />
                      <span>{org.name}</span>
                    </label>
                  );
                })}
              </div>
            )}
          </div>

          <div className="create-field">
            <label className="label" htmlFor="dialog-ws-name">
              工作区名称 <span className="label-required">*</span>
            </label>
            <input
              id="dialog-ws-name"
              className="input"
              placeholder="例如：研发团队"
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              autoComplete="off"
              autoFocus
              disabled={!orgId}
            />
          </div>

          <div className="create-field">
            <div className="create-field-label-row">
              <label className="label" htmlFor="dialog-ws-slug">
                唯一标识 <span className="label-required">*</span>
              </label>
              {!slugTouched && trimmedSlug ? <span className="ws-create-generated">已根据名称生成</span> : null}
            </div>
            <input
              id="dialog-ws-slug"
              className="input input-mono"
              placeholder="dev-team"
              value={slug}
              onChange={(e) => {
                setSlugTouched(true);
                setSlug(e.target.value);
              }}
              autoComplete="off"
              spellCheck={false}
              disabled={!orgId}
              aria-invalid={slugTouched && trimmedSlug.length > 0 && !slugValid}
            />
            {trimmedSlug && slugValid ? (
              <p className="create-preview">
                路径 <code>{slugPreview(trimmedSlug, "workspace")}</code>
              </p>
            ) : slugTouched && trimmedSlug && !slugValid ? (
              <p className="ws-create-slug-error">仅支持小写英文、数字和连字符</p>
            ) : (
              <p className="field-hint">英文名称会自动生成；中文名称请手动填写，例如 dev-team。</p>
            )}
          </div>

          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        </form>
      )}
    </Modal>
  );
}
