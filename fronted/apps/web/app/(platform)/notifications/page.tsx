import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { NotificationPanel } from "@/modules/notification/components/NotificationPanel";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadNotifications } from "@/server/loaders/notifications";

export default async function NotificationsPage() {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ??
    (await loadWorkspaceId()) ??
    workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  const data = await loadNotifications(workspaceId);

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Notification"
        title="站内信"
        description="构建、工作流与 Agent 运行完成时的提醒"
      />
      <NotificationPanel
        workspaceId={workspaceId}
        items={data.items}
        unreadCount={data.unread_count}
      />
    </div>
  );
}
