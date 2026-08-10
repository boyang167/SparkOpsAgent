import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { CreateKnowledgeBaseForm } from "@/modules/knowledge/components/KnowledgeBasePanel";
import { KnowledgeBaseList } from "@/modules/knowledge/components/KnowledgeBaseList";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadKnowledgeBases } from "@/server/loaders/knowledge";

export default async function KnowledgePage() {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ??
    (await loadWorkspaceId()) ??
    workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  const knowledgeBases = await loadKnowledgeBases(workspaceId);

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Knowledge"
        title="知识库"
        description="管理工作区内的知识库与文档"
        actions={<CreateKnowledgeBaseForm workspaceId={workspaceId} />}
      />
      <KnowledgeBaseList knowledgeBases={knowledgeBases} />
    </div>
  );
}
