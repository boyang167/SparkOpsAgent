import { listNotifications } from "@sparkops/bff-modules/notification";
import type { NotificationListVM } from "@sparkops/contracts/notification";
import { loadBffContext } from "./context";

export async function loadNotifications(workspaceId: string): Promise<NotificationListVM> {
  const ctx = await loadBffContext();
  return listNotifications(ctx, workspaceId);
}
