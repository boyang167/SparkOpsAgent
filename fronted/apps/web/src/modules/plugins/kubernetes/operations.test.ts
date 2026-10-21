import { describe, expect, it } from "vitest";
import {
  cloudTTYProxyPath,
  isNodeTTYReady,
  podOperationPath,
  toCommandQuery,
  waitForNodeTTYReady,
} from "./operations";

describe("kubernetes operation helpers", () => {
  it("encodes pod operation targets and repeated command arguments", () => {
    expect(podOperationPath("cluster", "dev", "api-0", "logs")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/cluster/namespaces/dev/pods/api-0/logs",
    );
    expect(toCommandQuery(["sh", "-c", "echo hello"])).toBe(
      "command=sh&command=-c&command=echo+hello",
    );
  });

  it("builds a sandboxed CloudTTY service proxy path from the CloudShell ref", () => {
    expect(cloudTTYProxyPath("prod", "default", "sparkops-cloudtty-abcd")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/prod/proxy/api/v1/namespaces/default/services/http:sparkops-cloudtty-abcd:7681/proxy/",
    );
  });

  it("requires Running phase and ready node-shell container", () => {
    expect(isNodeTTYReady({ status: { phase: "Pending", containerStatuses: [] } })).toBe(false);
    expect(isNodeTTYReady({
      status: { phase: "Running", containerStatuses: [{ name: "node-shell", ready: true }] },
    })).toBe(true);
  });

  it("polls pending NodeTTY pods and resolves only when ready", async () => {
    const states = [
      { status: { phase: "Pending", containerStatuses: [] } },
      { status: { phase: "Running", containerStatuses: [{ name: "node-shell", ready: true }] } },
    ];
    let calls = 0;
    await waitForNodeTTYReady({
      load: async () => states[calls++]!,
      attempts: 2,
      intervalMs: 0,
      sleep: async () => undefined,
      signal: new AbortController().signal,
    });
    expect(calls).toBe(2);
  });

  it("stops polling when canceled", async () => {
    const controller = new AbortController();
    controller.abort();
    await expect(waitForNodeTTYReady({
      load: async () => ({ status: { phase: "Pending" } }),
      attempts: 2,
      intervalMs: 0,
      sleep: async () => undefined,
      signal: controller.signal,
    })).rejects.toThrow("NodeTTY 等待已取消");
  });
});
