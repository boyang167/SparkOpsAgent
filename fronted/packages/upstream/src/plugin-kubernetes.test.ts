import { describe, expect, it } from "vitest";
import { buildKubernetesPluginApiPath } from "./plugin-kubernetes";

describe("buildKubernetesPluginApiPath", () => {
  it("encodes normalized kubernetes plugin segments", () => {
    expect(buildKubernetesPluginApiPath(["clusters", "名字"], "?watch=1")).toBe(
      "/api/v1/plugins/kubernetes/clusters/%E5%90%8D%E5%AD%97?watch=1",
    );
  });

  it("preserves validated service proxy segments", () => {
    expect(buildKubernetesPluginApiPath(["services", "https:cloudtty-1:443", "proxy"])).toBe(
      "/api/v1/plugins/kubernetes/services/https:cloudtty-1:443/proxy",
    );
    expect(
      buildKubernetesPluginApiPath([
        "clusters", "prod", "proxy", "api", "v1", "namespaces", "default", "services", "consul",
      ]),
    ).toBe(
      "/api/v1/plugins/kubernetes/clusters/prod/proxy/api/v1/namespaces/default/services/consul",
    );
    expect(() => buildKubernetesPluginApiPath(["services", "http:..:80"])).toThrow();
    expect(() => buildKubernetesPluginApiPath(["pods", "http:name:80"])).toThrow();
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
