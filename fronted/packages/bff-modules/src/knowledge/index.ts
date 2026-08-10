import type { BffContext } from "@sparkops/bff-core";
import type {
  CreateDocumentCommand,
  CreateKnowledgeBaseCommand,
  DocumentVM,
  KnowledgeBaseVM,
  UpdateDocumentCommand,
  UpdateKnowledgeBaseCommand,
} from "@sparkops/contracts/knowledge";

function mapKnowledgeBase(data: KnowledgeBaseVM): KnowledgeBaseVM {
  return {
    id: data.id,
    name: data.name,
    description: data.description ?? "",
    status: data.status,
  };
}

function mapDocument(data: DocumentVM): DocumentVM {
  return {
    id: data.id,
    knowledge_base_id: data.knowledge_base_id,
    title: data.title,
    content: data.content ?? "",
    status: data.status,
  };
}

export async function listKnowledgeBases(
  ctx: BffContext,
  workspaceId: string,
): Promise<KnowledgeBaseVM[]> {
  const data = await ctx.upstream.knowledge.listKnowledgeBases(workspaceId);
  return data.items.map(mapKnowledgeBase);
}

export async function createKnowledgeBase(
  ctx: BffContext,
  workspaceId: string,
  input: CreateKnowledgeBaseCommand,
): Promise<KnowledgeBaseVM> {
  const data = await ctx.upstream.knowledge.createKnowledgeBase(workspaceId, input);
  return mapKnowledgeBase(data);
}

export async function getKnowledgeBase(
  ctx: BffContext,
  workspaceId: string,
  kbId: string,
): Promise<KnowledgeBaseVM> {
  const data = await ctx.upstream.knowledge.getKnowledgeBase(workspaceId, kbId);
  return mapKnowledgeBase(data);
}

export async function updateKnowledgeBase(
  ctx: BffContext,
  workspaceId: string,
  kbId: string,
  input: UpdateKnowledgeBaseCommand,
): Promise<KnowledgeBaseVM> {
  const data = await ctx.upstream.knowledge.updateKnowledgeBase(workspaceId, kbId, input);
  return mapKnowledgeBase(data);
}

export async function archiveKnowledgeBase(
  ctx: BffContext,
  workspaceId: string,
  kbId: string,
): Promise<{ deleted: true }> {
  return ctx.upstream.knowledge.archiveKnowledgeBase(workspaceId, kbId);
}

export async function listDocuments(
  ctx: BffContext,
  workspaceId: string,
  kbId: string,
): Promise<DocumentVM[]> {
  const data = await ctx.upstream.knowledge.listDocuments(workspaceId, kbId);
  return data.items.map(mapDocument);
}

export async function createDocument(
  ctx: BffContext,
  workspaceId: string,
  kbId: string,
  input: CreateDocumentCommand,
): Promise<DocumentVM> {
  const data = await ctx.upstream.knowledge.createDocument(workspaceId, kbId, input);
  return mapDocument(data);
}

export async function updateDocument(
  ctx: BffContext,
  workspaceId: string,
  docId: string,
  input: UpdateDocumentCommand,
): Promise<DocumentVM> {
  const data = await ctx.upstream.knowledge.updateDocument(workspaceId, docId, input);
  return mapDocument(data);
}

export async function archiveDocument(
  ctx: BffContext,
  workspaceId: string,
  docId: string,
): Promise<{ deleted: true }> {
  return ctx.upstream.knowledge.archiveDocument(workspaceId, docId);
}

export async function searchDocuments(
  ctx: BffContext,
  workspaceId: string,
  kbId: string,
  input: { query: string; limit?: number },
): Promise<DocumentVM[]> {
  const data = await ctx.upstream.knowledge.searchDocuments(workspaceId, kbId, input);
  return data.items.map(mapDocument);
}
