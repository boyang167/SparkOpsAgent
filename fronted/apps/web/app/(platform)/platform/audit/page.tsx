import { PlatformAuditLogsPanel } from "@/modules/tenant/components/PlatformAuditLogsPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformAuditPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="platform-page">
      <PlatformAuditLogsPanel variant="platform" />
    </div>
  );
}
