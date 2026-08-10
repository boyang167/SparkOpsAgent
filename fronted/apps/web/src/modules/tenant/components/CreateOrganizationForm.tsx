"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { slugify } from "@/modules/tenant/lib/slugify";

export interface CreatedOrganization {
  id: string;
  name: string;
  slug: string;
}

interface CreateOrganizationFormProps {
  onCreated?: (org: CreatedOrganization) => void;
  compact?: boolean;
  embedded?: boolean;
}

export function CreateOrganizationForm({
  onCreated,
  compact,
  embedded,
}: CreateOrganizationFormProps) {
  const router = useRouter();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setSuccess(null);
    try {
      const org = await tenant.createOrganization({ name, slug });
      setName("");
      setSlug("");
      setSuccess(`组织「${org.name}」已创建`);
      onCreated?.(org);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建组织失败");
    } finally {
      setLoading(false);
    }
  }

  if (embedded) {
    return (
      <form className="tenant-org-form" onSubmit={onSubmit}>
        <div className="tenant-ws-field">
          <label className="label" htmlFor="create-org-name">
            组织名称
          </label>
          <input
            id="create-org-name"
            className="input"
            placeholder="Acme Corporation"
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (!slug) setSlug(slugify(e.target.value));
            }}
            required
          />
        </div>
        <div className="tenant-ws-field">
          <label className="label" htmlFor="create-org-slug">
            Slug
          </label>
          <input
            id="create-org-slug"
            className="input"
            placeholder="acme-corp"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            required
          />
        </div>
        <div className="tenant-ws-submit">
          <button className="btn" type="submit" disabled={loading}>
            {loading ? "创建中..." : "创建"}
          </button>
        </div>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}
      </form>
    );
  }

  const wrapperClass = compact ? "space-y-3" : "form-panel space-y-4";

  return (
    <form className={wrapperClass} onSubmit={onSubmit}>
      {!compact ? (
        <div>
          <h3 className="section-title">创建组织</h3>
          <p className="text-muted mt-1">组织是租户顶层容器，其下可创建多个工作区</p>
        </div>
      ) : null}

      <div className={compact ? "space-y-3" : "stack-md"}>
        <div>
          <label className="label" htmlFor="create-org-name">
            组织名称
          </label>
          <input
            id="create-org-name"
            className="input"
            placeholder="例如：Acme Corporation"
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (!slug) setSlug(slugify(e.target.value));
            }}
            required
          />
          {!compact ? <p className="field-hint">对外展示的企业或团队名称</p> : null}
        </div>
        <div>
          <label className="label" htmlFor="create-org-slug">
            Slug 标识
          </label>
          <input
            id="create-org-slug"
            className="input"
            placeholder="例如：acme-corp"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            required
          />
          {!compact ? <p className="field-hint">全局唯一标识，用于路由与系统集成</p> : null}
        </div>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}

      <button className={compact ? "btn w-full sm:w-auto" : "btn w-full"} type="submit" disabled={loading}>
        {loading ? "创建中..." : "创建组织"}
      </button>
    </form>
  );
}
