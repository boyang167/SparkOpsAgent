import type { BffContext } from "@sparkops/bff-core";
import type { NotificationListVM, NotificationVM } from "@sparkops/contracts/notification";

function mapNotification(data: NotificationVM): NotificationVM {
  return {
    id: data.id,
    type: data.type,
    title: data.title,
    body: data.body,
    link: data.link,
    read: data.read,
    created_at: data.created_at,
    read_at: data.read_at,
  };
}

export async function listNotifications(
  ctx: BffContext,
  workspaceId: string,
): Promise<NotificationListVM> {
  const data = await ctx.upstream.notification.listNotifications(workspaceId);
  return {
    items: data.items.map(mapNotification),
    unread_count: data.unread_count,
  };
}

export async function markNotificationRead(
  ctx: BffContext,
  workspaceId: string,
  notificationId: string,
): Promise<{ read: true }> {
  return ctx.upstream.notification.markRead(workspaceId, notificationId);
}

export async function markAllNotificationsRead(
  ctx: BffContext,
  workspaceId: string,
): Promise<{ updated: number }> {
  return ctx.upstream.notification.markAllRead(workspaceId);
}
