import { PageHeader } from "@/components/ui/PageHeader";
import { PlatformSettingsPanel } from "@/modules/platform_admin/components/PlatformSettingsPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformSettingsPage() {
  await requirePlatformConsoleAccess();
  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="平台运营"
        title="平台配置"
        description="管理全平台功能模块开关，以及 SSO / License 等企业级配置（部分能力预留）。"
      />
      <PlatformSettingsPanel />
    </div>
  );
}
