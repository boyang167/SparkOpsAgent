import { describe, expect, it } from "vitest";
import { deploymentRunTitle } from "./deployment-console-utils";
import { parseProjectDetailTab, projectDetailTabs } from "./project-detail-tabs";

describe("projectDetailTabs", () => {
  it("shows yaml/jenkins/dockerfile before publish", () => {
    const tabs = projectDetailTabs(false);
    expect(tabs.map((tab) => tab.id)).toEqual(["overview", "yaml", "jenkins", "dockerfile"]);
  });

  it("shows published tabs after deploy", () => {
    const tabs = projectDetailTabs(true);
    expect(tabs.map((tab) => tab.id)).toEqual([
      "overview",
      "build-logs",
      "configmap",
      "storage",
      "webhooks",
      "monitor",
    ]);
  });

  it("falls back to overview for unpublished-only tabs when published", () => {
    expect(parseProjectDetailTab("yaml", true)).toBe("overview");
    expect(parseProjectDetailTab("config", true)).toBe("overview");
    expect(parseProjectDetailTab("build-logs", true)).toBe("build-logs");
    expect(parseProjectDetailTab("configmap", true)).toBe("configmap");
  });
});

describe("deploymentRunTitle", () => {
  it("prefers git ref and image tag", () => {
    expect(
      deploymentRunTitle({
        id: "run-1",
        workspace_id: "ws",
        project_id: "p",
        cluster_id: "c",
        git_ref_type: "branch",
        git_ref_value: "master",
        image_tag: "v1.0.1",
        cross_cluster: false,
        run_type: "normal",
        status: "failed",
        error_message: "",
        current_stage: "",
        created_by: "u",
        created_at: "2026-09-06T00:00:00Z",
      }),
    ).toBe("master · v1.0.1");
  });
});
