import { z } from "zod";

export const BuildVMSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  status: z.string(),
  branch: z.string(),
  build_number: z.number(),
  jenkins_job: z.string().optional(),
  console_log: z.string().optional(),
  error: z.string().optional(),
  started_at: z.string(),
  completed_at: z.string().optional(),
  rollback_of_build_id: z.string().optional(),
});

export type BuildVM = z.infer<typeof BuildVMSchema>;

export const TriggerBuildCommandSchema = z.object({
  branch: z.string().optional().default("main"),
});

export type TriggerBuildCommand = z.infer<typeof TriggerBuildCommandSchema>;

export const CronJobVMSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  name: z.string(),
  schedule: z.string(),
  branch: z.string().optional(),
  enabled: z.boolean().optional(),
  created_at: z.string().optional(),
});

export type CronJobVM = z.infer<typeof CronJobVMSchema>;
