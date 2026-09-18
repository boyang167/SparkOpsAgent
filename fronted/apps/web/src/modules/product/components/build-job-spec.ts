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
  id: string;
  enabled: boolean;
  events: string[];
  channel: string;
  webhook_url: string;
  targets: string;
}

export interface BuildJobSpec {
  registry_id: string;
  project_builds: BuildServiceRow[];
  strategy: BuildJobStrategy;
  notify: BuildJobNotify[];
}

export function emptyBuildJobSpec(): BuildJobSpec {
  return {
    registry_id: "",
    project_builds: [],
    strategy: { timeout_minutes: 60, cache: false },
    notify: [],
  };
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function parseNotifyItem(value: unknown, index: number): BuildJobNotify | null {
  const row = asRecord(value);
  if (!row) return null;
  const modern = Array.isArray(row.events) || typeof row.channel === "string";
  const events = modern
    ? (Array.isArray(row.events) ? row.events.filter((item): item is string => typeof item === "string") : [])
    : [
        ...(row.on_success === true ? ["passed"] : []),
        ...(row.on_failure !== false ? ["failed"] : []),
      ];
  const channel = asString(row.channel);
  if (events.length === 0 && !channel) return null;
  return {
    id: asString(row.id) || `notify-${index + 1}`,
    enabled: row.enabled !== false,
    events,
    channel,
    webhook_url: asString(row.webhook_url),
    targets: asString(row.targets),
  };
}

function parseNotifyList(value: unknown): BuildJobNotify[] {
  if (Array.isArray(value)) {
    return value.flatMap((item, index) => {
      const row = parseNotifyItem(item, index);
      return row ? [row] : [];
    });
  }
  const single = parseNotifyItem(value, 0);
  return single ? [single] : [];
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
  const timeout = Number(strategy?.timeout_minutes);
  return {
    registry_id: asString(raw.registry_id),
    project_builds: services,
    strategy: {
      timeout_minutes: Number.isFinite(timeout) && timeout > 0 ? timeout : base.strategy.timeout_minutes,
      cache: strategy?.cache === true,
    },
    notify: parseNotifyList(raw.notify),
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
