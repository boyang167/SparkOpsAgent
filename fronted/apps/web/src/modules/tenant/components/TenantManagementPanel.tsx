"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { tenant } from "@sparkops/web-sdk";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { Breadcrumb } from "@/components/ui/Breadcrumb";
import { CreateOrganizationDialog } from "./CreateOrganizationDialog";
import { CreateWorkspaceDialog } from "./CreateWorkspaceDialog";
import { OrgMembersDialog } from "./OrgMembersDialog";
import { orgAccentColor, orgInitial, isOrgReadOnly, isWorkspaceReadOnly, extendOrgsWithWorkspaceOrgs, type OrgMeta } from "@/modules/tenant/lib/org-display";

interface TenantManagementPanelProps {
  organizations: OrganizationVM[];
  workspaces: WorkspaceVM[];
  currentWorkspaceId?: string;
  /** settings：个人设置入口；admin：管理控制台 IAM */
  variant?: "settings" | "admin";
}

function OrgAvatar({ label, orgId }: { label: string; orgId: string }) {
  const color = orgAccentColor(orgId);
  return (
    <span
      className="tenant-org-avatar"
      style={{ background: `${color}14`, color, boxShadow: `inset 0 0 0 1px ${color}28` }}
      aria-hidden
    >
      {orgInitial(label)}
    </span>
  );
}

function BuildingIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M3 21h18M6 21V7l6-3 6 3v14M10 21v-4h4v4" strokeLinejoin="round" />
    </svg>
  );
}

function SearchIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <circle cx="11" cy="11" r="7" />
      <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
    </svg>
  );
}

function matchesQuery(text: string, query: string): boolean {
  return text.toLowerCase().includes(query.trim().toLowerCase());
}

export function TenantManagementPanel({
  organizations,
  workspaces,
  currentWorkspaceId,
  variant = "settings",
}: TenantManagementPanelProps) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [orgDialogOpen, setOrgDialogOpen] = useState(false);
  const [wsDialogOpen, setWsDialogOpen] = useState(false);
  const [wsDialogOrgId, setWsDialogOrgId] = useState<string | undefined>();
  const [membersDialogOrg, setMembersDialogOrg] = useState<OrgMeta | null>(null);

  const orgOptions = useMemo<OrgMeta[]>(
    () =>
      extendOrgsWithWorkspaceOrgs(
        [...organizations]
          .map((org) => ({
            id: org.id,
            name: org.name,
            slug: org.slug,
            read_only: org.read_only,
          }))
          .sort((a, b) => a.name.localeCompare(b.name)),
        workspaces,
      ),
    [organizations, workspaces],
  );

  const orgMetaById = useMemo(
    () => new Map(orgOptions.map((org) => [org.id, org])),
    [orgOptions],
  );

  const writableOrgOptions = useMemo(
    () => orgOptions.filter((org) => !isOrgReadOnly(orgMetaById.get(org.id) ?? org)),
    [orgOptions, orgMetaById],
  );

  const orgCount = orgOptions.length;
  const workspaceCount = workspaces.length;

  useEffect(() => {
    if (variant === "settings" && orgCount === 0) {
      setOrgDialogOpen(true);
    }
  }, [orgCount, variant]);

  const grouped = useMemo(() => {
    const map = new Map<string, WorkspaceVM[]>();
    for (const ws of workspaces) {
      const list = map.get(ws.org_id) ?? [];
      list.push(ws);
      map.set(ws.org_id, list);
    }
    for (const list of map.values()) {
      list.sort((a, b) => a.name.localeCompare(b.name));
    }
    return map;
  }, [workspaces]);

  const filteredOrgOptions = useMemo(() => {
    if (!query.trim()) return orgOptions;
    return orgOptions.filter((org) => {
      const items = grouped.get(org.id) ?? [];
      if (matchesQuery(org.name, query) || matchesQuery(org.slug, query)) {
        return true;
      }
      return items.some(
        (ws) => matchesQuery(ws.name, query) || matchesQuery(ws.slug, query),
      );
    });
  }, [orgOptions, grouped, query]);

  const currentWorkspace = workspaces.find((ws) => ws.id === currentWorkspaceId);
  const currentOrg = currentWorkspace
    ? orgOptions.find((org) => org.id === currentWorkspace.org_id)
    : undefined;
  const emptyOrgsWithoutWs = orgOptions.filter((org) => !grouped.has(org.id));

  function openOrgDialog() {
    setOrgDialogOpen(true);
  }

  function openWsDialog(orgId?: string) {
    setWsDialogOrgId(orgId);
    setWsDialogOpen(true);
  }

  async function onSelectWorkspace(workspaceId: string) {
    const target = workspaces.find((ws) => ws.id === workspaceId);
    if (!target || isWorkspaceReadOnly(target)) return;
    if (workspaceId === currentWorkspaceId) return;
    await tenant.selectWorkspace(workspaceId);
    router.refresh();
  }

  function filterWorkspaceItems(items: WorkspaceVM[]): WorkspaceVM[] {
    if (!query.trim()) return items;
    return items.filter(
      (ws) =>
        matchesQuery(ws.name, query) ||
        matchesQuery(ws.slug, query) ||
        matchesQuery(orgOptions.find((org) => org.id === ws.org_id)?.name ?? "", query),
    );
  }

  return (
    <div className="tenant-page stack-lg">
      <CreateOrganizationDialog
        open={orgDialogOpen}
        onClose={() => setOrgDialogOpen(false)}
        onCreated={() => router.refresh()}
        onContinueToWorkspace={(org) => {
          setOrgDialogOpen(false);
          setWsDialogOrgId(org.id);
          setWsDialogOpen(true);
          router.refresh();
        }}
      />
      <CreateWorkspaceDialog
        open={wsDialogOpen}
        onClose={() => {
          setWsDialogOpen(false);
          setWsDialogOrgId(undefined);
        }}
        organizations={writableOrgOptions}
        defaultOrgId={wsDialogOrgId}
      />
      {membersDialogOrg ? (
        <OrgMembersDialog
          open={Boolean(membersDialogOrg)}
          orgId={membersDialogOrg.id}
          orgName={membersDialogOrg.name}
          onClose={() => setMembersDialogOrg(null)}
        />
      ) : null}

      {variant === "settings" ? (
        <Breadcrumb
          items={[
            { label: "设置", href: "/settings/profile" },
            { label: "组织与工作区" },
          ]}
        />
      ) : null}

      <section className={`tenant-hero ${variant === "admin" ? "tenant-hero-admin" : "tenant-hero-compact"}`}>
        <div className="tenant-hero-main">
          {variant === "settings" ? (
            <div>
              <h1 className="tenant-hero-title">组织与工作区</h1>
              <p className="tenant-hero-desc">
                管理租户结构、切换协作上下文，在同一视图完成创建与浏览。
              </p>
            </div>
          ) : null}
          <div className="tenant-hero-stats" aria-label="概览">
            <div className="tenant-stat-chip">
              <span className="tenant-stat-value">{orgCount}</span>
              <span className="tenant-stat-label">组织</span>
            </div>
            <div className="tenant-stat-chip">
              <span className="tenant-stat-value">{workspaceCount}</span>
              <span className="tenant-stat-label">工作区</span>
            </div>
            <div className="tenant-stat-chip tenant-stat-chip-active">
              <span className="tenant-stat-value truncate max-w-[8rem]">
                {currentWorkspace?.name ?? "—"}
              </span>
              <span className="tenant-stat-label">当前</span>
            </div>
          </div>
        </div>
        {currentWorkspace ? (
          <div className="tenant-hero-pill">
            <span className="tenant-live-dot" aria-hidden />
            <span>
              {currentOrg?.name ?? "组织"} / <strong>{currentWorkspace.name}</strong>
              <span className="tenant-hero-slug">{currentWorkspace.slug}</span>
            </span>
            <Link href="/dashboard/workplace" className="btn-secondary !py-1 !px-2.5 text-xs ml-auto shrink-0">
              进入控制台
            </Link>
          </div>
        ) : null}
      </section>

      <section className="section tenant-main">
        <div className="tenant-toolbar">
          <div className="tenant-search tenant-search-wide">
            <SearchIcon />
            <input
              className="tenant-search-input"
              placeholder="搜索组织或工作区..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <div className="tenant-toolbar-actions">
            <button type="button" className="btn" onClick={() => openWsDialog()}>
              新建工作区
            </button>
          </div>
        </div>

        {orgCount === 0 ? (
          <div className="tenant-empty-state">
            <div className="tenant-empty-art" aria-hidden>
              <BuildingIcon />
            </div>
            <h3 className="tenant-empty-title">开始配置您的租户</h3>
            <p className="tenant-empty-desc">
              第一步创建组织，第二步在其下添加工作区。整个过程约需 1 分钟。
            </p>
            <div className="tenant-empty-actions">
              <button type="button" className="btn" onClick={openOrgDialog}>
                创建组织
              </button>
              <button type="button" className="btn-secondary" onClick={() => openWsDialog()} disabled>
                创建工作区
              </button>
            </div>
          </div>
        ) : filteredOrgOptions.length === 0 ? (
          <div className="empty-state">未找到匹配「{query}」的组织或工作区</div>
        ) : (
          <div className="tenant-org-grid">
            {filteredOrgOptions
              .filter((org) => grouped.has(org.id))
              .map((org) => {
                const items = filterWorkspaceItems(grouped.get(org.id) ?? []);
                if (query.trim() && items.length === 0) return null;
                const accent = orgAccentColor(org.id);
                const orgMeta = orgMetaById.get(org.id);
                const readOnly = orgMeta ? isOrgReadOnly(orgMeta) : false;

                return (
                  <article
                    key={org.id}
                    className={`tenant-org-card${readOnly ? " tenant-org-card-readonly" : ""}`}
                    style={{ ["--org-accent" as string]: accent }}
                  >
                    <header className="tenant-org-header">
                      <OrgAvatar label={org.name} orgId={org.id} />
                      <div className="min-w-0 flex-1">
                        <p className="tenant-org-name">{org.name}</p>
                        <p className="tenant-org-slug">{org.slug}</p>
                      </div>
                      {readOnly ? (
                        <>
                          <span className="tenant-count-chip">{items.length} 工作区</span>
                          <span className="badge tenant-readonly-badge">仅查看</span>
                        </>
                      ) : (
                        <span className="tenant-count-chip">{items.length} 工作区</span>
                      )}
                      {!readOnly ? (
                        <button
                          type="button"
                          className="btn-secondary !py-1 !px-2.5 text-xs shrink-0"
                          onClick={() => setMembersDialogOrg(org)}
                        >
                          成员
                        </button>
                      ) : null}
                    </header>

                    <ul className="tenant-ws-list">
                      {items.map((ws) => {
                        const active = currentWorkspaceId === ws.id;
                        const wsReadOnly = isWorkspaceReadOnly(ws) || readOnly;
                        return (
                          <li key={ws.id}>
                            <div
                              className={`tenant-ws-row${active ? " tenant-ws-row-active" : ""}${wsReadOnly ? " tenant-ws-row-readonly" : ""}`}
                            >
                              <div className="tenant-ws-main">
                                <div className="tenant-ws-title-row">
                                  <p className="tenant-ws-name">{ws.name}</p>
                                  {active && !wsReadOnly ? (
                                    <span className="badge badge-primary">当前</span>
                                  ) : null}
                                  {wsReadOnly ? (
                                    <span className="badge tenant-readonly-badge">仅查看</span>
                                  ) : null}
                                </div>
                                <p className="tenant-ws-slug">{ws.slug}</p>
                                <div className="tenant-ws-meta">
                                  <span>项目 {ws.max_projects}</span>
                                  <span className="tenant-meta-dot" aria-hidden />
                                  <span>Agent {ws.max_agents}</span>
                                </div>
                              </div>
                              <div className="tenant-ws-actions">
                                {wsReadOnly ? null : active ? (
                                  <Link
                                    href="/dashboard/workplace"
                                    className="btn-secondary !py-1.5 !px-3 text-xs"
                                  >
                                    控制台
                                  </Link>
                                ) : (
                                  <button
                                    type="button"
                                    className="btn !py-1.5 !px-3 text-xs"
                                    onClick={() => onSelectWorkspace(ws.id)}
                                  >
                                    切换
                                  </button>
                                )}
                              </div>
                            </div>
                          </li>
                        );
                      })}
                    </ul>
                  </article>
                );
              })}

            {emptyOrgsWithoutWs
              .filter(
                (org) =>
                  !query.trim() ||
                  matchesQuery(org.name, query) ||
                  matchesQuery(org.slug, query),
              )
              .map((org) => {
                const orgMeta = orgMetaById.get(org.id);
                const readOnly = orgMeta ? isOrgReadOnly(orgMeta) : false;

                return (
                  <article
                    key={org.id}
                    className={`tenant-org-card tenant-org-card-empty${readOnly ? " tenant-org-card-readonly" : ""}`}
                    style={{ ["--org-accent" as string]: orgAccentColor(org.id) }}
                  >
                    <header className="tenant-org-header">
                      <OrgAvatar label={org.name} orgId={org.id} />
                      <div className="min-w-0 flex-1">
                        <p className="tenant-org-name">{org.name}</p>
                        <p className="tenant-org-slug">{org.slug}</p>
                      </div>
                      {readOnly ? (
                        <span className="badge tenant-readonly-badge">仅查看</span>
                      ) : (
                        <>
                          <span className="badge badge-warning">待配置</span>
                          <button
                            type="button"
                            className="btn-secondary !py-1 !px-2.5 text-xs shrink-0"
                            onClick={() => setMembersDialogOrg(org)}
                          >
                            成员
                          </button>
                        </>
                      )}
                    </header>
                    <div className="tenant-org-empty-body">
                      {readOnly ? (
                        <p>尚未加入此组织，仅可查看基本信息</p>
                      ) : (
                        <>
                          <p>尚未创建工作区</p>
                          <button
                            type="button"
                            className="btn-secondary text-xs !py-1.5 !px-3"
                            onClick={() => openWsDialog(org.id)}
                          >
                            立即创建
                          </button>
                        </>
                      )}
                    </div>
                  </article>
                );
              })}
          </div>
        )}
      </section>
    </div>
  );
}
