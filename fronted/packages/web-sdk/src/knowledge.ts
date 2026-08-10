import type { DocumentVM, KnowledgeBaseVM } from "@sparkops/contracts/knowledge";
import { bffFetch, type BffFetchOptions } from "./client";

export async function listKnowledgeBases(workspaceId: string) {
  const options: BffFetchOptions = { workspaceId };
  const data = await bffFetch<{ items: KnowledgeBaseVM[] }>(
    `/workspaces/${workspaceId}/knowledge-bases`,
    options,
  );
  return data.items;
}

export function createKnowledgeBase(
  workspaceId: string,
  input: { name: string; description?: string },
) {
  return bffFetch<KnowledgeBaseVM>(`/workspaces/${workspaceId}/knowledge-bases`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function getKnowledgeBase(kbId: string, workspaceId: string) {
  return bffFetch<KnowledgeBaseVM>(`/knowledge-bases/${kbId}`, { workspaceId });
}

export function updateKnowledgeBase(
  kbId: string,
  workspaceId: string,
  input: { name: string; description?: string },
) {
  return bffFetch<KnowledgeBaseVM>(`/knowledge-bases/${kbId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function archiveKnowledgeBase(kbId: string, workspaceId: string) {
  return bffFetch<{ deleted: true }>(`/knowledge-bases/${kbId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export async function listDocuments(kbId: string, workspaceId: string) {
  const data = await bffFetch<{ items: DocumentVM[] }>(`/knowledge-bases/${kbId}/documents`, {
    workspaceId,
  });
  return data.items;
}

export function createDocument(
  kbId: string,
  workspaceId: string,
  input: { title: string; content?: string },
) {
  return bffFetch<DocumentVM>(`/knowledge-bases/${kbId}/documents`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function updateDocument(
  docId: string,
  workspaceId: string,
  input: { title: string; content?: string },
) {
  return bffFetch<DocumentVM>(`/documents/${docId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function archiveDocument(docId: string, workspaceId: string) {
  return bffFetch<{ deleted: true }>(`/documents/${docId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export function searchDocuments(
  kbId: string,
  workspaceId: string,
  input: { query: string; limit?: number },
) {
  return bffFetch<{ items: DocumentVM[] }>(`/knowledge-bases/${kbId}/search`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}
