import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { ConversationSidebar } from "@/modules/agent/components/ConversationSidebar";
import { ChatPanel } from "@/modules/agent/components/ChatPanel";
import { AgentVersionPanel } from "@/modules/agent/components/AgentVersionPanel";
import { AgentSettingsPanel } from "@/modules/agent/components/AgentSettings";
import { PublishAgentButton } from "@/modules/agent/components/PublishAgentButton";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadProject } from "@/server/loaders/projects";
import { loadAgent } from "@/server/loaders/agents";
import { loadBffContext } from "@/server/loaders/context";
import { getConversation } from "@sparkops/bff-modules/runtime";
import { loadConversations } from "@/server/loaders/conversations";
import { loadKnowledgeBases } from "@/server/loaders/knowledge";
import type { MessageVM } from "@sparkops/contracts/runtime";

interface AgentChatPageProps {
  params: Promise<{ projectId: string; agentId: string }>;
  searchParams: Promise<{ conversationId?: string }>;
}

export default async function AgentChatPage({ params, searchParams }: AgentChatPageProps) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { projectId, agentId } = await params;
  const { conversationId } = await searchParams;
  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId = cookieStore.get(WORKSPACE_COOKIE)?.value ?? workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  let project;
  try {
    project = await loadProject(workspaceId, projectId);
  } catch {
    notFound();
  }

  let agent;
  try {
    agent = await loadAgent(workspaceId, agentId);
  } catch {
    notFound();
  }

  const latestVersion = agent.versions?.[agent.versions.length - 1];
  const isPublished = agent.status === "published";
  const knowledgeBases = await loadKnowledgeBases(workspaceId);

  const ctx = await loadBffContext();
  let initialMessages: MessageVM[] = [];
  let conversations: Awaited<ReturnType<typeof loadConversations>> = [];
  if (isPublished) {
    try {
      conversations = await loadConversations(agentId);
    } catch {
      conversations = [];
    }
  }
  if (conversationId) {
    try {
      initialMessages = await getConversation(ctx, conversationId);
    } catch {
      initialMessages = [];
    }
  }

  const versionInfo = latestVersion
    ? `v${latestVersion.version} · ${latestVersion.model_name}`
    : undefined;

  return (
    <div className="stack-lg">
      <Link href={`/projects/${projectId}`} className="link-back">
        ← 返回 {project.name}
      </Link>

      <PageHeader
        eyebrow="Agent Chat"
        title={agent.name}
        description={versionInfo}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <span className={`badge ${isPublished ? "badge-success" : "badge-warning"}`}>
              {agent.status}
            </span>
            {!isPublished && latestVersion ? (
              <PublishAgentButton
                agentId={agentId}
                workspaceId={workspaceId}
                versionId={latestVersion.id}
              />
            ) : null}
          </div>
        }
      />

      <AgentSettingsPanel
        workspaceId={workspaceId}
        agentId={agentId}
        projectId={projectId}
        agentName={agent.name}
      />

      {!isPublished ? (
        <div className="alert alert-warning">
          该 Agent 尚未发布，请先点击「发布 Agent」后再开始对话。
        </div>
      ) : null}

      <AgentVersionPanel
        agentId={agentId}
        workspaceId={workspaceId}
        versions={agent.versions ?? []}
        isPublished={isPublished}
        knowledgeBases={knowledgeBases}
      />

      <div className="flex flex-col lg:flex-row gap-4 items-start">
        {isPublished ? (
          <ConversationSidebar
            projectId={projectId}
            agentId={agentId}
            conversations={conversations}
            currentConversationId={conversationId}
          />
        ) : null}
        <div className="flex-1 w-full">
          <ChatPanel
            agentId={agentId}
            workspaceId={workspaceId}
            initialConversationId={conversationId}
            initialMessages={initialMessages}
            canChat={isPublished}
          />
        </div>
      </div>
    </div>
  );
}
