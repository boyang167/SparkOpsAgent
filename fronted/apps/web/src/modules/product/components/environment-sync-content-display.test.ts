import { describe, expect, it } from "vitest";
import { buildProjectSyncContentLines } from "./environment-sync-content-display";

describe("buildProjectSyncContentLines", () => {
  it("shows full image transition from source to target", () => {
    const lines = buildProjectSyncContentLines({
      projectId: "p1",
      sourceProjects: [{
        project_id: "p1",
        project_name: "app",
        enabled: true,
        current_image: "harbor.example/team/app:v2.1.0",
      }],
      targetProjects: [{
        project_id: "p1",
        project_name: "app",
        enabled: true,
        current_image: "harbor.example/team/app:v2.0.0",
      }],
      syncImage: true,
      syncYaml: false,
      firstSync: false,
    });
    expect(lines[0]).toMatchObject({
      label: "容器镜像",
      value: "harbor.example/team/app:v2.1.0 → harbor.example/team/app:v2.0.0",
    });
  });

  it("falls back to tag when full image is unavailable", () => {
    const lines = buildProjectSyncContentLines({
      projectId: "p1",
      sourceProjects: [{ project_id: "p1", project_name: "app", enabled: true, current_image_tag: "v2.1.0" }],
      targetProjects: [],
      syncImage: true,
      syncYaml: true,
      syncResourceKinds: ["Deployment", "Service", "ConfigMap", "Ingress"],
      firstSync: true,
    });
    expect(lines[0]?.value).toBe("v2.1.0 → 未部署");
    expect(lines.some((line) => line.label === "部署配置" && line.value.includes("Deployment"))).toBe(true);
  });
});
