import { z } from "zod";

export const AgentVMSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  name: z.string(),
  status: z.string(),
  current_version_id: z.string().optional(),
});

export type AgentVM = z.infer<typeof AgentVMSchema>;

export const AgentListVMSchema = z.object({
  items: z.array(AgentVMSchema),
});

export type AgentListVM = z.infer<typeof AgentListVMSchema>;

export const CreateAgentCommandSchema = z.object({
  name: z.string().min(1).max(255),
  system_prompt: z.string().min(1),
  model_name: z.string().min(1),
});

export type CreateAgentCommand = z.infer<typeof CreateAgentCommandSchema>;

export const AgentVersionVMSchema = z.object({
  id: z.string(),
  version: z.number(),
  system_prompt: z.string(),
  model_name: z.string(),
  knowledge_base_ids: z.array(z.string()).optional(),
});

export type AgentVersionVM = z.infer<typeof AgentVersionVMSchema>;

export const AgentDetailVMSchema = AgentVMSchema.extend({
  versions: z.array(AgentVersionVMSchema).optional(),
});

export type AgentDetailVM = z.infer<typeof AgentDetailVMSchema>;

export const PublishAgentCommandSchema = z.object({
  version_id: z.string().uuid(),
});

export type PublishAgentCommand = z.infer<typeof PublishAgentCommandSchema>;

export const AddAgentVersionCommandSchema = z.object({
  system_prompt: z.string().min(1),
  model_name: z.string().min(1),
  knowledge_base_ids: z.array(z.string()).optional().default([]),
});

export type AddAgentVersionCommand = z.infer<typeof AddAgentVersionCommandSchema>;

export const AddAgentVersionResultSchema = z.object({
  id: z.string(),
  version: z.number(),
});

export type AddAgentVersionResult = z.infer<typeof AddAgentVersionResultSchema>;

export const UpdateAgentCommandSchema = z.object({
  name: z.string().min(1).max(255),
});

export type UpdateAgentCommand = z.infer<typeof UpdateAgentCommandSchema>;
