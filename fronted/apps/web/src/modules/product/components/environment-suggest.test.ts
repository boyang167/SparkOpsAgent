import { describe, expect, it } from "vitest";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import {
  resolveWorkspaceNamespace,
  suggestEnvironmentNamespace,
  suggestEnvironmentsFromProjects,
} from "./environment-suggest";

function cluster(partial: Partial<AuthorizedClusterVM> & { id: string; cluster_name: string }): AuthorizedClusterVM {
  return {
    priority_id: 0,
    is_build_env: false,
    available: true,
    ...partial,
  };
}

function project(partial: Partial<ProjectVM> & { id: string; name: string }): ProjectVM {
  return {
    status: "active",
    description: "",
    review_status: "none",
    can_submit_review: false,
    can_review: false,
    ...partial,
  };
}

describe("suggestEnvironmentsFromProjects", () => {
  it("groups bound projects by cluster and namespace", () => {
    const clusters = [
      cluster({ id: "c1", cluster_name: "dev-cluster" }),
      cluster({ id: "c2", cluster_name: "prod-cluster" }),
      cluster({ id: "build", cluster_name: "build", is_build_env: true }),
    ];
    const workspaceProjects = [
      project({ id: "p1", name: "order", display_name: "订单", default_cluster_id: "c1", namespace: "pay-uat", product_id: "prod" }),
      project({ id: "p2", name: "pay", display_name: "支付", default_cluster_id: "c1", namespace: "pay-uat", product_id: "prod" }),
      project({ id: "p3", name: "admin", default_cluster_id: "c2", namespace: "pay-prod", product_id: "prod" }),
      project({ id: "p4", name: "orphan", default_cluster_id: "c1", namespace: "other", product_id: undefined }),
    ];

    const candidates = suggestEnvironmentsFromProjects(
      [{ id: "p1" }, { id: "p2" }, { id: "p3" }],
      workspaceProjects,
      clusters,
    );

    expect(candidates).toHaveLength(2);
    expect(candidates[0]).toMatchObject({
      name: "uat",
      env_type: "staging",
      cluster_id: "c1",
      namespace: "pay-uat",
      project_ids: ["p1", "p2"],
    });
    expect(candidates[1]).toMatchObject({
      name: "prod",
      env_type: "production",
      cluster_id: "c2",
      namespace: "pay-prod",
      project_ids: ["p3"],
    });
  });
});

describe("suggestEnvironmentNamespace", () => {
  it("builds product-env-envName", () => {
    expect(suggestEnvironmentNamespace("Pay Service", "UAT")).toBe("pay-service-env-uat");
    expect(suggestEnvironmentNamespace("demo", "dev")).toBe("demo-env-dev");
  });
});

describe("resolveWorkspaceNamespace", () => {
  it("returns the most common namespace among bound active projects", () => {
    const workspaceProjects = [
      project({ id: "p1", name: "a", namespace: "test-workspace" }),
      project({ id: "p2", name: "b", namespace: "test-workspace" }),
      project({ id: "p3", name: "c", namespace: "other-space" }),
    ];
    expect(resolveWorkspaceNamespace([{ id: "p1" }, { id: "p2" }, { id: "p3" }], workspaceProjects)).toBe(
      "test-workspace",
    );
  });

  it("falls back to any active workspace project namespace when none are bound", () => {
    const workspaceProjects = [
      project({ id: "p1", name: "a", namespace: "fallback-ns", status: "active" }),
    ];
    expect(resolveWorkspaceNamespace([], workspaceProjects)).toBe("fallback-ns");
  });
});
