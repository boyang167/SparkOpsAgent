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
  message: z.string().optional(),
});

export type PodVM = z.infer<typeof PodVMSchema>;

export const ServicePortOverviewVMSchema = z.object({
  port: z.number(),
  target_port: z.string(),
  protocol: z.string(),
  name: z.string().optional(),
});

export const ServiceOverviewVMSchema = z.object({
  name: z.string(),
  namespace: z.string(),
  labels: z.record(z.string()).optional(),
  annotations: z.record(z.string()).optional(),
  selector: z.record(z.string()).optional(),
  ports: z.array(ServicePortOverviewVMSchema).optional(),
  endpoints: z.array(z.string()).optional(),
  created_at: z.string().optional(),
});

export const DeploymentOverviewVMSchema = z.object({
  name: z.string(),
  namespace: z.string(),
  labels: z.record(z.string()).optional(),
  image: z.string().optional(),
  image_tag: z.string().optional(),
  strategy: z.string().optional(),
  replicas: z.number(),
  available_replicas: z.number(),
  unavailable_replicas: z.number(),
  memory_limit: z.string().optional(),
  uid: z.string().optional(),
  created_at: z.string().optional(),
});

export const ProjectRuntimeOverviewVMSchema = z.object({
  deployment: DeploymentOverviewVMSchema.optional(),
  service: ServiceOverviewVMSchema.optional(),
  pods: z.array(PodVMSchema),
});

export type ServicePortOverviewVM = z.infer<typeof ServicePortOverviewVMSchema>;
export type ServiceOverviewVM = z.infer<typeof ServiceOverviewVMSchema>;
export type DeploymentOverviewVM = z.infer<typeof DeploymentOverviewVMSchema>;
export type ProjectRuntimeOverviewVM = z.infer<typeof ProjectRuntimeOverviewVMSchema>;
