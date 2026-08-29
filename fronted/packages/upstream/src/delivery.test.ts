import { describe, expect, it } from "vitest";
import {
  normalizeDeploymentList,
  normalizeDeploymentOptions,
  normalizeDeploymentRun,
} from "./delivery";

describe("normalizeDeploymentOptions", () => {
  it("adapts backend availability details to the public contract", () => {
    expect(
      normalizeDeploymentOptions({
        clusters: [
          { id: "a", name: "dev", server: "https://dev", available: true },
          {
            id: "b",
            name: "prod",
            server: "https://prod",
            available: false,
            reason: "缺少 Kubernetes 密钥",
          },
        ],
      }),
    ).toEqual({
      clusters: [{ id: "a", cluster_name: "dev", server: "https://dev" }],
      missing: ["缺少 Kubernetes 密钥"],
    });
  });

  it("fills fields omitted by the backend envelope", () => {
    expect(
      normalizeDeploymentRun({
        id: "run",
        workspace_id: "w",
        project_id: "p",
        cluster_id: "c",
        git_ref_type: "branch",
        git_ref_value: "main",
        image_tag: "run",
        status: "succeeded",
        created_by: "m",
        created_at: "2026-08-29T00:00:00Z",
      }),
    ).toMatchObject({ current_stage: "", error_message: "" });
  });

  it("keeps backend totals and adapts legacy item-only lists", () => {
    expect(
      normalizeDeploymentList({ items: [], total: 42, page: 2, page_size: 20 }),
    ).toMatchObject({ total: 42, page: 2, page_size: 20 });
    expect(normalizeDeploymentList({ items: [] })).toEqual({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
  });
});
