import { afterEach, describe, expect, it, vi } from "vitest";
import { KubernetesApiError, k8sFetch, parseApiError } from "./api";

describe("kubernetes api helper", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("unwraps SparkOps envelope data while preserving business JSON", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse({
          code: 0,
          message: "ok",
          data: { items: [{ metadata: { name: "default" } }], total: 1 },
          trace_id: "trace-ok",
        }),
      ),
    );

    await expect(k8sFetch("/api/bff/v1/plugins/kubernetes/clusters/c/proxy/api/v1/namespaces")).resolves.toEqual({
      items: [{ metadata: { name: "default" } }],
      total: 1,
    });
  });

  it("keeps direct JSON and NDJSON responses intact", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ items: [1] })));
    await expect(k8sFetch("/direct")).resolves.toEqual({ items: [1] });

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(textResponse("{\"time\":0}\n", "application/x-ndjson")));
    await expect(k8sFetch("/records/1")).resolves.toBe("{\"time\":0}\n");
  });

  it("extracts entitlement errors from top-level and nested data with trace id", async () => {
    const parsed = await parseApiError(
      jsonResponse(
        {
          code: "ENTITLEMENT_MODULE_DISABLED",
          message: "Kubernetes 插件未启用",
          data: { error_code: "PLUGIN_KUBERNETES_DISABLED", message: "disabled" },
          trace_id: "trace-denied",
        },
        403,
      ),
    );

    expect(parsed).toEqual({
      status: 403,
      code: "ENTITLEMENT_MODULE_DISABLED",
      message: "Kubernetes 插件未启用",
      detail: "disabled",
      traceId: "trace-denied",
    });
  });

  it("does not treat HTML error pages as JSON", async () => {
    const parsed = await parseApiError(textResponse("<html>bad gateway</html>", "text/html", 502));
    expect(parsed).toEqual({ status: 502, message: "Bad Gateway", detail: "<html>bad gateway</html>" });
  });

  it("throws KubernetesApiError for failed requests", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse({ code: "ENTITLEMENT_MODULE_DISABLED", message: "disabled" }, 403)),
    );

    await expect(k8sFetch("/disabled")).rejects.toMatchObject({
      status: 403,
      code: "ENTITLEMENT_MODULE_DISABLED",
      message: "disabled",
    } satisfies Partial<KubernetesApiError>);
  });
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    statusText: status === 200 ? "OK" : status === 403 ? "Forbidden" : "Bad Gateway",
    headers: { "content-type": "application/json" },
  });
}

function textResponse(body: string, contentType: string, status = 200) {
  return new Response(body, {
    status,
    statusText: status === 200 ? "OK" : "Bad Gateway",
    headers: { "content-type": contentType },
  });
}
