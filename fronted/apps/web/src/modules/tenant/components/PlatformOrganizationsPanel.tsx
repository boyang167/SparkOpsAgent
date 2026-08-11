"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { identity, tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PlatformMemberVM } from "@sparkops/contracts/identity";
import type { PlatformOrganizationVM } from "@sparkops/contracts/tenant";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { slugify, isValidSlug } from "@/modules/tenant/lib/slug-utils";

export function PlatformOrganizationsPanel() {
  const router = useRouter();
  const [orgs, setOrgs] = useState<PlatformOrganizationVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugManual, setSlugManual] = useState(false);
  const [ownerEmail, setOwnerEmail] = useState("");
  const [platformMembers, setPlatformMembers] = useState<PlatformMemberVM[]>([]);
  const [membersLoading, setMembersLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [actionOrgId, setActionOrgId] = useState<string | null>(null);
  const [transferOrg, setTransferOrg] = useState<PlatformOrganizationVM | null>(null);
  const [transferEmail, setTransferEmail] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setOrgs(await tenant.listPlatformOrganizations());
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!createOpen) return;
    setMembersLoading(true);
    void identity
      .listPlatformMembers()
      .then(setPlatformMembers)
      .catch(() => setPlatformMembers([]))
      .finally(() => setMembersLoading(false));
  }, [createOpen]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!isValidSlug(slug)) return;
    setCreating(true);
    setError(null);
    try {
      await tenant.createPlatformOrganization({
        name,
        slug,
        ...(ownerEmail ? { owner_email: ownerEmail } : {}),
      });
      setCreateOpen(false);
      setName("");
      setSlug("");
      setSlugManual(false);
      setOwnerEmail("");
      await load();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    } finally {
      setCreating(false);
    }
  }

  async function toggleStatus(org: PlatformOrganizationVM) {
    setActionOrgId(org.id);
    setError(null);
    try {
      const next = org.status === "active" ? "suspended" : "active";
      await tenant.updatePlatformOrganizationStatus(org.id, next);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新状态失败");
    } finally {
      setActionOrgId(null);
    }
  }

  async function onTransfer(e: React.FormEvent) {
    e.preventDefault();
    if (!transferOrg || !transferEmail.trim()) return;
    setActionOrgId(transferOrg.id);
    setError(null);
    try {
      await tenant.transferPlatformOrgOwner(transferOrg.id, transferEmail.trim());
      setTransferOrg(null);
      setTransferEmail("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "转让失败");
    } finally {
      setActionOrgId(null);
    }
  }

  async function onDelete(org: PlatformOrganizationVM) {
    if (!window.confirm(`确定删除组织「${org.name}」？此操作为软删除，数据仍保留在数据库中。`)) return;
    setActionOrgId(org.id);
    setError(null);
    try {
      await tenant.deletePlatformOrganization(org.id);
      await load();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setActionOrgId(null);
    }
  }

  return (
    <div className="stack-lg">
      <Breadcrumb items={[{ label: "设置", href: "/settings/profile" }, { label: "平台组织" }]} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">平台组织管理</h1>
          <p className="text-sm text-muted mt-1">Super Admin 创建组织、启停、转让 Owner 与软删除</p>
        </div>
        <button
          type="button"
          className="btn"
          onClick={() => {
            setName("");
            setSlug("");
            setSlugManual(false);
            setOwnerEmail("");
            setCreateOpen(true);
          }}
        >
          创建组织
        </button>
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : orgs.length === 0 ? (
        <div className="empty-state">暂无组织</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>组织</th>
                <th>状态</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {orgs.map((org) => (
                <tr key={org.id}>
                  <td>
                    <p className="font-medium">{org.name}</p>
                    <p className="text-xs text-muted">{org.slug}</p>
                  </td>
                  <td>
                    <span className={`badge ${org.status === "active" ? "badge-primary" : "badge-warning"}`}>
                      {org.status === "active" ? "正常" : "已停用"}
                    </span>
                  </td>
                  <td className="text-sm text-muted">{new Date(org.created_at).toLocaleString()}</td>
                  <td>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        className="btn-secondary !py-1 !px-2 text-xs"
                        disabled={actionOrgId === org.id}
                        onClick={() => toggleStatus(org)}
                      >
                        {org.status === "active" ? "停用" : "启用"}
                      </button>
                      <button
                        type="button"
                        className="btn-secondary !py-1 !px-2 text-xs"
                        onClick={() => {
                          setTransferOrg(org);
                          setTransferEmail("");
                        }}
                      >
                        转让 Owner
                      </button>
                      <button
                        type="button"
                        className="text-xs text-red-600 hover:underline"
                        disabled={actionOrgId === org.id}
                        onClick={() => void onDelete(org)}
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal
        open={createOpen}
        onClose={() => {
          setCreateOpen(false);
          setSlugManual(false);
        }}
        title="创建组织（平台）"
      >
        <form onSubmit={onCreate} className="stack-md">
          <label className="stack-xs">
            <span className="label">组织名称</span>
            <input
              className="input"
              value={name}
              onChange={(e) => {
                const nextName = e.target.value;
                setName(nextName);
                if (!slugManual) setSlug(slugify(nextName));
              }}
              required
            />
          </label>
          <label className="stack-xs">
            <span className="label">Slug</span>
            <input
              className="input"
              value={slug}
              onChange={(e) => {
                setSlugManual(true);
                setSlug(e.target.value);
              }}
              required
            />
          </label>
          <label className="stack-xs">
            <span className="label">Owner</span>
            <select
              className="input"
              value={ownerEmail}
              onChange={(e) => setOwnerEmail(e.target.value)}
              disabled={membersLoading}
            >
              <option value="">暂不指定 Owner</option>
              {platformMembers.map((member) => (
                <option key={member.id} value={member.email}>
                  {member.display_name} ({member.email})
                </option>
              ))}
            </select>
            <p className="text-xs text-muted">
              从平台用户中选择；留空时可稍后在「转让 Owner」中指定
            </p>
          </label>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn" disabled={creating}>
              {creating ? "创建中…" : "创建"}
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        open={Boolean(transferOrg)}
        onClose={() => setTransferOrg(null)}
        title={transferOrg ? `转让 Owner · ${transferOrg.name}` : "转让 Owner"}
      >
        <form onSubmit={onTransfer} className="stack-md">
          <p className="text-sm text-muted">新 Owner 须已在平台注册；若不在组织中会自动加入。</p>
          <input
            className="input"
            type="email"
            placeholder="新 Owner 邮箱"
            value={transferEmail}
            onChange={(e) => setTransferEmail(e.target.value)}
            required
          />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setTransferOrg(null)}>
              取消
            </button>
            <button type="submit" className="btn" disabled={actionOrgId === transferOrg?.id}>
              确认转让
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
