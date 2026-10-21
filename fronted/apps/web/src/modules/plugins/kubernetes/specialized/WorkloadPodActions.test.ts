import { describe, expect, it } from "vitest";
import {
  METRIC_TEMPLATES,
  containerReadinessDots,
  formatPodMetricValue,
  podMetricCell,
  settledPodMetrics,
  withLimitedConcurrency,
} from "./WorkloadPodActions.helpers";

describe("WorkloadPodActions helpers", () => {
  it("summarizes container readiness with ready, waiting and terminated tones", () => {
    const dots = containerReadinessDots({
      spec: {
        containers: [{ name: "api" }, { name: "sidecar" }, { name: "done" }],
      },
      status: {
        containerStatuses: [
          { name: "api", ready: true, state: { running: { startedAt: "2026-09-01T00:00:00Z" } } },
          { name: "sidecar", ready: false, state: { waiting: { reason: "CrashLoopBackOff" } } },
          { name: "done", ready: false, state: { terminated: { reason: "Completed" } } },
        ],
      },
    });

    expect(dots).toEqual([
      { name: "api", tone: "ready", title: "api ready" },
      { name: "sidecar", tone: "bad", title: "sidecar waiting: CrashLoopBackOff" },
      { name: "done", tone: "muted", title: "done terminated: Completed" },
    ]);
  });

  it("formats pod metric responses as compact best-effort cell values", () => {
    expect(formatPodMetricValue({ value: [123, "0.25"] })).toBe("0.25");
    expect(formatPodMetricValue({ data: { result: [{ value: [123, "42"] }] } })).toBe("42");
    expect(formatPodMetricValue({ data: { result: [] } })).toBe("-");
  });

  it("formats the latest Prometheus query_range matrix sample", () => {
    expect(
      formatPodMetricValue({
        data: {
          result: [
            {
              values: [
                [123, "0.10"],
                [456, "0.25"],
              ],
            },
          ],
        },
      }),
    ).toBe("0.25");
  });

  it("only requests metric templates supported by the backend", () => {
    expect(Object.values(METRIC_TEMPLATES)).toEqual(["pod_cpu", "pod_memory"]);
  });

  it("keeps fulfilled pod metrics when another template fails", () => {
    const settled = [
      { status: "fulfilled", value: ["cpu", { value: [123, "0.1"] }] },
      { status: "rejected", reason: new Error("missing prometheus") },
    ] satisfies PromiseSettledResult<readonly ["cpu" | "memory", unknown]>[];

    expect(podMetricCell(settledPodMetrics(settled))).toBe("CPU 0.1 / MEM - / TCP -");
  });

  it("combines cpu, memory and tcp metric responses", () => {
    expect(
      podMetricCell({
        cpu: { data: { result: [{ value: [123, "0.1"] }] } },
        memory: { value: [123, "128Mi"] },
      }),
    ).toBe("CPU 0.1 / MEM 128Mi / TCP -");
  });

  it("limits concurrent work and preserves input order", async () => {
    let active = 0;
    let maxActive = 0;
    const releaseNext: Array<() => void> = [];

    const pending = withLimitedConcurrency([3, 1, 2, 0], 2, async (value) => {
      active += 1;
      maxActive = Math.max(maxActive, active);
      await new Promise<void>((resolve) => releaseNext.push(resolve));
      active -= 1;
      return `pod-${value}`;
    });

    expect(maxActive).toBe(2);
    releaseNext.shift()?.();
    await Promise.resolve();
    releaseNext.shift()?.();
    await Promise.resolve();
    releaseNext.shift()?.();
    await Promise.resolve();
    releaseNext.shift()?.();

    await expect(pending).resolves.toEqual(["pod-3", "pod-1", "pod-2", "pod-0"]);
    expect(maxActive).toBe(2);
  });
});
