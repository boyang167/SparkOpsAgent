"use client";

import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { identity, tenant } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { PlatformMemberVM } from "@sparkops/contracts/identity";
import type { PlatformOrganizationVM } from "@sparkops/contracts/tenant";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { orgAccentColor, orgInitial } from "@/modules/tenant/lib/org-display";
import { slugify, isValidSlug } from "@/modules/tenant/lib/slug-utils";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

type StatusFilter = "all" | "active" | "suspended" | "deleted";
type ViewMode = "card" | "table";

const STATUS_SORT_ORDER: Record<PlatformOrganizationVM["status"], number> = {
  active: 0,
  suspended: 1,
  deleted: 2,
};

function sortOrganizations(items: PlatformOrganizationVM[]): PlatformOrganizationVM[] {
  return [...items].sort((a, b) => {
    const byStatus = STATUS_SORT_ORDER[a.status] - STATUS_SORT_ORDER[b.status];
    if (byStatus !== 0) return byStatus;
    return a.name.localeCompare(b.name, "zh-CN");
  });
}

interface PlatformOrganizationsPanelProps {
  variant?: "platform" | "settings";
}

function OrgAvatar({ org }: { org: PlatformOrganizationVM }) {
  const color = orgAccentColor(org.id);
  const muted = org.status === "deleted";
  return (
    <span
      className="platform-org-avatar"
      style={{
        background: muted ? "#f1f5f9" : `${color}14`,
        color: muted ? "#94a3b8" : color,
        boxShadow: muted ? "inset 0 0 0 1px #e2e8f0" : `inset 0 0 0 1px ${color}28`,
      }}
      aria-hidden
    >
      {orgInitial(org.name)}
    </span>
  );
}

function BuildingIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden>
      <path d="M3 21h18M6 21V7l6-3 6 3v14M10 21v-4h4v4" strokeLinejoin="round" />
    </svg>
  );
}

function statusLabel(status: PlatformOrganizationVM["status"]): string {
  if (status === "active") return "正常";
  if (status === "deleted") return "已删除";
  return "已停用";
}

function statusBadgeClass(status: PlatformOrganizationVM["status"]): string {
  if (status === "active") return "platform-org-badge platform-org-badge-active";
  if (status === "deleted") return "platform-org-badge platform-org-badge-deleted";
  return "platform-org-badge platform-org-badge-suspended";
}

function ownerLabel(org: PlatformOrganizationVM): string {
  if (!org.owner_email?.trim()) return "未指定";
  const name = org.owner_display_name?.trim();
  return name ? `${name} (${org.owner_email})` : org.owner_email;
}

function ViewModeToggle({ mode, onChange }: { mode: ViewMode; onChange: (mode: ViewMode) => void }) {
  return (
    <div className="platform-orgs-view-toggle" role="group" aria-label="视图切换">
      <button
        type="button"
        className={`platform-orgs-view-btn${mode === "card" ? " platform-orgs-view-btn-active" : ""}`}
        aria-pressed={mode === "card"}
        onClick={() => onChange("card")}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
          <rect x="3" y="3" width="7" height="7" rx="1.5" />
          <rect x="14" y="3" width="7" height="7" rx="1.5" />
          <rect x="3" y="14" width="7" height="7" rx="1.5" />
          <rect x="14" y="14" width="7" height="7" rx="1.5" />
        </svg>
        卡片
      </button>
      <button
        type="button"
        className={`platform-orgs-view-btn${mode === "table" ? " platform-orgs-view-btn-active" : ""}`}
        aria-pressed={mode === "table"}
        onClick={() => onChange("table")}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
          <path d="M4 6h16M4 12h16M4 18h16" strokeLinecap="round" />
        </svg>
        表格
      </button>
    </div>
  );
}

function OrgActions({
  org,
  busy,
  onToggle,
  onTransfer,
  onDelete,
  layout,
}: {
  org: PlatformOrganizationVM;
  busy: boolean;
  onToggle: () => void;
  onTransfer: () => void;
  onDelete: () => void;
  layout: "inline" | "stack";
}) {
  if (org.status === "deleted") {
    return <span className="platform-org-readonly-hint">仅可查看</span>;
  }
  return (
    <div className={layout === "stack" ? "platform-org-card-actions" : "platform-row-actions"}>
      <button type="button" className="platform-row-action" disabled={busy} onClick={onToggle}>
        {org.status === "active" ? "停用" : "启用"}
      </button>
      <button type="button" className="platform-row-action" disabled={busy} onClick={onTransfer}>
        转让 Owner
      </button>
      <button
        type="button"
        className="platform-row-action platform-row-action-danger"
        disabled={busy}
        onClick={onDelete}
      >
        删除
      </button>
    </div>
  );
}

function OrgCardItem({
  org,
  busy,
  onToggle,
  onTransfer,
  onDelete,
}: {
  org: PlatformOrganizationVM;
  busy: boolean;
  onToggle: () => void;
  onTransfer: () => void;
  onDelete: () => void;
}) {
  const isDeleted = org.status === "deleted";
  const accent = orgAccentColor(org.id);
  return (
    <article
      className={`platform-org-card${isDeleted ? " platform-org-card-deleted" : ""}`}
      style={{ ["--org-accent" as string]: accent }}
    >
      <div className="platform-org-card-top">
        <OrgAvatar org={org} />
        <div className="min-w-0 flex-1">
          <div className="platform-org-card-title-row">
            <p className="platform-org-name">{org.name}</p>
            <span className={statusBadgeClass(org.status)}>{statusLabel(org.status)}</span>
          </div>
          <p className="platform-org-slug">{org.slug}</p>
          <p className="platform-org-owner">Owner：{ownerLabel(org)}</p>
        </div>
      </div>
      <p className="platform-org-card-time">
        创建于 {new Date(org.created_at).toLocaleString()}
      </p>
      <OrgActions
        org={org}
        busy={busy}
        layout="stack"
        onToggle={onToggle}
        onTransfer={onTransfer}
        onDelete={onDelete}
      />
    </article>
  );
}

export function PlatformOrganizationsPanel({ variant = "platform" }: PlatformOrganizationsPanelProps) {
  const isPlatform = variant === "platform";
  const router = useRouter();
  const [orgs, setOrgs] = useState<PlatformOrganizationVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
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
  const { confirm, ConfirmDialog } = useConfirmDialog();

  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [viewMode, setViewMode] = useState<ViewMode>("card");

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

  const stats = useMemo(
    () => ({
      total: orgs.length,
      active: orgs.filter((o) => o.status === "active").length,
      suspended: orgs.filter((o) => o.status === "suspended").length,
      deleted: orgs.filter((o) => o.status === "deleted").length,
    }),
    [orgs],
  );

  const filteredOrgs = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    const filtered = orgs.filter((org) => {
      if (statusFilter !== "all" && org.status !== statusFilter) return false;
      if (!q) return true;
      return org.name.toLowerCase().includes(q) ||
        org.slug.toLowerCase().includes(q) ||
        (org.owner_email?.toLowerCase().includes(q) ?? false) ||
        (org.owner_display_name?.toLowerCase().includes(q) ?? false);
    });
    return sortOrganizations(filtered);
  }, [orgs, searchQuery, statusFilter]);

  function flashSuccess(message: string) {
    setSuccess(message);
    window.setTimeout(() => setSuccess(null), 3000);
  }

  function onResetFilters() {
    setSearchQuery("");
    setStatusFilter("all");
  }

  function openCreate() {
    setName("");
    setSlug("");
    setSlugManual(false);
    setOwnerEmail("");
    setCreateOpen(true);
  }

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
      flashSuccess(`组织「${name}」创建成功`);
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
      flashSuccess(next === "active" ? "组织已启用" : "组织已停用");
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
      flashSuccess("Owner 转让成功");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "转让失败");
    } finally {
      setActionOrgId(null);
    }
  }

  async function onDelete(org: PlatformOrganizationVM) {
    const ok = await confirm({ title: `确定删除组织「${org.name}」？此操作为软删除，数据仍保留在数据库中。`, danger: true });
    if (!ok) return;
    setActionOrgId(org.id);
    setError(null);
    try {
      await tenant.deletePlatformOrganization(org.id);
      flashSuccess("组织已软删除");
      await load();
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setActionOrgId(null);
    }
  }

  const statusTabs: { id: StatusFilter; label: string; count: number }[] = [
    { id: "all", label: "全部", count: stats.total },
    { id: "active", label: "正常", count: stats.active },
    { id: "suspended", label: "已停用", count: stats.suspended },
    { id: "deleted", label: "已删除", count: stats.deleted },
  ];

  return (
    <>
    <div className={isPlatform ? "platform-orgs-page" : "stack-lg"}>
      {!isPlatform ? (
        <>
          <Breadcrumb items={[{ label: "设置", href: "/settings/profile" }, { label: "平台组织" }]} />
          <div>
            <h1 className="text-xl font-semibold">平台组织管理</h1>
            <p className="text-sm text-muted mt-1">
              Super Admin 查看平台全部组织（含已软删除记录）；活跃组织可启停、转让 Owner 与删除
            </p>
          </div>
        </>
      ) : null}

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}

      <section className={isPlatform ? "platform-table-card platform-orgs-card" : "card p-4 stack-md"}>
        {isPlatform ? (
          <PlatformPanelHead
            title="平台组织"
            description="管理平台全部租户组织，支持创建、启停、转让 Owner；已删除记录保留供审计查阅。"
            action={
              <button type="button" className="btn platform-add-btn" onClick={openCreate}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
                  <path d="M12 5v14M5 12h14" strokeLinecap="round" />
                </svg>
                创建组织
              </button>
            }
          />
        ) : null}

        <div className="platform-orgs-controls">
          <div className="platform-orgs-search-wrap">
            <svg className="platform-orgs-search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
              <circle cx="11" cy="11" r="7" />
              <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
            </svg>
            <input
              className="input platform-orgs-search"
              placeholder="搜索组织名称、Slug 或 Owner…"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
            {searchQuery || statusFilter !== "all" ? (
              <button type="button" className="platform-orgs-clear" onClick={onResetFilters}>
                清除
              </button>
            ) : null}
          </div>

          {isPlatform ? (
            <div className="platform-orgs-toolbar-row">
              <div className="platform-orgs-tabs" role="tablist" aria-label="按状态筛选">
                {statusTabs.map((tab) => (
                  <button
                    key={tab.id}
                    type="button"
                    role="tab"
                    aria-selected={statusFilter === tab.id}
                    className={`platform-orgs-tab${statusFilter === tab.id ? " platform-orgs-tab-active" : ""}`}
                    onClick={() => setStatusFilter(tab.id)}
                  >
                    {tab.label}
                    <span className="platform-orgs-tab-count">{tab.count}</span>
                  </button>
                ))}
              </div>
              <ViewModeToggle mode={viewMode} onChange={setViewMode} />
            </div>
          ) : (
            <div className="platform-orgs-toolbar-row platform-orgs-toolbar-row-end">
              <ViewModeToggle mode={viewMode} onChange={setViewMode} />
            </div>
          )}
        </div>

        {!isPlatform ? (
          <div className="platform-table-toolbar platform-orgs-toolbar">
            <button type="button" className="btn platform-add-btn" onClick={openCreate}>
              创建组织
            </button>
          </div>
        ) : null}

        {loading ? (
          <div className="platform-table-empty">加载中…</div>
        ) : filteredOrgs.length === 0 ? (
          <div className="platform-table-empty">
            <div className="platform-empty-icon" aria-hidden>
              <BuildingIcon />
            </div>
            <p>{orgs.length === 0 ? "暂无组织，点击右上角创建第一个" : "无匹配组织，请调整筛选条件"}</p>
          </div>
        ) : viewMode === "card" ? (
          <div className="platform-orgs-grid-wrap">
            <div className="platform-orgs-grid">
              {filteredOrgs.map((org, index) => {
                const busy = actionOrgId === org.id;
                const prev = filteredOrgs[index - 1];
                const showDeletedDivider =
                  statusFilter === "all" &&
                  org.status === "deleted" &&
                  prev !== undefined &&
                  prev.status !== "deleted";

                return (
                  <Fragment key={org.id}>
                    {showDeletedDivider ? (
                      <div className="platform-orgs-grid-divider">
                        <span className="platform-orgs-divider-label">已删除组织</span>
                      </div>
                    ) : null}
                    <OrgCardItem
                      org={org}
                      busy={busy}
                      onToggle={() => void toggleStatus(org)}
                      onTransfer={() => {
                        setTransferOrg(org);
                        setTransferEmail("");
                      }}
                      onDelete={() => void onDelete(org)}
                    />
                  </Fragment>
                );
              })}
            </div>
          </div>
        ) : (
          <div className="platform-table-wrap">
            <table className="platform-table">
              <thead>
                <tr>
                  <th>组织</th>
                  <th>Owner</th>
                  <th>状态</th>
                  <th>创建时间</th>
                  <th className="platform-table-actions-col">操作</th>
                </tr>
              </thead>
              <tbody>
                {filteredOrgs.map((org, index) => {
                  const isDeleted = org.status === "deleted";
                  const busy = actionOrgId === org.id;
                  const prev = filteredOrgs[index - 1];
                  const showDeletedDivider =
                    statusFilter === "all" &&
                    isDeleted &&
                    prev !== undefined &&
                    prev.status !== "deleted";

                  return (
                    <Fragment key={org.id}>
                      {showDeletedDivider ? (
                        <tr className="platform-orgs-divider-row">
                          <td colSpan={5}>
                            <span className="platform-orgs-divider-label">已删除组织</span>
                          </td>
                        </tr>
                      ) : null}
                      <tr className={isDeleted ? "platform-org-row-deleted" : undefined}>
                        <td>
                          <div className="platform-org-cell">
                            <OrgAvatar org={org} />
                            <div className="min-w-0">
                              <p className="platform-org-name">{org.name}</p>
                              <p className="platform-org-slug">{org.slug}</p>
                            </div>
                          </div>
                        </td>
                        <td className="platform-org-owner-cell">{ownerLabel(org)}</td>
                        <td>
                          <span className={statusBadgeClass(org.status)}>{statusLabel(org.status)}</span>
                        </td>
                        <td className="platform-table-time">
                          {new Date(org.created_at).toLocaleString()}
                        </td>
                        <td>
                          <OrgActions
                            org={org}
                            busy={busy}
                            layout="inline"
                            onToggle={() => void toggleStatus(org)}
                            onTransfer={() => {
                              setTransferOrg(org);
                              setTransferEmail("");
                            }}
                            onDelete={() => void onDelete(org)}
                          />
                        </td>
                      </tr>
                    </Fragment>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {!loading && filteredOrgs.length > 0 ? (
          <div className="platform-table-footer">
            <span className="text-sm text-muted">共 {filteredOrgs.length} 个组织</span>
          </div>
        ) : null}
      </section>

      <Modal
        open={createOpen}
        onClose={() => {
          setCreateOpen(false);
          setSlugManual(false);
        }}
        title="创建组织"
      >
        <form onSubmit={onCreate} className="stack-md">
          <p className="text-sm text-muted -mt-1">
            在平台层创建新租户组织，可选指定首位 Owner。
          </p>
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
              placeholder="例如：Acme Corp"
              required
            />
          </label>
          <label className="stack-xs">
            <span className="label">Slug</span>
            <input
              className="input font-mono text-sm"
              value={slug}
              onChange={(e) => {
                setSlugManual(true);
                setSlug(e.target.value);
              }}
              placeholder="acme-corp"
              required
            />
          </label>
          <label className="stack-xs">
            <span className="label">Owner（可选）</span>
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
          <div className="flex justify-end gap-2 pt-1">
            <button type="button" className="btn-secondary" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn" disabled={creating}>
              {creating ? "创建中…" : "创建组织"}
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
          <p className="text-sm text-muted -mt-1">
            新 Owner 须已在平台注册；若不在组织中会自动加入并设为 Owner。
          </p>
          <label className="stack-xs">
            <span className="label">新 Owner 邮箱</span>
            <input
              className="input"
              type="email"
              placeholder="owner@company.com"
              value={transferEmail}
              onChange={(e) => setTransferEmail(e.target.value)}
              required
            />
          </label>
          <div className="flex justify-end gap-2 pt-1">
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
    <ConfirmDialog />
    </>
  );
}
