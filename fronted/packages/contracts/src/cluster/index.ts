import { z } from "zod";

export const PodVMSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  name: z.string(),
  namespace: z.string(),
  phase: z.string(),
  node: z.string(),
  restarts: z.number(),
  container: z.string(),
  image: z.string(),
  started_at: z.string(),
});

export type PodVM = z.infer<typeof PodVMSchema>;
