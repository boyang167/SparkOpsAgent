import { describe, expect, it } from "vitest";
import { emptyBuildJobSpec, parseBuildJobSpec, type BuildJobSpec } from "./build-job-spec";

describe("parseBuildJobSpec", () => {
  it("keeps registry, services, strategy and notify from a saved spec", () => {
    const spec = parseBuildJobSpec({
      registry_id: "reg-1",
      project_builds: [
        {
          project_id: "p1",
          build_name: "orders-build",
          branch: "release",
          git_url: "https://git.example/orders",
          secret_id: "git-1",
          git_ref_type: "tag",
          dockerfile_path: "Dockerfile",
          variables: [{ key: "ENV", value: "dev" }],
          script: "echo hi",
        },
      ],
      strategy: { timeout_minutes: 30, cache: true },
      notify: { on_success: true, on_failure: false, message: "done" },
    });

    expect(spec.registry_id).toBe("reg-1");
    expect(spec.project_builds).toEqual([
      {
        project_id: "p1",
        build_name: "orders-build",
        branch: "release",
        git_url: "https://git.example/orders",
        secret_id: "git-1",
        git_ref_type: "tag",
        dockerfile_path: "Dockerfile",
        variables: [{ key: "ENV", value: "dev" }],
        script: "echo hi",
      },
    ]);
    expect(spec.strategy).toEqual({ timeout_minutes: 30, cache: true });
    expect(spec.notify).toEqual({ on_success: true, on_failure: false, message: "done" });
  });

  it("falls back to the job branch for legacy project_builds", () => {
    const spec = parseBuildJobSpec({
      branch: "main",
      project_builds: [{ project_id: "p1" }],
    });
    expect(spec.project_builds[0]).toMatchObject({
      project_id: "p1",
      branch: "main",
      git_ref_type: "branch",
      build_name: "",
    });
  });

  it("returns an empty editable spec", () => {
    const spec: BuildJobSpec = emptyBuildJobSpec();
    expect(spec.registry_id).toBe("");
    expect(spec.project_builds).toEqual([]);
    expect(spec.strategy.timeout_minutes).toBe(60);
    expect(spec.notify.on_failure).toBe(true);
  });
});
