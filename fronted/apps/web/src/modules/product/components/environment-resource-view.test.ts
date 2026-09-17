import { describe, expect, it } from "vitest";
import { resourceImages, resourceReady } from "./environment-resource-view";

describe("environment resource view helpers", () => {
  it("reads deployment ready and images", () => {
    const item = {
      kind: "Deployment",
      spec: {
        replicas: 2,
        template: { spec: { containers: [{ image: "backend:v1" }, { image: "sidecar:v1" }] } },
      },
      status: { readyReplicas: 2 },
    };
    expect(resourceReady(item)).toBe("2/2");
    expect(resourceImages(item)).toBe("backend:v1, sidecar:v1");
  });

  it("reads cronjob images from job template", () => {
    const item = {
      kind: "CronJob",
      spec: {
        suspend: false,
        jobTemplate: { spec: { template: { spec: { containers: [{ image: "job:1" }] } } } },
      },
    };
    expect(resourceReady(item)).toBe("调度中");
    expect(resourceImages(item)).toBe("job:1");
  });
});
