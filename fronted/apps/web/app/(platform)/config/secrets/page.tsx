import { PageHeader } from "@/components/ui/PageHeader";
import { SecretPanel } from "@/modules/infra/components/SecretPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function SecretsPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Infrastructure"
        title="服务连接配置管理"
        description="管理各种服务的连接和认证信息，如 GitLab、Jenkins、Harbor 等"
      />
      <SecretPanel workspaceId={workspaceId} />
    </div>
  );
}
