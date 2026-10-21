import { redirect } from "next/navigation";
import Link from "next/link";
import { PageHeader } from "@/components/ui/PageHeader";
import { PipelineStudio } from "@/modules/delivery/cd/components/PipelineStudio/PipelineStudio";
import { loadSession } from "@/server/loaders/session";

interface PageProps {
  params: Promise<{ pipelineId: string }>;
}

export default async function PipelineStudioPage({ params }: PageProps) {
  const session = await loadSession();
  if (!session) redirect("/login");
  const { pipelineId } = await params;

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Pipeline Studio" title="编排画布" description="从 Step 库拖拽编排 DAG，配置并行 / 顺序 / 分波（Mock）" />
      <Link href="/delivery/pipelines" className="link-back">
        返回 Pipeline 列表
      </Link>
      <PipelineStudio pipelineId={pipelineId} />
    </div>
  );
}
