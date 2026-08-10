import { redirect } from "next/navigation";
import { PlatformAuditLogsPanel } from "@/modules/tenant/components/PlatformAuditLogsPanel";
import { loadSession } from "@/server/loaders/session";

export default async function MonitorPlatformAuditPage() {
  const session = await loadSession();
  if (!session) redirect("/login");
  if (!session.is_super_admin) redirect("/dashboard");

  return <PlatformAuditLogsPanel showBreadcrumb={false} />;
}
