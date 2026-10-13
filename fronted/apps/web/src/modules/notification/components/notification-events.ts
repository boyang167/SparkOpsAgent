export const NOTIFICATIONS_CHANGED_EVENT = "sparkops:notifications-changed";

export function emitNotificationsChanged(unread?: number) {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent(NOTIFICATIONS_CHANGED_EVENT, { detail: { unread } }));
}

export function unreadBadgeText(count: number): string {
  if (count <= 0) return "";
  return count > 99 ? "99+" : String(count);
}
