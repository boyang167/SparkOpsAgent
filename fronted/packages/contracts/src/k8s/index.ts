import { z } from "zod";

export const K8sResourceItemSchema = z.object({
  name: z.string(),
  namespace: z.string().optional(),
  created_at: z.string().optional(),
  labels: z.record(z.string()).optional(),
  data: z.record(z.unknown()).optional(),
});

export type K8sResourceItem = z.infer<typeof K8sResourceItemSchema>;

export const K8sResourceListSchema = z.object({
  items: z.array(K8sResourceItemSchema),
});

export type K8sResourceList = z.infer<typeof K8sResourceListSchema>;
