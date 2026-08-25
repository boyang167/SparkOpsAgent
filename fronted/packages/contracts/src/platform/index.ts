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
  go_os: z.string().optional(),
  go_arch: z.string().optional(),
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

export const OperationRecordVMSchema = z.object({
  id: z.string(),
  ip: z.string(),
  method: z.string(),
  path: z.string(),
  status: z.number(),
  latency_ms: z.number(),
  agent: z.string(),
  error_message: z.string(),
  body: z.string(),
  resp: z.string(),
  member_id: z.string().optional(),
  workspace_id: z.string().optional(),
  trace_id: z.string().optional(),
  created_at: z.string(),
});

export type OperationRecordVM = z.infer<typeof OperationRecordVMSchema>;

export const OperationRecordListVMSchema = z.object({
  items: z.array(OperationRecordVMSchema),
  total: z.number(),
});

export type OperationRecordListVM = z.infer<typeof OperationRecordListVMSchema>;

export const ListOperationRecordsQuerySchema = z.object({
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(100).optional(),
  ip: z.string().optional(),
  method: z.string().optional(),
  path: z.string().optional(),
  status: z.number().int().optional(),
  member_id: z.string().optional(),
  trace_id: z.string().optional(),
});

export type ListOperationRecordsQuery = z.infer<typeof ListOperationRecordsQuerySchema>;

export const BatchDeleteOperationRecordsCommandSchema = z.object({
  ids: z.array(z.string()).min(1),
});

export type BatchDeleteOperationRecordsCommand = z.infer<
  typeof BatchDeleteOperationRecordsCommandSchema
>;

export const AppLogVMSchema = z.object({
  id: z.string(),
  level: z.string(),
  category: z.string(),
  message: z.string(),
  trace_id: z.string().optional(),
  workspace_id: z.string().optional(),
  caller: z.string().optional(),
  fields: z.record(z.unknown()).optional(),
  created_at: z.string(),
});

export type AppLogVM = z.infer<typeof AppLogVMSchema>;

export const AppLogListVMSchema = z.object({
  items: z.array(AppLogVMSchema),
  total: z.number(),
});

export type AppLogListVM = z.infer<typeof AppLogListVMSchema>;

export const ListAppLogsQuerySchema = z.object({
  page: z.number().int().min(1).optional(),
  page_size: z.number().int().min(1).max(200).optional(),
  level: z.string().optional(),
  category: z.string().optional(),
  trace_id: z.string().optional(),
  keyword: z.string().optional(),
});

export type ListAppLogsQuery = z.infer<typeof ListAppLogsQuerySchema>;

export const LogFileVMSchema = z.object({
  name: z.string(),
  size: z.number(),
  modified: z.string(),
});

export type LogFileVM = z.infer<typeof LogFileVMSchema>;

export const LogFileListVMSchema = z.object({
  items: z.array(LogFileVMSchema),
  directory: z.string(),
});

export type LogFileListVM = z.infer<typeof LogFileListVMSchema>;

export const LogTailVMSchema = z.object({
  lines: z.array(z.string()),
  offset: z.number(),
  next_offset: z.number(),
  eof: z.boolean(),
});

export type LogTailVM = z.infer<typeof LogTailVMSchema>;

export const MetricSeriesVMSchema = z.object({
  label: z.string(),
  value: z.number(),
  labels: z.record(z.string()).optional(),
});

export type MetricSeriesVM = z.infer<typeof MetricSeriesVMSchema>;

export const MetricsSummaryVMSchema = z.object({
  http_requests_total: z.array(MetricSeriesVMSchema),
  http_duration_p50_ms: z.number(),
  http_duration_p95_ms: z.number(),
  http_duration_p99_ms: z.number(),
  collected_at: z.string(),
  prometheus_url: z.string().optional(),
});

export type MetricsSummaryVM = z.infer<typeof MetricsSummaryVMSchema>;

export const TraceDetailVMSchema = z.object({
  trace_id: z.string(),
  logs: z.array(AppLogVMSchema),
  operations: z.array(OperationRecordVMSchema),
});

export type TraceDetailVM = z.infer<typeof TraceDetailVMSchema>;

export const SysConfigVMSchema = z.object({
  id: z.string().optional(),
  kind: z.string().optional(),
  name: z.string().optional(),
  whether_share: z.boolean().optional(),
  config: z.record(z.unknown()).optional(),
});

export type SysConfigVM = z.infer<typeof SysConfigVMSchema>;

export const WorkspaceEntitlementsVMSchema = z.object({
  agent: z.boolean(),
  knowledge: z.boolean(),
  delivery: z.boolean(),
  k8s: z.boolean(),
  observability: z.boolean(),
  mcp: z.boolean(),
});

export type WorkspaceEntitlementsVM = z.infer<typeof WorkspaceEntitlementsVMSchema>;

export const FEATURE_MODULE_LABELS: Record<keyof WorkspaceEntitlementsVM, string> = {
  agent: "智能体 / 项目",
  knowledge: "知识库",
  delivery: "持续交付",
  k8s: "Kubernetes 运行",
  observability: "可观测性",
  mcp: "MCP 集成",
};
