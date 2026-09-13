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
          {
            id: "a",
            cluster_name: "dev",
            server: "https://dev",
            available: true,
            is_build_env: true,
            priority_id: 0,
          },
          {
            id: "b",
            cluster_name: "prod",
            server: "https://prod",
            available: false,
            reason: "缺少 Kubernetes 密钥",
          },
        ],
      }),
    ).toEqual({
      clusters: [
        {
          id: "a",
          cluster_name: "dev",
          server: "https://dev",
          is_build_env: true,
          priority_id: 0,
          available: true,
        },
        {
          id: "b",
          cluster_name: "prod",
          server: "https://prod",
          is_build_env: false,
          priority_id: 0,
          available: false,
        },
      ],
      missing: ["缺少 Kubernetes 密钥"],
    });
  });

  it("accepts legacy name field when cluster_name is absent", () => {
    expect(
      normalizeDeploymentOptions({
        clusters: [{ id: "a", name: "legacy-dev", available: true }],
      }),
    ).toEqual({
      clusters: [
        {
          id: "a",
          cluster_name: "legacy-dev",
          server: undefined,
          is_build_env: false,
          priority_id: 0,
          available: true,
        },
      ],
      missing: [],
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
