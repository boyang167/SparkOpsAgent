import { describe, expect, it } from "vitest";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import {
  artifactsFromDeploymentRuns,
  defaultReleaseSource,
  hasProjectJenkinsTemplate,
  pipelineContainsBuild,
  sourceStepWarnings,
  validateReleaseDraft,
} from "./release-draft";
import type { ReleaseDraft } from "../types";
import { getMockPipeline } from "../mock/data";

function run(partial: Partial<DeploymentRunVM> & Pick<DeploymentRunVM, "id" | "status" | "image_tag">): DeploymentRunVM {
  return {
    workspace_id: "ws",
    project_id: "p",
    cluster_id: "c",
    git_ref_type: "branch",
    git_ref_value: "main",
    error_message: "",
    current_stage: "",
    created_by: "u",
    created_at: "2026-09-01T00:00:00Z",
    ...partial,
  };
}

function draft(partial: Partial<ReleaseDraft> = {}): ReleaseDraft {
  return {
    source: "artifact",
    pipelineId: "pipe-release-staging-then-prod",
    parameters: {},
    fleetIds: ["fleet-staging-all"],
    gitRefType: "branch",
    gitRefValue: "main",
    imageTag: "abc123",
    imageRepository: "harbor.local/demo/app",
    ...partial,
  };
}

describe("release-draft", () => {
  it("lists unique succeeded image tags and skips failed runs", () => {
    const artifacts = artifactsFromDeploymentRuns(
      [
        run({ id: "1", status: "succeeded", image_tag: "tag-a", build_id: "b1" }),
        run({ id: "2", status: "failed", image_tag: "tag-b" }),
        run({ id: "3", status: "succeeded", image_tag: "tag-a" }),
        run({ id: "4", status: "succeeded", image_tag: "tag-c", git_ref_value: "release" }),
      ],
      "harbor.local/ns/app",
    );
    expect(artifacts.map((item) => item.image_tag)).toEqual(["tag-a", "tag-c"]);
    expect(artifacts[0]?.image_repository).toBe("harbor.local/ns/app");
  });

  it("detects build steps on CI pipelines and not on release-only pipelines", () => {
    expect(pipelineContainsBuild(getMockPipeline("pipe-staging-only")!.definition.spec.stages)).toBe(true);
    expect(pipelineContainsBuild(getMockPipeline("pipe-release-staging-then-prod")!.definition.spec.stages)).toBe(
      false,
    );
  });

  it("requires a tag when using an existing image", () => {
    const errors = validateReleaseDraft(draft({ imageTag: "" }), {
      hasJenkinsTemplate: true,
      buildBlockedReasons: [],
    });
    expect(errors.some((e) => e.includes("镜像"))).toBe(true);
  });

  it("defaults to build when there is no artifact yet", () => {
    expect(defaultReleaseSource(0)).toBe("build");
    expect(defaultReleaseSource(2)).toBe("artifact");
  });

  it("treats project template_id as a Jenkins template binding", () => {
    expect(hasProjectJenkinsTemplate({ template_id: "tpl-1" })).toBe(true);
    expect(hasProjectJenkinsTemplate({ resources: { jenkinsfile_template_id: "jf-1" } })).toBe(true);
    expect(hasProjectJenkinsTemplate({})).toBe(false);
  });

  it("does not block build-then-release on missing cluster or secret warnings", () => {
    const errors = validateReleaseDraft(draft({ source: "build", imageTag: "" }), {
      hasJenkinsTemplate: true,
      buildBlockedReasons: ["Jenkins 密钥", "未授权集群"],
    });
    expect(errors).toEqual([]);
  });

  it("never shows cluster or DeploymentRun gates on the artifact-source step", () => {
    expect(
      sourceStepWarnings({
        source: "build",
        missing: ["Jenkins 密钥未配置", "未授权集群（先构建仍需一个目标集群写入 DeploymentRun）"],
        clusterCount: 0,
      }),
    ).toEqual([]);
  });

  it("requires git ref and at least one fleet", () => {
    const errors = validateReleaseDraft(draft({ source: "build", imageTag: "", gitRefValue: "", fleetIds: [] }), {
      hasJenkinsTemplate: true,
      buildBlockedReasons: [],
    });
    expect(errors.some((e) => e.includes("Git"))).toBe(true);
    expect(errors.some((e) => e.includes("Fleet"))).toBe(true);
  });

  it("accepts a complete existing-image draft", () => {
    expect(
      validateReleaseDraft(draft(), {
        hasJenkinsTemplate: false,
        buildBlockedReasons: ["Jenkins 密钥"],
      }),
    ).toEqual([]);
  });
});
