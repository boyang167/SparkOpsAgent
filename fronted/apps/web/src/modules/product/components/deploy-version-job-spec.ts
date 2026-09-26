import { parseDeployJobSpec } from "./deploy-job-spec";

export type DeployVersionJobSpec = {
  environmentId: string;
  jobName: string;
};

/** @deprecated Use parseDeployJobSpec with artifactSource=version instead. */
export function parseDeployVersionJobSpec(value: unknown): DeployVersionJobSpec {
  const spec = value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
  const parsed = parseDeployJobSpec(spec, "sparkops-deploy-version");
  return {
    environmentId: parsed.environmentId,
    jobName: parsed.jobName,
  };
}
