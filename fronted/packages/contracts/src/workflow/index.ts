import { z } from "zod";

export const WorkflowVMSchema = z.object({
  id: z.string(),
  project_id: z.string().optional(),
  name: z.string(),
  description: z.string(),
  status: z.string(),
  definition: z.string().optional(),
});

export type WorkflowVM = z.infer<typeof WorkflowVMSchema>;

export const CreateWorkflowCommandSchema = z.object({
  name: z.string().min(1).max(255),
  description: z.string().optional().default(""),
  definition: z.string().optional().default("{}"),
});

export type CreateWorkflowCommand = z.infer<typeof CreateWorkflowCommandSchema>;

export const UpdateWorkflowCommandSchema = CreateWorkflowCommandSchema;

export type UpdateWorkflowCommand = z.infer<typeof UpdateWorkflowCommandSchema>;

export const WorkflowRunVMSchema = z.object({
  id: z.string(),
  workflow_id: z.string(),
  status: z.string(),
  input: z.string().optional(),
  output: z.string().optional(),
  error: z.string().optional(),
  started_at: z.string(),
  completed_at: z.string().optional(),
});

export type WorkflowRunVM = z.infer<typeof WorkflowRunVMSchema>;

export const StartWorkflowRunCommandSchema = z.object({
  input: z.string().optional().default(""),
});

export type StartWorkflowRunCommand = z.infer<typeof StartWorkflowRunCommandSchema>;
