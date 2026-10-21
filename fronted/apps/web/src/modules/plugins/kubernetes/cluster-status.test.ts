import { describe, expect, it } from "vitest";
import {
  clusterStatusFromProbe,
  clusterStatusView,
  clusterVersionProxyPath,
} from "./cluster-status";

describe("cluster status", () => {
  it("builds encoded version proxy path", () => {
    expect(clusterVersionProxyPath("cluster/prod")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/cluster%2Fprod/proxy/version",
    );
  });

  it("maps checking, healthy, and unhealthy probe results", () => {
    expect(clusterStatusView("checking")).toEqual({ kind: "checking", label: "检测中" });
    expect(clusterStatusFromProbe(true)).toEqual({ kind: "healthy", label: "正常" });
    expect(clusterStatusFromProbe(false, "connection refused")).toEqual({
      kind: "unhealthy",
      label: "异常",
      detail: "connection refused",
    });
  });
});
