import { describe, expect, it } from "vitest";
import {
  CreateDeploymentCommandSchema,
  DeploymentEventVMSchema,
  DeploymentOptionsVMSchema,
  DeploymentRunListVMSchema,
  DeploymentRunVMSchema,
  SyncPreviewVMSchema,
} from "./index";

describe("deployment contracts", () => {
  it("parses a deployment run", () => {
    const run = DeploymentRunVMSchema.parse({
      id: "550e8400-e29b-41d4-a716-446655440000",
      workspace_id: "w",
      project_id: "p",
      cluster_id: "c",
      git_ref_type: "branch",
      git_ref_value: "main",
      image_tag: "main",
      status: "queued",
      current_stage: "",
      created_by: "m",
      created_at: new Date().toISOString(),
    });

    expect(run.image_tag).toBe("main");
    expect(run.error_message).toBe("");
  });

  it("requires manifests for sync_only", () => {
    expect(() =>
      CreateDeploymentCommandSchema.parse({
        sync_only: true,
      }),
    ).toThrow();
    expect(
      CreateDeploymentCommandSchema.parse({
        sync_only: true,
        manifests: [
          {
            kind: "ConfigMap",
            name: "cfg",
            yaml: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cfg\n",
          },
        ],
      }),
    ).toMatchObject({ sync_only: true });
  });

  it("parses sync preview payload", () => {
    expect(
      SyncPreviewVMSchema.parse({
        build_cluster_id: "c1",
        build_cluster_name: "build",
        namespace: "demo",
        items: [
          {
            id: "Deployment/demo",
            kind: "Deployment",
            name: "demo",
            related_reason: "core workload",
            default_selected: true,
            yaml: "kind: Deployment",
          },
        ],
      }).items,
    ).toHaveLength(1);
  });

  it("rejects unsupported deployment values", () => {
    expect(() =>
      CreateDeploymentCommandSchema.parse({
        git_ref_type: "commit",
        git_ref_value: "",
        image_tag: "x".repeat(256),
      }),
    ).toThrow();
  });

  it("parses authorized deployment options", () => {
    expect(
      DeploymentOptionsVMSchema.parse({
        clusters: [{ id: "c", cluster_name: "dev", server: "https://k8s" }],
        missing: [],
      }).clusters[0]?.cluster_name,
    ).toBe("dev");
  });

  it("parses deployment SSE events", () => {
    expect(DeploymentEventVMSchema.parse({ delta: "building\n" })).toEqual({
      delta: "building\n",
    });
    expect(DeploymentEventVMSchema.parse({ done: "true" })).toEqual({ done: "true" });
  });

  it("requires deployment list totals and pagination", () => {
    expect(
      DeploymentRunListVMSchema.parse({
        items: [],
        total: 42,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({ total: 42, page: 1, page_size: 20 });
  });
});
