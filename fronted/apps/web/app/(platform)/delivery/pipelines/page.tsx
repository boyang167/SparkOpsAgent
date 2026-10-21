import { redirect } from "next/navigation";
import { PageHeader } from "@/components/ui/PageHeader";
import { PipelineListPanel } from "@/modules/delivery/cd/components/PipelineStudio/PipelineListPanel";
import { loadSession } from "@/server/loaders/session";

export default async function DeliveryPipelinesPage() {
  const session = await loadSession();
  if (!session) redirect("/login");

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Delivery Center"
        title="Pipeline 模板"
        description="声明式编排 DAG，在 Studio 中配置「怎么发」（当前为 Mock 数据）"
      />
      <PipelineListPanel />
    </div>
  );
}
