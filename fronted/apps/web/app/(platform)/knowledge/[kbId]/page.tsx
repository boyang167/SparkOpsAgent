import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { DocumentPanel } from "@/modules/knowledge/components/DocumentPanel";
import { KnowledgeSearchPanel } from "@/modules/knowledge/components/KnowledgeSearchPanel";
import { KnowledgeBaseSettings } from "@/modules/knowledge/components/KnowledgeBasePanel";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadDocuments, loadKnowledgeBase } from "@/server/loaders/knowledge";

interface KnowledgeDetailPageProps {
  params: Promise<{ kbId: string }>;
}

export default async function KnowledgeDetailPage({ params }: KnowledgeDetailPageProps) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { kbId } = await params;
  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ??
    (await loadWorkspaceId()) ??
    workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  let kb;
  try {
    kb = await loadKnowledgeBase(workspaceId, kbId);
  } catch {
    notFound();
  }

  const documents = await loadDocuments(workspaceId, kbId);

  return (
    <div className="stack-lg">
      <Link href="/knowledge" className="link-back">
        ← 返回知识库列表
      </Link>

      <PageHeader
        eyebrow="Knowledge Base"
        title={kb.name}
        description={kb.description || undefined}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <span className="badge badge-neutral">{kb.status}</span>
            <KnowledgeBaseSettings workspaceId={workspaceId} kb={kb} />
          </div>
        }
      />

      <section className="section">
        <DocumentPanel workspaceId={workspaceId} kbId={kbId} documents={documents} />
      </section>

      <KnowledgeSearchPanel workspaceId={workspaceId} kbId={kbId} />
    </div>
  );
}
