import { z } from "zod";

export const PluginRecordVMSchema = z.object({
  id: z.string(),
  version: z.string(),
  display_name: z.string(),
  status: z.string(),
  pr_url: z.string().optional(),
  enabled: z.boolean().optional(),
});

export type PluginRecordVM = z.infer<typeof PluginRecordVMSchema>;

export const PluginBindingVMSchema = z.object({
  workspace_id: z.string(),
  plugin_id: z.string(),
  enabled: z.boolean(),
  enabled_by: z.string().optional(),
  enabled_at: z.string().optional(),
});

export type PluginBindingVM = z.infer<typeof PluginBindingVMSchema>;

export const ExamplePingVMSchema = z.object({
  ok: z.boolean(),
});

export type ExamplePingVM = z.infer<typeof ExamplePingVMSchema>;
