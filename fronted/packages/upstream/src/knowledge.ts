import type { DocumentVM, KnowledgeBaseVM } from "@sparkops/contracts/knowledge";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createKnowledgeUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listKnowledgeBases(workspaceId: string) {
      return upstreamFetch<{ items: KnowledgeBaseVM[] }>(
        `/api/v1/workspaces/${workspaceId}/knowledge-bases`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createKnowledgeBase(
      workspaceId: string,
      input: { name: string; description?: string },
    ) {
      return upstreamFetch<KnowledgeBaseVM>(`/api/v1/workspaces/${workspaceId}/knowledge-bases`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async getKnowledgeBase(workspaceId: string, kbId: string) {
      return upstreamFetch<KnowledgeBaseVM>(`/api/v1/knowledge-bases/${kbId}`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async updateKnowledgeBase(
      workspaceId: string,
      kbId: string,
      input: { name: string; description?: string },
    ) {
      return upstreamFetch<KnowledgeBaseVM>(`/api/v1/knowledge-bases/${kbId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async archiveKnowledgeBase(workspaceId: string, kbId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/knowledge-bases/${kbId}`, {
        method: "DELETE",
        options: { ...options, workspaceId },
      });
    },

    async listDocuments(workspaceId: string, kbId: string) {
      return upstreamFetch<{ items: DocumentVM[] }>(`/api/v1/knowledge-bases/${kbId}/documents`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async createDocument(
      workspaceId: string,
      kbId: string,
      input: { title: string; content?: string },
    ) {
      return upstreamFetch<DocumentVM>(`/api/v1/knowledge-bases/${kbId}/documents`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async updateDocument(
      workspaceId: string,
      docId: string,
      input: { title: string; content?: string },
    ) {
      return upstreamFetch<DocumentVM>(`/api/v1/documents/${docId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async archiveDocument(workspaceId: string, docId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/documents/${docId}`, {
        method: "DELETE",
        options: { ...options, workspaceId },
      });
    },

    async searchDocuments(workspaceId: string, kbId: string, input: { query: string; limit?: number }) {
      return upstreamFetch<{ items: DocumentVM[] }>(`/api/v1/knowledge-bases/${kbId}/search`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },
  };
}

export type KnowledgeUpstream = ReturnType<typeof createKnowledgeUpstream>;
