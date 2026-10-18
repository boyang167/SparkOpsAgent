import { describe, expect, it } from "vitest";
import {
  confirmEnvironmentSyncPreview,
  createEnvironmentSyncRunState,
  updateEnvironmentSyncProject,
  type EnvironmentSyncPreviewVM,
} from "./environment-sync-run-state";

const preview: EnvironmentSyncPreviewVM = {
  preview_id: "preview-1",
  source_environment_id: "dev",
  target_environment_id: "prod",
  projects: [
    {
      project_id: "p1",
      first_sync: false,
      sync_image: true,
      sync_yaml: false,
      source_image: "registry/app:v2",
      target_image: "registry/app:v1",
      diffs: [{ kind: "Deployment", name: "app", action: "update" }],
    },
  ],
};

describe("environment-sync-run-state", () => {
  it("invalidates confirmation when any option changes", () => {
    const state = createEnvironmentSyncRunState({
      jobName: "promote",
      sourceEnvironmentId: "dev",
      targetEnvironmentId: "prod",
      projects: [{ projectId: "p1", syncImage: true, syncYaml: false, syncResourceKinds: [],
      resourceOverrides: {} }],
    });
    const confirmed = confirmEnvironmentSyncPreview(state, preview);
    expect(confirmed.previewId).toBe("preview-1");
    expect(updateEnvironmentSyncProject(confirmed, "p1", { syncYaml: true }).previewId).toBe("");
  });

  it("builds preview id map for selected jobs", async () => {
    const { environmentSyncPreviewIdsFromStates } = await import("./environment-sync-run-state");
    const state = confirmEnvironmentSyncPreview(
      createEnvironmentSyncRunState({
        jobName: "promote",
        sourceEnvironmentId: "dev",
        targetEnvironmentId: "prod",
        projects: [{ projectId: "p1", syncImage: true, syncYaml: false, syncResourceKinds: [],
      resourceOverrides: {} }],
      }),
      preview,
    );
    expect(
      environmentSyncPreviewIdsFromStates(
        [
          {
            jobName: "promote",
            sourceEnvironmentId: "dev",
            targetEnvironmentId: "prod",
            projects: [{ projectId: "p1", syncImage: true, syncYaml: false, syncResourceKinds: [],
      resourceOverrides: {} }],
          },
        ],
        { promote: state },
      ),
    ).toEqual({ promote: "preview-1" });
  });
});
