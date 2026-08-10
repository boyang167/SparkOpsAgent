import { z } from "zod";

export const ProjectVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  status: z.string(),
});

export type ProjectVM = z.infer<typeof ProjectVMSchema>;

export const ProjectListVMSchema = z.object({
  items: z.array(ProjectVMSchema),
});

export type ProjectListVM = z.infer<typeof ProjectListVMSchema>;

export const CreateProjectCommandSchema = z.object({
  name: z.string().min(1).max(255),
  description: z.string().optional().default(""),
});

export type CreateProjectCommand = z.infer<typeof CreateProjectCommandSchema>;

export const UpdateProjectCommandSchema = CreateProjectCommandSchema;

export type UpdateProjectCommand = z.infer<typeof UpdateProjectCommandSchema>;

export const ProjectMemberVMSchema = z.object({
  member_id: z.string(),
  email: z.string().email(),
  display_name: z.string(),
  role: z.enum(["lead", "member", "viewer"]),
  joined_at: z.string(),
});

export type ProjectMemberVM = z.infer<typeof ProjectMemberVMSchema>;

export const AddProjectMemberCommandSchema = z.object({
  member_id: z.string().uuid(),
  role: z.enum(["lead", "member", "viewer"]).optional().default("member"),
});

export type AddProjectMemberCommand = z.infer<typeof AddProjectMemberCommandSchema>;

export const UpdateProjectMemberRoleCommandSchema = z.object({
  role: z.enum(["lead", "member", "viewer"]),
});

export type UpdateProjectMemberRoleCommand = z.infer<typeof UpdateProjectMemberRoleCommandSchema>;
