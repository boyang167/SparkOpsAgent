import { describe, expect, it } from "vitest";
import { buildConfigResourceItems, defaultConfigResources, imageRepositoryPath, imageTagFromRef, sourceImageLines } from "./deploy-job-spec";

describe("deploy image name", () => {
  it("sanitizes the branch into the image tag", () => {
    expect(imageTagFromRef("feature/login")).toBe("feature-login");
    expect(imageTagFromRef("refs/tags/v1.0.0")).toBe("v1.0.0");
    expect(imageTagFromRef("")).toBe("latest");
  });

  it("builds the repository path from the registry host", () => {
    expect(imageRepositoryPath("https://192.168.5.24", "Team-A", "Orders")).toBe("192.168.5.24/team-a/orders");
  });

  it("shows the image produced by the source build job", () => {
    const lines = sourceImageLines(
      {
        registry_id: "harbor",
        project_builds: [{ project_id: "p1", branch: "main" }],
      },
      [{ id: "harbor", serviceUrl: "https://harbor.example" }],
      [{ id: "p1", name: "orders", namespace: "team-a", displayName: "订单" }],
    );
    expect(lines).toEqual([
      { projectId: "p1", name: "订单", image: "harbor.example/team-a/orders:main", complete: true },
    ]);
  });

  it("lists referenced config resources as selected by default and keeps unrelated maps visible", () => {
    const items = buildConfigResourceItems(
      "p1",
      [
        { name: "app-config", namespace: "dev", data: { "app.yaml": "level: info" } },
        { name: "extra-flags", namespace: "dev", data: { flag: "off" } },
      ],
      [
        {
          kind: "ConfigMap",
          name: "app-config",
          related_reason: "referenced by Deployment envFrom",
          yaml: "kind: ConfigMap",
        },
        { kind: "Secret", name: "registry-pull", related_reason: "image pull secret", note: "不会复制凭据" },
        { kind: "Deployment", name: "orders", related_reason: "core workload" },
      ],
    );
    expect(items.map((item) => [item.kind, item.name, item.referenced])).toEqual([
      ["ConfigMap", "app-config", true],
      ["ConfigMap", "extra-flags", false],
      ["Secret", "registry-pull", true],
    ]);
    expect(defaultConfigResources(items)).toEqual([
      { projectId: "p1", kind: "ConfigMap", name: "app-config", yaml: "kind: ConfigMap" },
      { projectId: "p1", kind: "Secret", name: "registry-pull", yaml: "" },
    ]);
  });
});
