import { PlatformSettingsPanel } from "@/modules/platform_admin/components/PlatformSettingsPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformSettingsPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="platform-page">
      <PlatformSettingsPanel variant="platform" />
    </div>
  );
}
