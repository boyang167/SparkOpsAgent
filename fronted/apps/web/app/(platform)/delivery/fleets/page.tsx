import { redirect } from "next/navigation";
import { PageHeader } from "@/components/ui/PageHeader";
import { FleetListPanel } from "@/modules/delivery/cd/components/FleetEditor/FleetListPanel";
import { loadSession } from "@/server/loaders/session";

export default async function DeliveryFleetsPage() {
  const session = await loadSession();
  if (!session) redirect("/login");

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Delivery Center"
        title="Fleet 集群组"
        description="配置目标集群与 Wave 分波拓扑（当前为 Mock 数据）"
      />
      <FleetListPanel />
    </div>
  );
}
