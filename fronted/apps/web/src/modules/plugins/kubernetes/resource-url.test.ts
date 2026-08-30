import { describe, expect, it } from "vitest";
import { CATALOG } from "./catalog";
import { resourceCollectionUrl } from "./resource-url";

describe("kubernetes resource collection URL", () => {
  it("builds backend pagination query for cluster scoped resources", () => {
    const node = mustResource("Node");
    expect(resourceCollectionUrl("local", node, { page: 2, pageSize: 50, keyword: "worker" })).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/local/proxy/api/v1/nodes?page=2&pageSize=50&keyword=worker",
    );
  });

  it("requires an explicit namespace for namespaced resources", () => {
    const deployment = mustResource("Deployment");
    expect(resourceCollectionUrl("prod", deployment, { page: 1, pageSize: 20 })).toBeNull();
    expect(
      resourceCollectionUrl("prod", deployment, {
        namespace: "apps",
        page: 1,
        pageSize: 20,
        keyword: "api/web",
      }),
    ).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/prod/proxy/apis/apps/v1/namespaces/apps/deployments?page=1&pageSize=20&keyword=api%2Fweb",
    );
  });
});

function mustResource(kind: string) {
  const found = CATALOG.find((item) => item.kind === kind);
  if (!found) throw new Error(`missing ${kind}`);
  return found;
}
