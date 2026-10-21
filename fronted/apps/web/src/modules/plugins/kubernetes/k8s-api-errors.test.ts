import { describe, expect, it } from "vitest";
import { humanizeK8sApiError, humanizeResourceLoadError, isCrdUnavailableError } from "./k8s-api-errors";

describe("humanizeK8sApiError", () => {
  it("maps CloudTTY CRD unavailable to a friendly message", () => {
    const result = humanizeK8sApiError({
      code: "PLUGIN_CLOUDTTY_CRD_UNAVAILABLE",
      message: "CloudTTY CRD is unavailable",
      status: 502,
      detail: '{"code":1,"message":"CloudTTY CRD is unavailable"}',
      traceId: "abc-123",
    });
    expect(result.known).toBe(true);
    expect(result.message).toBe("当前集群未安装 CloudTTY");
    expect(result.hint).toContain("CloudTTY Operator");
    expect(result.detail).toBeUndefined();
  });

  it("maps Prometheus not configured to a friendly message", () => {
    const result = humanizeK8sApiError({
      code: "PLUGIN_PROMETHEUS_NOT_CONFIGURED",
      message: "prometheus is not configured",
      status: 400,
      detail: '{"code":1}',
    });
    expect(result.known).toBe(true);
    expect(result.message).toBe("尚未配置 Prometheus");
    expect(result.hint).toContain("Prometheus 监控");
  });

  it("maps OpenKruise 404 to a friendly CRD message", () => {
    const result = humanizeResourceLoadError(
      { status: 404, message: "Not Found", detail: "404 page not found" },
      "CloneSet",
      "apps.kruise.io",
    );
    expect(result.known).toBe(true);
    expect(result.message).toBe("当前集群未安装 OpenKruise");
    expect(result.detail).toBeUndefined();
    expect(isCrdUnavailableError({ status: 404, message: "Not Found" }, "CloneSet", "apps.kruise.io")).toBe(true);
  });

  it("maps Gateway API 404 to a friendly CRD message", () => {
    const result = humanizeResourceLoadError(
      { status: 404, message: "Not Found" },
      "GatewayClass",
      "gateway.networking.k8s.io",
    );
    expect(result.known).toBe(true);
    expect(result.message).toBe("当前集群未安装 Gateway API");
    expect(result.hint).toContain("GatewayClass");
  });

  it("passes through unknown errors unchanged", () => {
    const input = { code: "OTHER", message: "something failed", detail: "raw" };
    expect(humanizeK8sApiError(input)).toEqual(input);
  });
});
