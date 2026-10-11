function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export interface CreateVersionJobSpec {
  versionTag: string;
  jobName: string;
  projectIds: string[];
}

export function parseCreateVersionJobSpec(value: unknown): CreateVersionJobSpec {
  const spec = asRecord(value);
  const projectIds = Array.isArray(spec?.project_ids)
    ? spec.project_ids.flatMap((item) => (typeof item === "string" && item.trim() ? [item.trim()] : []))
    : [];
  return {
    versionTag: asString(spec?.version_tag),
    jobName: asString(spec?.job_name),
    projectIds,
  };
}

const PLACEHOLDER_PATTERN = /\{\{[^}]*\}\}/;

export function hasVersionPlaceholder(tag: string): boolean {
  return PLACEHOLDER_PATTERN.test(tag);
}

export function runtimeVersionTag(tag: string): string {
  const trimmed = tag.trim();
  return hasVersionPlaceholder(trimmed) ? "" : trimmed;
}

export function createVersionJobSpecToRecord(spec: CreateVersionJobSpec): Record<string, unknown> {
  return {
    version_tag: spec.versionTag,
    source: "fromjob",
    job_name: spec.jobName,
    project_ids: spec.projectIds,
  };
}
