import { redirect } from "next/navigation";
import { PageHeader } from "@/components/ui/PageHeader";
import { FleetBoardMock } from "@/modules/delivery/cd/components/FleetBoard/FleetBoardMock";
import { loadSession } from "@/server/loaders/session";

interface PageProps {
  params: Promise<{ runId: string }>;
}

export default async function DeliveryRunPage({ params }: PageProps) {
  const session = await loadSession();
  if (!session) redirect("/login");
  const { runId } = await params;

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Fleet Board" title={`Run ${runId}`} description="发布执行观测（Mock 占位）" />
      <FleetBoardMock runId={runId} />
    </div>
  );
}
