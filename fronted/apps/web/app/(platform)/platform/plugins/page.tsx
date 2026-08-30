import { PlatformPluginsPanel } from "@/modules/plugin/components/PlatformPluginsPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformPluginsPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="platform-page">
      <PlatformPluginsPanel />
    </div>
  );
}
