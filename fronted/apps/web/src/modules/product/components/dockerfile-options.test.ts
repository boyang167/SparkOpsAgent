import { describe, expect, it } from "vitest";
import { isDockerfileName, toRepoRelativePath } from "./dockerfile-options";

describe("dockerfile options", () => {
  it("recognizes Dockerfile variants", () => {
    expect(isDockerfileName("Dockerfile")).toBe(true);
    expect(isDockerfileName("Dockerfile.dev")).toBe(true);
    expect(isDockerfileName("docker/Dockerfile")).toBe(true);
    expect(isDockerfileName("README.md")).toBe(false);
  });

  it("normalizes repo-relative paths", () => {
    expect(toRepoRelativePath("docker/Dockerfile")).toBe("./docker/Dockerfile");
    expect(toRepoRelativePath("./Dockerfile")).toBe("./Dockerfile");
  });
});
