import type { BuildVM } from "@sparkops/contracts/delivery";

export interface BuildSummaryState {
  count: string;
  latest: BuildVM | null;
  unavailable: boolean;
}

export function buildStatusLabel(status: string) {
  const map: Record<string, string> = {
    running: "构建中",
    success: "成功",
    failed: "失败",
    aborted: "已中止",
  };
  return map[status] ?? status;
}

export function buildStatusTone(status: string) {
  if (status === "success") return "success";
  if (status === "failed") return "failed";
  if (status === "aborted") return "aborted";
  if (status === "running") return "running";
  return "neutral";
}

export function buildSummaryState(
  builds: BuildVM[] | null,
  loading: boolean,
  error: string | null,
): BuildSummaryState {
  if (loading) return { count: "加载中…", latest: null, unavailable: false };
  if (error || builds === null) return { count: "不可用", latest: null, unavailable: true };
  return {
    count: String(builds.length),
    latest: builds[0] ?? null,
    unavailable: false,
  };
}
