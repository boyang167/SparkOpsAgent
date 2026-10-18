import { z } from "zod";

export const McpToolTierSchema = z.enum(["discovery", "read", "action"]);

export type McpToolTier = z.infer<typeof McpToolTierSchema>;

export const McpToolVMSchema = z.object({
  name: z.string(),
  description: z.string(),
  inputSchema: z.record(z.unknown()),
  tier: McpToolTierSchema.optional(),
});

export type McpToolVM = z.infer<typeof McpToolVMSchema>;

export const McpToolListVMSchema = z.object({
  items: z.array(McpToolVMSchema),
});

export type McpToolListVM = z.infer<typeof McpToolListVMSchema>;
