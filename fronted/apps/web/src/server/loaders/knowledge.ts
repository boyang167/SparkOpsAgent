import {
  getKnowledgeBase,
  listDocuments,
  listKnowledgeBases,
} from "@sparkops/bff-modules/knowledge";
import type { DocumentVM, KnowledgeBaseVM } from "@sparkops/contracts/knowledge";
import { loadBffContext } from "./context";

export async function loadKnowledgeBases(workspaceId: string): Promise<KnowledgeBaseVM[]> {
  const ctx = await loadBffContext();
  return listKnowledgeBases(ctx, workspaceId);
}

export async function loadKnowledgeBase(
  workspaceId: string,
  kbId: string,
): Promise<KnowledgeBaseVM> {
  const ctx = await loadBffContext();
  return getKnowledgeBase(ctx, workspaceId, kbId);
}

export async function loadDocuments(workspaceId: string, kbId: string): Promise<DocumentVM[]> {
  const ctx = await loadBffContext();
  return listDocuments(ctx, workspaceId, kbId);
}
