import { z } from "zod";

export const DashboardVMSchema = z.object({
  project_count: z.number().optional(),
  build_count: z.number().optional(),
  pod_count: z.number().optional(),
  recent_builds: z.array(z.record(z.unknown())).optional(),
});

export type DashboardVM = z.infer<typeof DashboardVMSchema>;

export const MonitorOverviewVMSchema = z.object({
  grafana_url: z.string().optional(),
  prometheus_url: z.string().optional(),
  panels: z.array(z.record(z.unknown())).optional(),
});

export type MonitorOverviewVM = z.infer<typeof MonitorOverviewVMSchema>;

export const DeploymentStatVMSchema = z.object({
  date: z.string(),
  count: z.number(),
  project_name: z.string().optional(),
});

export type DeploymentStatVM = z.infer<typeof DeploymentStatVMSchema>;

export const ClusterVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  api_server: z.string().optional(),
  status: z.string().optional(),
  created_at: z.string().optional(),
});

export type ClusterVM = z.infer<typeof ClusterVMSchema>;

export const ProclaimVMSchema = z.object({
  id: z.string(),
  title: z.string(),
  content: z.string().optional(),
  published_at: z.string().optional(),
  status: z.string().optional(),
});

export type ProclaimVM = z.infer<typeof ProclaimVMSchema>;

export const ToolVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional(),
  path: z.string().optional(),
});

export type ToolVM = z.infer<typeof ToolVMSchema>;

export const ServerStatusVMSchema = z.object({
  hostname: z.string(),
  go_version: z.string(),
  num_cpu: z.number(),
  goroutines: z.number(),
  uptime_sec: z.number(),
  mem_alloc_mb: z.number(),
  mem_sys_mb: z.number(),
  heap_inuse_mb: z.number(),
  heap_objects: z.number(),
  gc_pause_ms: z.number(),
  collected_at: z.string(),
});

export type ServerStatusVM = z.infer<typeof ServerStatusVMSchema>;

export const SyncJobVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  workspace_id: z.string().optional(),
  version: z.string().optional(),
  source_cluster: z.string().optional(),
  target_cluster: z.string().optional(),
  status: z.string().optional(),
  result: z.string().optional(),
});

export type SyncJobVM = z.infer<typeof SyncJobVMSchema>;

export const SysConfigVMSchema = z.object({
  id: z.string().optional(),
  kind: z.string().optional(),
  name: z.string().optional(),
  whether_share: z.boolean().optional(),
  config: z.record(z.unknown()).optional(),
});

export type SysConfigVM = z.infer<typeof SysConfigVMSchema>;
