export type PipelineStatus = "draft" | "active" | "archived";

export interface PipelineParameter {
  name: string;
  type: "string" | "number" | "boolean";
  required?: boolean;
  default?: string;
  description?: string;
}

export interface PipelineStep {
  uses: string;
  with?: Record<string, unknown>;
  timeout?: string;
  retry?: { max: number; backoff?: string };
}

export interface PipelineStage {
  id: string;
  needs?: string[];
  if?: string;
  steps: PipelineStep[];
}

export interface PipelineDefinition {
  apiVersion: string;
  kind: "Pipeline";
  metadata: {
    name: string;
    description?: string;
  };
  spec: {
    parameters: PipelineParameter[];
    stages: PipelineStage[];
  };
}

export interface StageLayout {
  x: number;
  y: number;
}

export interface PipelineLayout {
  stages: Record<string, StageLayout>;
}

export interface PipelineVM {
  id: string;
  name: string;
  description: string;
  status: PipelineStatus;
  version: number;
  definition: PipelineDefinition;
  layout: PipelineLayout;
  updatedAt: string;
}

export interface ClusterVM {
  id: string;
  name: string;
  region: string;
  env: string;
  ready: boolean;
  reason?: string;
}

export interface FleetWave {
  name: string;
  clusters: string[];
}

export interface FleetTopology {
  waves: FleetWave[];
}

export interface FleetVM {
  id: string;
  name: string;
  description: string;
  clusters: ClusterVM[];
  topology: FleetTopology;
  updatedAt: string;
}

export interface ProfileVM {
  id: string;
  name: string;
  description: string;
  env: string;
  values: Record<string, string | number | boolean>;
  patch: Record<string, unknown>;
  updatedAt: string;
}

export interface ReleasePreflightVM {
  pipelines: Array<{ id: string; name: string; version: number; description: string }>;
  fleets: Array<{
    id: string;
    name: string;
    clusters: Array<ClusterVM & { wave?: string }>;
  }>;
  policies: Array<{ name: string; passed: boolean; message?: string }>;
  blocked: boolean;
}

export type ArtifactSource = "artifact" | "build";

export interface ArtifactOption {
  image_tag: string;
  image_repository: string;
  git_ref_type: "branch" | "tag";
  git_ref_value: string;
  build_id?: string;
  run_id: string;
}

export interface ReleaseDraft {
  source: ArtifactSource;
  pipelineId: string;
  parameters: Record<string, string>;
  fleetIds: string[];
  gitRefType: "branch" | "tag";
  gitRefValue: string;
  imageTag: string;
  imageRepository: string;
}

export interface StepCatalogEntry {
  uses: string;
  label: string;
  category: string;
  description: string;
  pauses?: boolean;
  fields: StepFieldDef[];
}

export interface StepFieldDef {
  key: string;
  label: string;
  type: "string" | "number" | "boolean" | "select" | "string-list";
  options?: Array<{ value: string; label: string }>;
  placeholder?: string;
  required?: boolean;
}
