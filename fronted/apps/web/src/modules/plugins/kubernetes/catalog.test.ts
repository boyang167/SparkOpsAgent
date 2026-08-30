import { describe, expect, it } from "vitest";
import {
  CATALOG,
  buildDeploymentManifest,
  proxyPath,
  resourcesForMenu,
  selectStoredClusterId,
  validateDeploymentDraft,
} from "./catalog";

describe("kubernetes catalog", () => {
  it("covers representative resource groups for menus", () => {
    expect(resourcesForMenu("nodes").map((item) => item.kind)).toContain("Node");
    expect(resourcesForMenu("workloads").map((item) => item.kind)).toEqual(
      expect.arrayContaining(["Pod", "Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Job", "CronJob"]),
    );
    expect(resourcesForMenu("gateway").map((item) => item.kind)).toEqual(
      expect.arrayContaining(["GatewayClass", "Gateway", "HTTPRoute", "GRPCRoute", "TCPRoute", "TLSRoute", "UDPRoute"]),
    );
    expect(resourcesForMenu("backup").map((item) => item.kind)).toEqual(
      expect.arrayContaining(["Backup", "Restore", "Schedule"]),
    );
    expect(CATALOG.find((item) => item.kind === "Namespace")).toMatchObject({
      group: "",
      version: "v1",
      plural: "namespaces",
      namespaced: false,
    });
  });

  it("builds encoded proxy paths for core, group, CRD, and cluster scoped resources", () => {
    const pod = CATALOG.find((item) => item.kind === "Pod");
    const deployment = CATALOG.find((item) => item.kind === "Deployment");
    const application = CATALOG.find((item) => item.kind === "Application");
    const clusterRole = CATALOG.find((item) => item.kind === "ClusterRole");

    expect(pod && proxyPath("local/one", pod, "dev ns", "api/pod")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/local%2Fone/proxy/api/v1/namespaces/dev%20ns/pods/api%2Fpod",
    );
    expect(deployment && proxyPath("prod", deployment, "apps", "web")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/prod/proxy/apis/apps/v1/namespaces/apps/deployments/web",
    );
    expect(application && proxyPath("prod", application, "vela-system")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/prod/proxy/apis/core.oam.dev/v1beta1/namespaces/vela-system/applications",
    );
    expect(clusterRole && proxyPath("prod", clusterRole, "ignored", "admin/view")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/prod/proxy/apis/rbac.authorization.k8s.io/v1/clusterroles/admin%2Fview",
    );
  });

  it("selects only cluster ids returned by the backend", () => {
    expect(selectStoredClusterId("prod", [{ id: "local" }, { id: "prod" }])).toBe("prod");
    expect(selectStoredClusterId("missing", [{ id: "local" }, { id: "prod" }])).toBe("local");
    expect(selectStoredClusterId(null, [{ id: "local" }])).toBe("local");
    expect(selectStoredClusterId("local", [])).toBe("");
  });

  it("builds a deployment manifest from the create wizard draft", () => {
    const manifest = buildDeploymentManifest({
      namespace: "apps",
      name: "spark-api",
      image: "registry.local/spark/api:1.2.3",
      replicas: 3,
      ports: "8080, 9090",
    });

    expect(manifest).toMatchObject({
      apiVersion: "apps/v1",
      kind: "Deployment",
      metadata: { name: "spark-api", namespace: "apps" },
      spec: {
        replicas: 3,
        selector: { matchLabels: { app: "spark-api" } },
        template: {
          metadata: { labels: { app: "spark-api" } },
          spec: {
            containers: [
              {
                name: "spark-api",
                image: "registry.local/spark/api:1.2.3",
                ports: [{ containerPort: 8080 }, { containerPort: 9090 }],
              },
            ],
          },
        },
      },
    });
  });

  it("validates deployment wizard draft fields", () => {
    expect(
      validateDeploymentDraft({
        namespace: "bad_ns",
        name: "Bad",
        image: "",
        replicas: 0,
        ports: "abc,70000",
      }),
    ).toEqual({
      namespace: "命名空间必须是 DNS label",
      name: "名称必须是 DNS subdomain",
      image: "镜像不能为空",
      replicas: "副本数必须在 1-1000 之间",
      ports: "端口必须是 1-65535 的数字",
    });
  });
});
