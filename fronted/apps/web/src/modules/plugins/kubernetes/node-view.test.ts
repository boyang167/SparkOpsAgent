import { describe, expect, it } from "vitest";
import {
  cpuToMillicores,
  formatNodeAge,
  isEvictablePod,
  memoryToBytes,
  nodeConditionTone,
  nodeInternalIP,
  nodeResourceView,
  nodeRoles,
  nodeStatus,
} from "./node-view";

describe("node-view", () => {
  it("parses status, roles and internal IP", () => {
    const node = {
      metadata: { labels: { "node-role.kubernetes.io/control-plane": "", "node-role.kubernetes.io/master": "" } },
      spec: { unschedulable: true },
      status: {
        addresses: [{ type: "InternalIP", address: "192.168.5.21" }],
        conditions: [{ type: "Ready", status: "True" }],
      },
    };
    expect(nodeInternalIP(node)).toBe("192.168.5.21");
    expect(nodeStatus(node)).toBe("Ready, SchedulingDisabled");
    expect(nodeRoles(node.metadata.labels)).toEqual(["control-plane", "master"]);
  });

  it("computes usage against allocatable capacity", () => {
    const view = nodeResourceView(
      {
        status: {
          allocatable: { cpu: "8", memory: "16Gi" },
        },
      },
      [
        {
          spec: {
            containers: [{ resources: { requests: { cpu: "500m", memory: "1Gi" }, limits: { cpu: "1", memory: "2Gi" } } }],
          },
        },
      ],
      { usage: { cpu: "250m", memory: "512Mi" } },
    );
    expect(view.cpu.usedLabel).toContain("0.25 / 8.00 核");
    expect(view.memory.usedLabel).toContain("Gi");
    expect(cpuToMillicores("100n")).toBeCloseTo(0.0001);
    expect(memoryToBytes("1Gi")).toBe(1024 ** 3);
  });

  it("skips daemonset pods when draining", () => {
    expect(isEvictablePod({ metadata: { ownerReferences: [{ kind: "DaemonSet" }] } })).toBe(false);
    expect(isEvictablePod({ metadata: { name: "app-1" } })).toBe(true);
  });

  it("formats age in days", () => {
    const now = Date.parse("2026-08-31T00:00:00Z");
    expect(formatNodeAge("2024-01-07T00:00:00Z", now)).toBe("967 天");
  });

  it("maps node condition tone with inverted pressure types", () => {
    expect(nodeConditionTone("Ready", "True")).toBe("success");
    expect(nodeConditionTone("Ready", "False")).toBe("danger");
    expect(nodeConditionTone("PIDPressure", "False")).toBe("success");
    expect(nodeConditionTone("PIDPressure", "True")).toBe("danger");
  });
});
