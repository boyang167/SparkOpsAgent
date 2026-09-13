import { describe, expect, it } from "vitest";
import { parseCrossClusterSyncProgress } from "./cross-cluster-sync-progress";

describe("parseCrossClusterSyncProgress", () => {
  it("parses started/succeeded/failed messages from SSE JSON lines", () => {
    const output = [
      `{"message":"cross-cluster sync started: staging"}`,
      `{"message":"cross-cluster sync succeeded: staging"}`,
      `{"message":"cross-cluster sync started: prod"}`,
      `{"message":"cross-cluster sync failed: prod (timeout)"}`,
    ].join("\n");

    expect(parseCrossClusterSyncProgress(output)).toEqual([
      { name: "staging", status: "succeeded" },
      { name: "prod", status: "failed", detail: "timeout" },
    ]);
  });
});
