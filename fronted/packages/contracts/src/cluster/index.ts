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
  cpu_millicores: z.number().optional(),
  memory_bytes: z.number().optional(),
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
  cpu_request: z.string().optional(),
  cpu_limit: z.string().optional(),
  memory_request: z.string().optional(),
  memory_limit: z.string().optional(),
  uid: z.string().optional(),
  created_at: z.string().optional(),
});

export const LogCollectionOverviewVMSchema = z.object({
  configured: z.boolean(),
  path: z.string().optional(),
  pattern: z.string().optional(),
  file_suffix: z.string().optional(),
});

export const HealthProbeOverviewVMSchema = z.object({
  configured: z.boolean(),
  port: z.number().optional(),
  protocol: z.string().optional(),
  path: z.string().optional(),
  readiness: z.boolean(),
  liveness: z.boolean(),
  initial_delay_seconds: z.number().optional(),
  period_seconds: z.number().optional(),
  timeout_seconds: z.number().optional(),
  success_threshold: z.number().optional(),
  failure_threshold: z.number().optional(),
});

export const IngressPathOverviewVMSchema = z.object({
  path: z.string(),
  service_name: z.string(),
  service_port: z.number(),
});

export const IngressOverviewVMSchema = z.object({
  name: z.string(),
  host: z.string(),
  paths: z.array(IngressPathOverviewVMSchema).optional(),
  created_at: z.string().optional(),
});

export const ProjectRuntimeOverviewVMSchema = z.object({
  deployment: DeploymentOverviewVMSchema.optional(),
  service: ServiceOverviewVMSchema.optional(),
  log_collection: LogCollectionOverviewVMSchema.optional(),
  health_probe: HealthProbeOverviewVMSchema.optional(),
  ingress: IngressOverviewVMSchema.optional(),
  pods: z.array(PodVMSchema),
});

export type ServicePortOverviewVM = z.infer<typeof ServicePortOverviewVMSchema>;
export type ServiceOverviewVM = z.infer<typeof ServiceOverviewVMSchema>;
export type DeploymentOverviewVM = z.infer<typeof DeploymentOverviewVMSchema>;
export type LogCollectionOverviewVM = z.infer<typeof LogCollectionOverviewVMSchema>;
export type HealthProbeOverviewVM = z.infer<typeof HealthProbeOverviewVMSchema>;
export type IngressPathOverviewVM = z.infer<typeof IngressPathOverviewVMSchema>;
export type IngressOverviewVM = z.infer<typeof IngressOverviewVMSchema>;
export type ProjectRuntimeOverviewVM = z.infer<typeof ProjectRuntimeOverviewVMSchema>;

export interface LogCollectionInput {
  path: string;
  pattern: string;
}

export interface HealthProbeInput {
  port: number;
  probe: Array<"Readiness" | "Liveness">;
  path?: string;
  initial_delay_seconds: number;
  period_seconds: number;
  timeout_seconds: number;
  success_threshold: number;
  failure_threshold: number;
}

export interface RuntimeServicePortInput {
  port: number;
  target_port?: number;
  name: string;
  protocol: string;
}

export interface RuntimeIngressInput {
  host?: string;
  path?: string;
}

export interface ScaleDeploymentInput {
  replicas: number;
}

export interface DeploymentResourcesInput {
  cpu_request: string;
  cpu_limit: string;
  memory_request: string;
  memory_limit: string;
}

export const ProjectConfigMapVMSchema = z.object({
  name: z.string(),
  namespace: z.string(),
  labels: z.record(z.string()).optional(),
  data: z.record(z.string()).optional(),
  created_at: z.string().optional(),
});

export type ProjectConfigMapVM = z.infer<typeof ProjectConfigMapVMSchema>;

export interface CreateProjectConfigMapInput {
  name?: string;
  data?: Record<string, string>;
}

export interface UpsertConfigMapEntryInput {
  key: string;
  value: string;
}

export interface ReplaceConfigMapDataInput {
  data: Record<string, string>;
}

export const ProjectPVCVMSchema = z.object({
  name: z.string(),
  namespace: z.string(),
  status: z.string().optional(),
  storage: z.string().optional(),
  access_modes: z.array(z.string()).optional(),
  storage_class: z.string().optional(),
  volume_name: z.string().optional(),
  mount_name: z.string().optional(),
  mount_path: z.string().optional(),
  mounted: z.boolean().optional(),
  created_at: z.string().optional(),
});

export type ProjectPVCVM = z.infer<typeof ProjectPVCVMSchema>;

export const StorageClassVMSchema = z.object({
  name: z.string(),
  provisioner: z.string().optional(),
  reclaim_policy: z.string().optional(),
});

export type StorageClassVM = z.infer<typeof StorageClassVMSchema>;

export interface CreateProjectPVCInput {
  name?: string;
  storage?: string;
  access_mode?: string;
  storage_class?: string;
  mount_path?: string;
  volume_name?: string;
}

export interface BindProjectPVCInput {
  mount_path: string;
  volume_name?: string;
}
