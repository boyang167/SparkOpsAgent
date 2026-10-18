import { describe, expect, it } from "vitest";
import {
  isEnvironmentSyncJobValid,
  normalizeEnvironmentSyncProjects,
  validateEnvironmentSyncJob,
} from "./environment-sync-job-validation";

describe("validateEnvironmentSyncJob", () => {
  it("requires distinct environments and one project", () => {
    expect(
      validateEnvironmentSyncJob({
        sourceEnvironmentId: "env",
        targetEnvironmentId: "env",
        projects: [],
      }),
    ).toEqual({
      sourceEnvironmentId: "源环境和目标环境不能相同",
      projects: "请至少选择一个项目",
    });
  });

  it("passes with valid spec", () => {
    const errors = validateEnvironmentSyncJob({
      sourceEnvironmentId: "source",
      targetEnvironmentId: "target",
      projects: [{ projectId: "p1", syncImage: true, syncYaml: false, syncResourceKinds: [],
        resourceOverrides: {} }],
    });
    expect(errors).toEqual({});
    expect(isEnvironmentSyncJobValid(errors)).toBe(true);
  });

  it("requires source and target environments", () => {
    expect(
      validateEnvironmentSyncJob({
        sourceEnvironmentId: "",
        targetEnvironmentId: "",
        projects: [{ projectId: "p1", syncImage: true, syncYaml: false, syncResourceKinds: [],
        resourceOverrides: {} }],
      }),
    ).toEqual({
      sourceEnvironmentId: "请选择源环境",
      targetEnvironmentId: "请选择目标环境",
    });
  });
});

describe("normalizeEnvironmentSyncProjects", () => {
  it("forces first-sync projects to image and yaml", () => {
    expect(
      normalizeEnvironmentSyncProjects(
        [{ projectId: "p1", syncImage: true, syncYaml: false, syncResourceKinds: [],
        resourceOverrides: {} }],
        new Set<string>(),
      )[0],
    ).toMatchObject({ syncImage: true, syncYaml: true });
  });

  it("preserves options for projects already in target", () => {
    expect(
      normalizeEnvironmentSyncProjects(
        [{ projectId: "p1", syncImage: true, syncYaml: false, syncResourceKinds: [],
        resourceOverrides: {} }],
        new Set(["p1"]),
      )[0],
    ).toMatchObject({ syncImage: true, syncYaml: false });
  });
});
