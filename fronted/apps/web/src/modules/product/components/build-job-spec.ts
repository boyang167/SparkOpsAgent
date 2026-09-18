export interface BuildVariable {
  key: string;
  value: string;
}

export interface BuildServiceRow {
  project_id: string;
  build_name: string;
  branch: string;
  git_url: string;
  secret_id: string;
  git_ref_type: "branch" | "tag";
  dockerfile_path: string;
  variables: BuildVariable[];
  script: string;
}

export interface BuildJobStrategy {
  timeout_minutes: number;
  cache: boolean;
}

export interface BuildJobNotify {
  on_success: boolean;
  on_failure: boolean;
  message: string;
}

export interface BuildJobSpec {
  registry_id: string;
  project_builds: BuildServiceRow[];
  strategy: BuildJobStrategy;
  notify: BuildJobNotify;
}

export function emptyBuildJobSpec(): BuildJobSpec {
  return {
    registry_id: "",
    project_builds: [],
    strategy: { timeout_minutes: 60, cache: false },
    notify: { on_success: false, on_failure: true, message: "" },
  };
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function parseVariables(value: unknown): BuildVariable[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    const row = asRecord(item);
    if (!row) return [];
    return [{ key: asString(row.key), value: asString(row.value) }];
  });
}

function parseService(item: unknown, fallbackBranch: string): BuildServiceRow | null {
  const row = asRecord(item);
  if (!row) return null;
  const projectId = asString(row.project_id).trim();
  if (!projectId) return null;
  return {
    project_id: projectId,
    build_name: asString(row.build_name),
    branch: asString(row.branch, fallbackBranch) || fallbackBranch,
    git_url: asString(row.git_url),
    secret_id: asString(row.secret_id),
    git_ref_type: asString(row.git_ref_type) === "tag" ? "tag" : "branch",
    dockerfile_path: asString(row.dockerfile_path),
    variables: parseVariables(row.variables),
    script: asString(row.script),
  };
}

export function parseBuildJobSpec(raw: Record<string, unknown> | undefined): BuildJobSpec {
  const base = emptyBuildJobSpec();
  if (!raw) return base;
  const fallbackBranch = asString(raw.branch, "main") || "main";
  const services = Array.isArray(raw.project_builds)
    ? raw.project_builds.flatMap((item) => {
        const row = parseService(item, fallbackBranch);
        return row ? [row] : [];
      })
    : [];
  const strategy = asRecord(raw.strategy);
  const notify = asRecord(raw.notify);
  const timeout = Number(strategy?.timeout_minutes);
  return {
    registry_id: asString(raw.registry_id),
    project_builds: services,
    strategy: {
      timeout_minutes: Number.isFinite(timeout) && timeout > 0 ? timeout : base.strategy.timeout_minutes,
      cache: strategy?.cache === true,
    },
    notify: {
      on_success: notify?.on_success === true,
      on_failure: notify ? notify.on_failure !== false : base.notify.on_failure,
      message: asString(notify?.message),
    },
  };
}

export function buildJobSpecToRecord(spec: BuildJobSpec): Record<string, unknown> {
  const branch = spec.project_builds[0]?.branch || "main";
  return {
    registry_id: spec.registry_id,
    branch,
    project_builds: spec.project_builds,
    strategy: spec.strategy,
    notify: spec.notify,
  };
}
