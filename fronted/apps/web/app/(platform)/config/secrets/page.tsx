import { PageHeader } from "@/components/ui/PageHeader";
import { SecretPanel } from "@/modules/infra/components/SecretPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function SecretsPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Secret"
        title="服务连接配置"
        description="管理 GitLab、Jenkins、Harbor 等外部服务的连接与认证信息"
      />
      <SecretPanel workspaceId={workspaceId} />
    </div>
  );
}
