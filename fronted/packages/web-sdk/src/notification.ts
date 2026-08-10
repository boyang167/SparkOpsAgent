import type { NotificationListVM } from "@sparkops/contracts/notification";
import { bffFetch, type BffFetchOptions } from "./client";

export async function listNotifications(workspaceId: string) {
  const options: BffFetchOptions = { workspaceId };
  return bffFetch<NotificationListVM>("/notifications", options);
}

export function markRead(notificationId: string, workspaceId: string) {
  return bffFetch<{ read: true }>(`/notifications/${notificationId}/read`, {
    method: "PATCH",
    workspaceId,
  });
}

export function markAllRead(workspaceId: string) {
  return bffFetch<{ updated: number }>("/notifications/read-all", {
    method: "POST",
    workspaceId,
  });
}
