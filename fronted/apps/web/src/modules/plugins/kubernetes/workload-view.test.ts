import { describe, expect, it, vi, afterEach } from "vitest";
import { CATALOG } from "./catalog";
import { formatDateTime } from "./node-view";
import {
  WORKLOAD_TABS,
  clampPageToTotal,
  cronJobView,
  daemonSetCounts,
  emptyWorkloadTemplate,
  jobStatus,
  podPhaseBadge,
  replicaBadge,
  validateCreateIdentity,
  validateExistingIdentity,
} from "./workload-view";

describe("workload-view", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("fixes WORKLOAD_TABS to six kinds without ReplicaSet", () => {
    expect(WORKLOAD_TABS).toEqual([
      "Deployment",
      "DaemonSet",
      "StatefulSet",
      "Job",
      "CronJob",
      "Pod",
    ]);
    expect(WORKLOAD_TABS).not.toContain("ReplicaSet");
  });

  it("marks replica badge bad when not all ready", () => {
    expect(replicaBadge(0, 1)).toEqual({ text: "0/1", tone: "bad" });
  });

  it("marks replica badge ok when all ready", () => {
    expect(replicaBadge(1, 1)).toEqual({ text: "1/1", tone: "ok" });
  });

  it("marks replica badge ok when zero replicas are all ready", () => {
    expect(replicaBadge(0, 0)).toEqual({ text: "0/0", tone: "ok" });
  });

  it("reads daemonset ready, current and desired counts", () => {
    const obj = {
      status: {
        numberReady: 2,
        currentNumberScheduled: 3,
        desiredNumberScheduled: 3,
      },
    };
    expect(daemonSetCounts(obj)).toEqual({ ready: 2, current: 3, desired: 3 });
  });

  it("reads job active, succeeded, failed counts and completed duration", () => {
    const obj = {
      status: {
        active: 1,
        succeeded: 2,
        failed: 0,
        startTime: "2026-08-31T10:00:00Z",
        completionTime: "2026-08-31T10:05:00Z",
      },
    };
    expect(jobStatus(obj)).toEqual({
      active: 1,
      succeeded: 2,
      failed: 0,
      duration: "5 分钟",
    });
  });

  it("computes running job duration from startTime to now", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-31T10:10:00Z"));
    const obj = {
      status: {
        active: 1,
        succeeded: 0,
        failed: 0,
        startTime: "2026-08-31T10:00:00Z",
      },
    };
    expect(jobStatus(obj)).toMatchObject({
      active: 1,
      succeeded: 0,
      failed: 0,
      duration: "10 分钟",
    });
  });

  it("reads cronjob schedule, last schedule and suspend flag", () => {
    const lastScheduleTime = "2026-08-31T09:00:00Z";
    const obj = {
      spec: { schedule: "0 * * * *", suspend: true },
      status: { lastScheduleTime },
    };
    expect(cronJobView(obj)).toEqual({
      schedule: "0 * * * *",
      lastSchedule: formatDateTime(lastScheduleTime),
      suspend: true,
    });
  });

  it("shows Running pod phase as ok badge", () => {
    expect(podPhaseBadge("Running")).toEqual({ text: "Running", tone: "ok" });
  });

  it("maps Succeeded pod phase to Completed ok badge", () => {
    expect(podPhaseBadge("Succeeded")).toEqual({ text: "Completed", tone: "ok" });
  });

  it("builds empty deployment template with kind, apiVersion, namespace and base spec", () => {
    const tpl = emptyWorkloadTemplate("Deployment", "default");
    expect(tpl).toMatchObject({
      apiVersion: "apps/v1",
      kind: "Deployment",
      metadata: { namespace: "default" },
      spec: {
        replicas: 1,
        selector: {},
        template: {},
      },
    });
  });

  it("keeps the current page when it remains inside total pages", () => {
    expect(clampPageToTotal(2, 35, 10)).toBe(2);
  });

  it("moves past-end pages to the last valid page after totals shrink", () => {
    expect(clampPageToTotal(5, 31, 10)).toBe(4);
    expect(clampPageToTotal(3, 20, 10)).toBe(2);
  });

  it("keeps an empty result on page one after deleting the final item", () => {
    expect(clampPageToTotal(2, 0, 10)).toBe(1);
  });

  it("accepts existing workload saves when immutable identity stays unchanged", () => {
    const original = {
      apiVersion: "apps/v1",
      kind: "Deployment",
      metadata: { name: "api", namespace: "default" },
    };

    expect(validateExistingIdentity(original, { ...original, spec: { replicas: 2 } })).toBeNull();
  });

  it("rejects existing workload saves that change immutable identity fields", () => {
    const original = {
      apiVersion: "apps/v1",
      kind: "Deployment",
      metadata: { name: "api", namespace: "default" },
    };

    expect(validateExistingIdentity(original, { ...original, apiVersion: "batch/v1" })).toContain("apiVersion");
    expect(validateExistingIdentity(original, { ...original, kind: "StatefulSet" })).toContain("kind");
    expect(validateExistingIdentity(original, { ...original, metadata: { name: "worker", namespace: "default" } })).toContain("metadata.name");
    expect(validateExistingIdentity(original, { ...original, metadata: { name: "api", namespace: "prod" } })).toContain("metadata.namespace");
  });

  it("accepts create manifests that match the captured workload definition", () => {
    const deployment = mustResource("Deployment");

    expect(
      validateCreateIdentity(deployment, {
        apiVersion: "apps/v1",
        kind: "Deployment",
        metadata: { name: "api", namespace: "default" },
      }),
    ).toBeNull();
  });

  it("rejects create manifests that try to switch kind or apiVersion", () => {
    const deployment = mustResource("Deployment");

    expect(validateCreateIdentity(deployment, { apiVersion: "apps/v1", kind: "StatefulSet" })).toContain("kind");
    expect(validateCreateIdentity(deployment, { apiVersion: "batch/v1", kind: "Deployment" })).toContain("apiVersion");
  });
});

function mustResource(kind: string) {
  const found = CATALOG.find((item) => item.kind === kind);
  if (!found) throw new Error(`missing ${kind}`);
  return found;
}
