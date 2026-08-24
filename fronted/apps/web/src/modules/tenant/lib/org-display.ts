export interface OrgMeta {
  id: string;
  name: string;
  slug: string;
  read_only?: boolean;
}

export function isOrgReadOnly(org: { read_only?: boolean }): boolean {
  return org.read_only === true;
}

export function isWorkspaceReadOnly(ws: { read_only?: boolean }): boolean {
  return ws.read_only === true;
}

const ORG_COLORS = ["#0ea5e9", "#8b5cf6", "#10b981", "#f59e0b", "#6366f1", "#ec4899"];

export function orgAccentColor(orgId: string): string {
  let hash = 0;
  for (let i = 0; i < orgId.length; i++) {
    hash = orgId.charCodeAt(i) + ((hash << 5) - hash);
  }
  return ORG_COLORS[Math.abs(hash) % ORG_COLORS.length] ?? ORG_COLORS[0];
}

export function orgInitial(label: string): string {
  const trimmed = label.trim();
  if (!trimmed) return "?";
  return trimmed.charAt(0).toUpperCase();
}

export function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

/** 补齐工作区所属但不在组织列表中的 org（如已删除/历史组织），避免统计与卡片不一致 */
export function extendOrgsWithWorkspaceOrgs(
  organizations: OrgMeta[],
  workspaces: ReadonlyArray<{ org_id: string }>,
): OrgMeta[] {
  const byId = new Map(organizations.map((org) => [org.id, org]));
  const extras: OrgMeta[] = [];

  for (const ws of workspaces) {
    if (byId.has(ws.org_id)) continue;
    if (extras.some((org) => org.id === ws.org_id)) continue;
    const stub: OrgMeta = {
      id: ws.org_id,
      name: "未关联组织",
      slug: shortId(ws.org_id),
      read_only: true,
    };
    extras.push(stub);
    byId.set(ws.org_id, stub);
  }

  return [...organizations, ...extras].sort((a, b) => a.name.localeCompare(b.name));
}
