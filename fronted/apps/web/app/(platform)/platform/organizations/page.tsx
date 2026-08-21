import { PageHeader } from "@/components/ui/PageHeader";
import { PlatformOrganizationsPanel } from "@/modules/tenant/components/PlatformOrganizationsPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformOrganizationsPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="平台运营" title="平台组织" description="跨租户组织治理与 Owner 管理。" />
      <PlatformOrganizationsPanel />
    </div>
  );
}
