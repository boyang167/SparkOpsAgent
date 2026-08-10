import type { NotificationListVM } from "@sparkops/contracts/notification";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createNotificationUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listNotifications(workspaceId: string) {
      return upstreamFetch<NotificationListVM>("/api/v1/notifications", {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async markRead(workspaceId: string, notificationId: string) {
      return upstreamFetch<{ read: true }>(`/api/v1/notifications/${notificationId}/read`, {
        method: "PATCH",
        options: { ...options, workspaceId },
      });
    },

    async markAllRead(workspaceId: string) {
      return upstreamFetch<{ updated: number }>("/api/v1/notifications/read-all", {
        method: "POST",
        options: { ...options, workspaceId },
      });
    },
  };
}

export type NotificationUpstream = ReturnType<typeof createNotificationUpstream>;
