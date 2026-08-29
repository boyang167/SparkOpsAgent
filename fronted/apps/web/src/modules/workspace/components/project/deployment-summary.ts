import type {
  DeploymentRunListVM,
  DeploymentRunVM,
} from "@sparkops/contracts/delivery";

export interface DeploymentSummaryState {
  count: string;
  latest: DeploymentRunVM | null;
  unavailable: boolean;
}

export function deploymentSummaryState(
  page: DeploymentRunListVM | null,
  loading: boolean,
  error: string | null,
): DeploymentSummaryState {
  if (loading) return { count: "加载中…", latest: null, unavailable: false };
  if (error || !page) return { count: "不可用", latest: null, unavailable: true };
  return {
    count: String(page.total),
    latest: page.items[0] ?? null,
    unavailable: false,
  };
}
