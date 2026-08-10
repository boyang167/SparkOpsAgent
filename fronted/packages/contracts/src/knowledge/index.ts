import { z } from "zod";

export const KnowledgeBaseVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  status: z.string(),
});

export type KnowledgeBaseVM = z.infer<typeof KnowledgeBaseVMSchema>;

export const DocumentVMSchema = z.object({
  id: z.string(),
  knowledge_base_id: z.string().optional(),
  title: z.string(),
  content: z.string(),
  status: z.string(),
});

export type DocumentVM = z.infer<typeof DocumentVMSchema>;

export const CreateKnowledgeBaseCommandSchema = z.object({
  name: z.string().min(1).max(255),
  description: z.string().optional().default(""),
});

export type CreateKnowledgeBaseCommand = z.infer<typeof CreateKnowledgeBaseCommandSchema>;

export const UpdateKnowledgeBaseCommandSchema = CreateKnowledgeBaseCommandSchema;

export type UpdateKnowledgeBaseCommand = z.infer<typeof UpdateKnowledgeBaseCommandSchema>;

export const CreateDocumentCommandSchema = z.object({
  title: z.string().min(1).max(255),
  content: z.string().optional().default(""),
});

export type CreateDocumentCommand = z.infer<typeof CreateDocumentCommandSchema>;

export const UpdateDocumentCommandSchema = CreateDocumentCommandSchema;

export type UpdateDocumentCommand = z.infer<typeof UpdateDocumentCommandSchema>;

export const SearchDocumentsCommandSchema = z.object({
  query: z.string().min(1),
  limit: z.number().int().positive().optional(),
});

export type SearchDocumentsCommand = z.infer<typeof SearchDocumentsCommandSchema>;
