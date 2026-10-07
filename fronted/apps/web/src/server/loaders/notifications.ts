import { listNotifications } from "@sparkops/bff-modules/notification";
import type { NotificationListVM } from "@sparkops/contracts/notification";
import { loadBffContext } from "./context";

export async function loadNotifications(workspaceId: string): Promise<NotificationListVM> {
  const ctx = await loadBffContext();
  try {
    return await listNotifications(ctx, workspaceId);
  } catch (err) {
    console.error("[notifications] list failed", err);
    return { items: [], unread_count: 0 };
  }
}
