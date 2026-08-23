import { PlatformOrganizationsPanel } from "@/modules/tenant/components/PlatformOrganizationsPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformOrganizationsPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="platform-page">
      <PlatformOrganizationsPanel variant="platform" />
    </div>
  );
}
