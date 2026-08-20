import { z } from "zod";

export const ProjectPortSchema = z.object({
  name: z.string().optional(),
  container_port: z.number().int(),
  service_port: z.number().int().optional(),
  protocol: z.enum(["TCP", "UDP"]).optional(),
});

export type ProjectPort = z.infer<typeof ProjectPortSchema>;

export const ProjectResourcesSchema = z.object({
  cpu_request: z.string().optional(),
  cpu_limit: z.string().optional(),
  memory_request: z.string().optional(),
  memory_limit: z.string().optional(),
  replicas: z.number().int().optional(),
  ports: z.array(ProjectPortSchema).optional(),
  command: z.string().optional(),
  args: z.string().optional(),
  project_config: z.string().optional(),
  service_template_id: z.string().optional(),
  deployment_template_id: z.string().optional(),
  jenkinsfile_template_id: z.string().optional(),
  registry_type: z.enum(["Harbor", "Docker_Registry"]).optional(),
  registry_secret_id: z.string().optional(),
  sonarqube_secret_id: z.string().optional(),
  enable_static_scan: z.boolean().optional(),
});

export type ProjectResources = z.infer<typeof ProjectResourcesSchema>;

export const ProjectReviewStatusSchema = z.enum(["none", "pending", "approved", "rejected"]);
export type ProjectReviewStatus = z.infer<typeof ProjectReviewStatusSchema>;

export const ProjectVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  display_name: z.string().optional(),
  description: z.string().optional().default(""),
  namespace: z.string().optional(),
  language: z.string().optional(),
  git_url: z.string().optional(),
  secret_id: z.string().optional(),
  git_ref_type: z.enum(["branch", "tag", "commit"]).optional(),
  git_ref_value: z.string().optional(),
  dockerfile_path: z.string().optional(),
  template_id: z.string().optional(),
  resources: ProjectResourcesSchema.optional(),
  status: z.string(),
  review_status: ProjectReviewStatusSchema.optional().default("none"),
  submitted_by: z.string().optional(),
  submitted_at: z.string().optional(),
  reviewed_by: z.string().optional(),
  reviewed_at: z.string().optional(),
  review_comment: z.string().optional().default(""),
  can_submit_review: z.boolean().optional().default(false),
  can_review: z.boolean().optional().default(false),
  created_at: z.string().optional(),
});

export type ProjectVM = z.infer<typeof ProjectVMSchema>;

export const ProjectListVMSchema = z.object({
  items: z.array(ProjectVMSchema),
});

export type ProjectListVM = z.infer<typeof ProjectListVMSchema>;

export const CreateProjectCommandSchema = z.object({
  name: z.string().min(2).max(50).regex(/^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$/),
  display_name: z.string().max(50).optional(),
  description: z.string().max(200).optional().default(""),
  namespace: z.string().min(1).max(63).optional(),
  language: z.string().optional(),
  git_url: z.string().optional(),
  secret_id: z.string().optional(),
  git_ref_type: z.enum(["branch", "tag", "commit"]).optional(),
  git_ref_value: z.string().optional(),
  dockerfile_path: z.string().optional(),
  template_id: z.string().optional(),
  resources: ProjectResourcesSchema.optional(),
});

export type CreateProjectCommand = z.infer<typeof CreateProjectCommandSchema>;

export const UpdateProjectCommandSchema = CreateProjectCommandSchema;

export type UpdateProjectCommand = z.infer<typeof UpdateProjectCommandSchema>;

export const DecideProjectReviewCommandSchema = z
  .object({
    decision: z.enum(["approved", "rejected"]),
    comment: z.string().optional().default(""),
  })
  .superRefine((value, ctx) => {
    if (value.decision === "rejected" && !value.comment.trim()) {
      ctx.addIssue({
        code: "custom",
        path: ["comment"],
        message: "驳回意见不能为空",
      });
    }
  });

export type DecideProjectReviewCommand = z.infer<typeof DecideProjectReviewCommandSchema>;

export const PROJECT_LANGUAGE_OPTIONS = [
  { value: "go", label: "Go", color: "#00ADD8" },
  { value: "java", label: "Java", color: "#ED8B00" },
  { value: "python", label: "Python", color: "#3776AB" },
  { value: "nodejs", label: "Node.js", color: "#339933" },
  { value: "dotnet", label: ".NET", color: "#512BD4" },
  { value: "php", label: "PHP", color: "#777BB4" },
  { value: "other", label: "其他", color: "#909399" },
] as const;

export const PROJECT_STATUS_OPTIONS = [
  { value: "active", label: "活跃" },
  { value: "archived", label: "已归档" },
] as const;

export function languageLabel(value?: string) {
  return PROJECT_LANGUAGE_OPTIONS.find((o) => o.value === value)?.label ?? value ?? "-";
}

export function languageColor(value?: string) {
  return PROJECT_LANGUAGE_OPTIONS.find((o) => o.value === value)?.color ?? "#909399";
}

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

export const GeneratedTemplateArtifactSchema = z.object({
  key: z.string(),
  kind: z.enum(["Service", "Deployment", "jenkinsFile"]),
  title: z.string(),
  filename: z.string(),
  content: z.string(),
  template_id: z.string().optional(),
  template_name: z.string().optional(),
});

export type GeneratedTemplateArtifact = z.infer<typeof GeneratedTemplateArtifactSchema>;

export const GeneratedTemplateListVMSchema = z.object({
  items: z.array(GeneratedTemplateArtifactSchema),
});

export type GeneratedTemplateListVM = z.infer<typeof GeneratedTemplateListVMSchema>;
