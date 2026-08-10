import { z } from "zod";

export const NotificationVMSchema = z.object({
  id: z.string(),
  type: z.string(),
  title: z.string(),
  body: z.string(),
  link: z.string(),
  read: z.boolean(),
  created_at: z.string(),
  read_at: z.string().optional(),
});

export type NotificationVM = z.infer<typeof NotificationVMSchema>;

export const NotificationListVMSchema = z.object({
  items: z.array(NotificationVMSchema),
  unread_count: z.number(),
});

export type NotificationListVM = z.infer<typeof NotificationListVMSchema>;
