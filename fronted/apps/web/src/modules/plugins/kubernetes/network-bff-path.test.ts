import { describe, expect, it } from "vitest";
import { buildKubernetesPluginApiPath } from "@sparkops/bff-core";
import { CATALOG, proxyPath } from "./catalog";

describe("network proxy paths through BFF", () => {
  const clusterId = "550e8400-e29b-41d4-a716-446655440000";

  it("accepts list paths for network and namespace resources", () => {
    for (const kind of ["Service", "Ingress", "Endpoints", "NetworkPolicy", "Namespace"] as const) {
      const def = CATALOG.find((item) => item.kind === kind);
      expect(def).toBeTruthy();
      const url = proxyPath(clusterId, def!, "default");
      const segments = url.replace("/api/bff/v1/plugins/kubernetes/", "").split("/");
      expect(() => buildKubernetesPluginApiPath(segments, "?page=1&pageSize=10")).not.toThrow();
    }
  });

  it("accepts cluster-scoped service list without namespace", () => {
    const def = CATALOG.find((item) => item.kind === "Service");
    const url = proxyPath(clusterId, def!);
    const segments = url.replace("/api/bff/v1/plugins/kubernetes/", "").split("/");
    expect(buildKubernetesPluginApiPath(segments, "?page=1&pageSize=10")).toContain("/services?");
  });

  it("accepts service resource names after services segment", () => {
    const segments = [
      "clusters",
      clusterId,
      "proxy",
      "api",
      "v1",
      "namespaces",
      "default",
      "services",
      "consul",
    ];
    expect(buildKubernetesPluginApiPath(segments)).toBe(
      `/api/v1/plugins/kubernetes/clusters/${clusterId}/proxy/api/v1/namespaces/default/services/consul`,
    );
  });
});
