import { PageHeader } from "@/components/ui/PageHeader";
import { ConsulKvPanel } from "@/modules/infra/components/ConsulKvPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

export default async function ConsulKvPage() {
  const { workspaceId } = await loadPlatformPageContext();

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Config"
        title="Consul KV"
        description="配置中心键值管理（Greenfield Mock，数据存 PostgreSQL）"
      />
      <ConsulKvPanel workspaceId={workspaceId} />
    </div>
  );
}
