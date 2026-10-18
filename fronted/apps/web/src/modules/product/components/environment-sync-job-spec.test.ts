import { describe, expect, it } from "vitest";
import {
  environmentSyncJobSpecToRecord,
  parseEnvironmentSyncJobSpec,
} from "./environment-sync-job-spec";

describe("environment sync job spec", () => {
  it("normalizes environment sync defaults", () => {
    expect(parseEnvironmentSyncJobSpec({
      source_environment_id: "source",
      target_environment_id: "target",
      projects: [{ project_id: "p1" }],
    })).toEqual({
      sourceEnvironmentId: "source",
      targetEnvironmentId: "target",
      projects: [{ projectId: "p1", syncImage: true, syncYaml: false, syncResourceKinds: [], resourceOverrides: {} }],
    });
  });

  it("serializes the stable task spec shape", () => {
    expect(environmentSyncJobSpecToRecord({
      sourceEnvironmentId: "source",
      targetEnvironmentId: "target",
      projects: [{
        projectId: "p1",
        syncImage: false,
        syncYaml: true,
        syncResourceKinds: ["Deployment", "Service", "ConfigMap", "Ingress"],
        resourceOverrides: {},
      }],
    })).toEqual({
      source_environment_id: "source",
      target_environment_id: "target",
      projects: [{
        project_id: "p1",
        sync_image: false,
        sync_yaml: true,
        sync_resource_kinds: ["Deployment", "Service", "ConfigMap", "Ingress"],
      }],
    });
  });

  it("round-trips resource overrides", () => {
    expect(parseEnvironmentSyncJobSpec({
      source_environment_id: "source",
      target_environment_id: "target",
      projects: [{
        project_id: "p1",
        sync_yaml: true,
        sync_resource_kinds: ["Deployment"],
        resource_overrides: { "Deployment/app": "apiVersion: apps/v1\nkind: Deployment" },
      }],
    }).projects[0]?.resourceOverrides).toEqual({
      "Deployment/app": "apiVersion: apps/v1\nkind: Deployment",
    });
    expect(environmentSyncJobSpecToRecord({
      sourceEnvironmentId: "source",
      targetEnvironmentId: "target",
      projects: [{
        projectId: "p1",
        syncImage: true,
        syncYaml: true,
        syncResourceKinds: ["Deployment"],
        resourceOverrides: { "Deployment/app": "yaml" },
      }],
    })).toEqual({
      source_environment_id: "source",
      target_environment_id: "target",
      projects: [{
        project_id: "p1",
        sync_image: true,
        sync_yaml: true,
        sync_resource_kinds: ["Deployment"],
        resource_overrides: { "Deployment/app": "yaml" },
      }],
    });
  });
});
