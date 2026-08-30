import { afterEach, describe, expect, it, vi } from "vitest";
import {
  buildKubernetesPluginBffPath,
  kubernetesPluginFetch,
} from "./plugin-kubernetes";

describe("buildKubernetesPluginBffPath", () => {
  it("encodes kubernetes plugin route segments for the BFF", () => {
    expect(buildKubernetesPluginBffPath(["clusters", "名字"], "?watch=1")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/%E5%90%8D%E5%AD%97?watch=1",
    );
  });

  it.each([
    [[".."]],
    [["%2e%2e"]],
    [["%25252e"]],
    [["pods/admin"]],
    [["pods%2Fadmin"]],
    [["pods%252fadmin"]],
    [["pods\\admin"]],
    [["http:"]],
    [["name%25252541"]],
    [["bad%zz"]],
  ])("rejects unsafe route segments %#", (segments) => {
    expect(() => buildKubernetesPluginBffPath(segments)).toThrow("invalid kubernetes plugin path");
  });
});

describe("kubernetesPluginFetch", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("calls the BFF catch-all with credentials and workspace header", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("ok"));
    vi.stubGlobal("fetch", fetchMock);

    await kubernetesPluginFetch(["clusters", "dev"], {
      method: "POST",
      workspaceId: "workspace-1",
      headers: {
        Accept: "application/json",
      },
      body: "{}",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/bff/v1/plugins/kubernetes/clusters/dev",
      expect.objectContaining({
        method: "POST",
        body: "{}",
        credentials: "include",
      }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const headers = new Headers(init.headers);
    expect(headers.get("Accept")).toBe("application/json");
    expect(headers.get("X-Workspace-Id")).toBe("workspace-1");
    expect(headers.has("Cookie")).toBe(false);
    expect(headers.has("X-Token")).toBe(false);
  });
});
