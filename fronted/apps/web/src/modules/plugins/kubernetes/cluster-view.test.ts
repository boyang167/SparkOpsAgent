import { describe, expect, it } from "vitest";
import { clusterIntegrations, formatClusterServer, truncateClusterId } from "./cluster-view";

describe("cluster-view", () => {
  it("lists configured integrations only", () => {
    expect(
      clusterIntegrations({
        id: "c1",
        name: "dev",
        server: "https://k8s.example",
        prometheus_url: "http://prom",
        llm_base_url: "http://llm",
      }),
    ).toEqual(["prometheus", "llm"]);
  });

  it("truncates long cluster ids", () => {
    expect(truncateClusterId("5721bb3d-15d0-4782-ab62-b773ab04fabc")).toBe("5721bb3d…");
  });

  it("formats missing server address", () => {
    expect(formatClusterServer("")).toBe("未配置接口地址");
  });
});
