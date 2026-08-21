import { PageHeader } from "@/components/ui/PageHeader";
import { PlatformAuditLogsPanel } from "@/modules/tenant/components/PlatformAuditLogsPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformAuditPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="平台运营" title="平台审计" description="跨租户操作与变更审计记录。" />
      <PlatformAuditLogsPanel showBreadcrumb={false} />
    </div>
  );
}
