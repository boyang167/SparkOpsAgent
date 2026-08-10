import { z } from "zod";

export const ModelVMSchema = z.object({
  id: z.string(),
  provider: z.string(),
});

export type ModelVM = z.infer<typeof ModelVMSchema>;

export const ModelListVMSchema = z.object({
  items: z.array(ModelVMSchema),
});

export type ModelListVM = z.infer<typeof ModelListVMSchema>;
