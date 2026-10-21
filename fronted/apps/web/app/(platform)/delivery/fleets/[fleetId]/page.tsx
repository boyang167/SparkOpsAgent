import { redirect } from "next/navigation";
import Link from "next/link";
import { PageHeader } from "@/components/ui/PageHeader";
import { FleetEditorPage } from "@/modules/delivery/cd/components/FleetEditor/FleetEditorPage";
import { loadSession } from "@/server/loaders/session";

interface PageProps {
  params: Promise<{ fleetId: string }>;
}

export default async function FleetEditorRoutePage({ params }: PageProps) {
  const session = await loadSession();
  if (!session) redirect("/login");
  const { fleetId } = await params;

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Fleet 编辑器" title="Wave 拓扑" description="配置目标集群与分波顺序（Mock）" />
      <Link href="/delivery/fleets" className="link-back">
        返回 Fleet 列表
      </Link>
      <FleetEditorPage fleetId={fleetId} />
    </div>
  );
}
