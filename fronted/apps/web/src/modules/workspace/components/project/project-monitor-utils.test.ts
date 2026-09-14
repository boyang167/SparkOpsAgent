import { describe, expect, it } from "vitest";
import { parsePrometheusMatrix, resolveProjectMonitorClusterId } from "./project-monitor-utils";

describe("project-monitor-utils", () => {
  it("parses prometheus matrix samples", () => {
    expect(
      parsePrometheusMatrix({
        data: {
          result: [
            {
              values: [
                [100, "0.10"],
                [160, "0.25"],
              ],
            },
          ],
        },
      }),
    ).toEqual([
      { timestamp: 100, value: 0.1 },
      { timestamp: 160, value: 0.25 },
    ]);
  });

  it("resolves cluster id from latest successful deployment", () => {
    expect(
      resolveProjectMonitorClusterId(
        { default_cluster_id: "fallback" } as never,
        {
          items: [
            { cluster_id: "cluster-a", status: "succeeded" },
            { cluster_id: "cluster-b", status: "failed" },
          ],
        } as never,
      ),
    ).toBe("cluster-a");
  });
});
