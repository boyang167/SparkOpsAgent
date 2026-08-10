import { z } from "zod";

export const ApiEnvelopeSchema = z.object({
  code: z.number(),
  message: z.string(),
  data: z.unknown().optional(),
  trace_id: z.string(),
});

export type ApiEnvelope<T = unknown> = {
  code: number;
  message: string;
  data?: T;
  trace_id: string;
};

export const ApiErrorSchema = z.object({
  code: z.number(),
  message: z.string(),
  data: z
    .object({
      error_code: z.string().optional(),
    })
    .optional(),
  trace_id: z.string(),
});

export class BffError extends Error {
  constructor(
    message: string,
    public readonly code: string,
    public readonly status: number,
    public readonly traceId?: string,
  ) {
    super(message);
    this.name = "BffError";
  }
}

export const PageResultSchema = <T extends z.ZodTypeAny>(item: T) =>
  z.object({
    items: z.array(item),
    total: z.number().optional(),
  });
