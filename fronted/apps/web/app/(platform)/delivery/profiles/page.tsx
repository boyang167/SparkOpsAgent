import { redirect } from "next/navigation";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProfileListPanel } from "@/modules/delivery/cd/components/ProfileEditor/ProfileListPanel";
import { loadSession } from "@/server/loaders/session";

export default async function DeliveryProfilesPage() {
  const session = await loadSession();
  if (!session) redirect("/login");

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Delivery Center"
        title="Environment Profile"
        description="按环境配置 replicas、Ingress、资源等差异参数（当前为 Mock 数据）"
      />
      <ProfileListPanel />
    </div>
  );
}
