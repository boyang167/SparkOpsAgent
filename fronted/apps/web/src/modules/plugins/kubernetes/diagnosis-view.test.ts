import { describe, expect, it } from "vitest";
import {
  dashboardFromProxy,
  extractNameValuePairs,
  flattenArthasResults,
  isTargetReady,
  threadStateSummary,
} from "./diagnosis-view";

describe("diagnosis-view", () => {
  it("checks target readiness", () => {
    expect(isTargetReady({ clusterId: "c1", namespace: "dev", pod: "web", container: "app", processId: "" })).toBe(true);
    expect(isTargetReady({ clusterId: "", namespace: "dev", pod: "web", container: "app", processId: "" })).toBe(false);
  });

  it("summarizes thread states", () => {
    const summary = threadStateSummary([
      { state: "RUNNABLE" },
      { state: "WAITING" },
      { state: "RUNNABLE" },
    ]);
    expect(summary.SUM).toBe(3);
    expect(summary.RUNNABLE).toBe(2);
    expect(summary.WAITING).toBe(1);
  });

  it("parses dashboard payload", () => {
    const parsed = dashboardFromProxy({
      body: {
        results: [
          {
            threads: [{ name: "main", state: "RUNNABLE" }],
            memoryInfo: { heap: [{ name: "PS Eden Space", used: 1, total: 2 }] },
            gcInfos: [{ name: "PS Scavenge", collectionCount: 3, collectionTime: 12 }],
            runtimeInfo: { javaVersion: "17" },
          },
        ],
      },
    });
    expect(parsed.threads).toHaveLength(1);
    expect(parsed.memory).toHaveLength(1);
    expect(parsed.gcInfos).toHaveLength(1);
    expect(parsed.runtimeInfo.javaVersion).toBe("17");
  });

  it("flattens arthas exec rows", () => {
    const rows = flattenArthasResults({
      body: {
        results: [{ message: "ok", jvmInfo: { JAVA_VERSION: "17" } }],
      },
    });
    expect(rows.some((row) => row.name.includes("JAVA_VERSION") || row.name === "message")).toBe(true);
  });

  it("extracts nested name value pairs", () => {
    const rows = extractNameValuePairs({ jvmInfo: { HEAP: { used: 1 } } });
    expect(rows.some((row) => row.name.includes("HEAP"))).toBe(true);
  });
});
