import { describe, expect, it } from "vitest";
import { buildKubernetesPluginApiPath } from "./plugin-kubernetes";

describe("buildKubernetesPluginApiPath", () => {
  it("encodes normalized kubernetes plugin segments", () => {
    expect(buildKubernetesPluginApiPath(["clusters", "名字"], "?watch=1")).toBe(
      "/api/v1/plugins/kubernetes/clusters/%E5%90%8D%E5%AD%97?watch=1",
    );
  });

  it.each([
    [[""]],
    [["."]],
    [[".."]],
    [["%2e%2e"]],
    [["%25252e"]],
    [["pods/admin"]],
    [["pods%2Fadmin"]],
    [["pods%252fadmin"]],
    [["pods\\admin"]],
    [["pods\u0000admin"]],
    [["http:"]],
    [["name%25252541"]],
    [["bad%zz"]],
  ])("rejects unsafe route segments %#", (segments) => {
    expect(() => buildKubernetesPluginApiPath(segments)).toThrow("invalid kubernetes plugin path");
  });
});
