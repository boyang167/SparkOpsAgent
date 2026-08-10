import { z } from "zod";

export const StartChatCommandSchema = z.object({
  message: z.string().min(1),
  conversation_id: z.string().uuid().optional(),
});

export type StartChatCommand = z.infer<typeof StartChatCommandSchema>;

export const ChatStartVMSchema = z.object({
  run_id: z.string(),
  conversation_id: z.string(),
});

export type ChatStartVM = z.infer<typeof ChatStartVMSchema>;

export const MessageVMSchema = z.object({
  id: z.string(),
  role: z.string(),
  content: z.string(),
  created_at: z.string(),
});

export type MessageVM = z.infer<typeof MessageVMSchema>;

export const ConversationSummaryVMSchema = z.object({
  id: z.string(),
  preview: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});

export type ConversationSummaryVM = z.infer<typeof ConversationSummaryVMSchema>;

export const ConversationListVMSchema = z.object({
  items: z.array(ConversationSummaryVMSchema),
});

export type ConversationListVM = z.infer<typeof ConversationListVMSchema>;

export const StreamEventSchema = z.object({
  delta: z.string().optional(),
  done: z.string().optional(),
});

export type StreamEvent = z.infer<typeof StreamEventSchema>;
