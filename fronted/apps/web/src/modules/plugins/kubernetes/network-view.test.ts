import { describe, expect, it } from "vitest";
import {
  emptyNetworkTemplate,
  formatEndpointSubsets,
  formatExternalIPs,
  formatIngressRules,
  formatLoadBalancerIngress,
  formatSelector,
  formatServiceExternalEndpoints,
  formatServiceInternalEndpoints,
  networkTabLabel,
} from "./network-view";

describe("network view", () => {
  it("labels network tabs", () => {
    expect(networkTabLabel("Service")).toBe("Services");
    expect(networkTabLabel("NetworkPolicy")).toBe("Network Policies");
  });

  it("creates service template", () => {
    expect(emptyNetworkTemplate("Service", "default")).toEqual({
      apiVersion: "v1",
      kind: "Service",
      metadata: { namespace: "default" },
      spec: { type: "ClusterIP", ports: [], selector: {} },
    });
  });

  it("formats service endpoints", () => {
    expect(formatServiceInternalEndpoints("consul", [{ port: 8500, protocol: "TCP" }])).toEqual([
      "consul: 8500 TCP",
    ]);
    expect(
      formatServiceExternalEndpoints([{ port: 80 }], [{ ip: "1.2.3.4" }]),
    ).toEqual(["1.2.3.4: 80"]);
  });

  it("formats external ips and selector", () => {
    expect(formatExternalIPs([])).toBe("-");
    expect(formatExternalIPs(["1.1.1.1", "2.2.2.2"])).toBe("1.1.1.1, 2.2.2.2");
    expect(formatSelector({ app: "consul" })).toEqual(["app=consul"]);
  });

  it("formats ingress rules and load balancer", () => {
    expect(
      formatIngressRules([
        {
          host: "demo.local",
          http: {
            paths: [{ path: "/api", backend: { service: { name: "api", port: { number: 8080 } } } }],
          },
        },
      ]),
    ).toEqual(["demo.local/api -> api:8080"]);
    expect(formatLoadBalancerIngress([{ ip: "10.0.0.1" }])).toEqual(["10.0.0.1"]);
  });

  it("formats endpoint subsets", () => {
    expect(
      formatEndpointSubsets([{ addresses: [{ ip: "10.0.0.5" }], ports: [{ port: 443 }] }]),
    ).toEqual(["10.0.0.5:443"]);
  });
});
