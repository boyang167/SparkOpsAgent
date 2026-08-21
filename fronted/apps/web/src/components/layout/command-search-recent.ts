const STORAGE_KEY = "sparkops:command-search:recent";
const MAX_RECENT = 10;

export interface RecentVisit {
  href: string;
  label: string;
  group: string;
  visitedAt: number;
}

export function readRecentVisits(): RecentVisit[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as RecentVisit[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function recordRecentVisit(entry: Omit<RecentVisit, "visitedAt">) {
  if (typeof window === "undefined") return;
  const next: RecentVisit = { ...entry, visitedAt: Date.now() };
  const items = readRecentVisits().filter((item) => item.href !== next.href);
  items.unshift(next);
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(items.slice(0, MAX_RECENT)));
}

export function recentVisitsToSearchItems() {
  return readRecentVisits().map((item) => ({
    href: item.href,
    label: item.label,
    group: item.group,
    kind: "recent" as const,
  }));
}
