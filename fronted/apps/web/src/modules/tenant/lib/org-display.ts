export interface OrgMeta {
  id: string;
  name: string;
  slug: string;
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
