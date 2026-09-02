import { BffError } from "@sparkops/contracts";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { BffContext } from "./index";
import {
  buildKubernetesPluginApiPath,
  proxyUpstreamStreamRequest,
} from "./index";

const maxBodyBytes = 50 * 1024 * 1024;

function createContext(): BffContext {
  return {
    traceId: "trace-1",
    token: "token-1",
    workspaceId: "workspace-1",
    upstream: {
      platform: {
        getWorkspaceEntitlements: vi.fn().mockResolvedValue({
          agent: true,
          knowledge: true,
          delivery: true,
          observability: true,
          k8s: true,
          mcp: true,
          plugins: ["kubernetes"],
        }),
      },
    },
  } as unknown as BffContext;
}

describe("buildKubernetesPluginApiPath", () => {
  it("encodes every segment under the kubernetes plugin API", () => {
    expect(buildKubernetesPluginApiPath(["clusters", "名字"], "?watch=1")).toBe(
      "/api/v1/plugins/kubernetes/clusters/%E5%90%8D%E5%AD%97?watch=1",
    );
  });

  it("allows validated service proxy segments only after services", () => {
    expect(
      buildKubernetesPluginApiPath([
        "clusters", "prod", "proxy", "api", "v1", "namespaces", "default",
        "services", "http:sparkops-cloudtty-abcd:7681", "proxy",
      ]),
    ).toBe(
      "/api/v1/plugins/kubernetes/clusters/prod/proxy/api/v1/namespaces/default/services/http:sparkops-cloudtty-abcd:7681/proxy",
    );
  });

  it("allows regular service resource names after services", () => {
    expect(
      buildKubernetesPluginApiPath([
        "clusters", "prod", "proxy", "api", "v1", "namespaces", "argocd",
        "services", "devops-argocd-dex-server",
      ]),
    ).toBe(
      "/api/v1/plugins/kubernetes/clusters/prod/proxy/api/v1/namespaces/argocd/services/devops-argocd-dex-server",
    );
  });

  it.each([
    "http:..:80",
    "http:name:0",
    "http:name:65536",
    "ftp:name:80",
    "http:user@name:80",
    "http:name%2Fevil:80",
  ])("rejects malicious Kubernetes service proxy segment %s", (service) => {
    expect(() => buildKubernetesPluginApiPath(["services", service])).toThrow(BffError);
  });

  it("rejects service-shaped scheme segments outside the services position", () => {
    expect(() => buildKubernetesPluginApiPath(["pods", "http:name:7681"])).toThrow(BffError);
  });

  it.each([
    [[]],
    [[""]],
    [["."]],
    [[".."]],
    [["%2e%2e"]],
    [["%25252e"]],
    [["pods%2Fadmin"]],
    [["pods%252fadmin"]],
    [["name%25252541"]],
    [["bad%zz"]],
    [["pods\\admin"]],
    [["pods\u0000admin"]],
    [["http:"]],
  ])("rejects unsafe path segments %#", (segments) => {
    expect(() => buildKubernetesPluginApiPath(segments)).toThrow(BffError);
  });
});

describe("proxyUpstreamStreamRequest", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
  });

  it("strips sensitive request headers and injects upstream identity headers", async () => {
    vi.stubEnv("UPSTREAM_URL", "http://upstream.test");
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const req = new Request("http://bff.test/api/bff/v1/plugins/kubernetes/pods", {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/yaml",
        "If-None-Match": "etag-1",
        Cookie: "sparkops_token=leak",
        "X-Token": "leak",
        Impersonate: "leak",
        Host: "evil.test",
      },
      body: "kind: Pod",
    });

    await proxyUpstreamStreamRequest(createContext(), "/api/v1/plugins/kubernetes/pods", req);

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const headers = new Headers(init.headers);
    expect(headers.get("Accept")).toBe("application/json");
    expect(headers.get("Content-Type")).toBe("application/yaml");
    expect(headers.get("If-None-Match")).toBe("etag-1");
    expect(headers.get("Authorization")).toBe("Bearer token-1");
    expect(headers.get("X-Workspace-Id")).toBe("workspace-1");
    expect(headers.get("X-Trace-Id")).toBe("trace-1");
    expect(headers.has("Cookie")).toBe(false);
    expect(headers.has("X-Token")).toBe(false);
    expect(headers.has("Impersonate")).toBe(false);
    expect(headers.has("Host")).toBe(false);
  });

  it("forwards a CloudTTY service proxy path through the catch-all unchanged", async () => {
    vi.stubEnv("UPSTREAM_URL", "http://upstream.test");
    const fetchMock = vi.fn().mockResolvedValue(new Response("<html />", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const path = buildKubernetesPluginApiPath([
      "clusters", "prod", "proxy", "api", "v1", "namespaces", "ns",
      "services", "http:name:7681", "proxy", "",
    ].slice(0, -1));

    await proxyUpstreamStreamRequest(createContext(), `${path}/`, new Request("http://bff.test/cloudtty"));

    expect(fetchMock.mock.calls[0]?.[0]).toBe(`http://upstream.test${path}/`);
  });

  it("propagates the incoming abort signal to upstream fetch", async () => {
    vi.stubEnv("UPSTREAM_URL", "http://upstream.test");
    const fetchMock = vi.fn().mockResolvedValue(new Response("data: ok\n\n"));
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();
    const req = new Request("http://bff.test/api/bff/v1/plugins/kubernetes/logs", {
      signal: controller.signal,
    });

    await proxyUpstreamStreamRequest(createContext(), "/api/v1/plugins/kubernetes/logs", req);

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(init.signal).toBe(req.signal);
  });

  it("rejects request bodies larger than 50 MiB before calling upstream", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new Uint8Array(maxBodyBytes + 1));
        controller.close();
      },
    });
    const req = new Request("http://bff.test/api/bff/v1/plugins/kubernetes/upload", {
      method: "POST",
      body,
      duplex: "half",
    } as RequestInit & { duplex: "half" });

    await expect(
      proxyUpstreamStreamRequest(createContext(), "/api/v1/plugins/kubernetes/upload", req),
    ).rejects.toMatchObject({ status: 413, code: "PAYLOAD_TOO_LARGE" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("passes through upstream status, content type, and SSE body without an envelope", async () => {
    vi.stubEnv("UPSTREAM_URL", "http://upstream.test");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response("data: hello\n\n", {
          status: 202,
          headers: {
            "Content-Type": "text/event-stream",
            "X-Accel-Buffering": "no",
            "X-Internal-Secret": "hidden",
          },
        }),
      ),
    );

    const res = await proxyUpstreamStreamRequest(
      createContext(),
      "/api/v1/plugins/kubernetes/chat/query",
      new Request("http://bff.test/api/bff/v1/plugins/kubernetes/chat/query"),
    );

    expect(res.status).toBe(202);
    expect(res.headers.get("Content-Type")).toBe("text/event-stream");
    expect(res.headers.get("X-Accel-Buffering")).toBe("no");
    expect(res.headers.has("X-Internal-Secret")).toBe(false);
    expect(await res.text()).toBe("data: hello\n\n");
  });

  it("adds sandbox security headers for HTML responses without trusting upstream overrides", async () => {
    vi.stubEnv("UPSTREAM_URL", "http://upstream.test");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response("<html></html>", {
          status: 200,
          headers: {
            "Content-Type": "text/html; charset=utf-8",
            "Content-Security-Policy": "default-src *",
            "Set-Cookie": "leak=1",
          },
        }),
      ),
    );

    const res = await proxyUpstreamStreamRequest(
      createContext(),
      "/api/v1/plugins/kubernetes/clusters/prod/velaux/",
      new Request("http://bff.test/api/bff/v1/plugins/kubernetes/clusters/prod/velaux/"),
    );

    expect(res.headers.get("Content-Security-Policy")).toBe(
      "sandbox allow-scripts allow-forms allow-popups allow-downloads",
    );
    expect(res.headers.get("X-Content-Type-Options")).toBe("nosniff");
    expect(res.headers.get("Referrer-Policy")).toBe("no-referrer");
    expect(res.headers.has("Set-Cookie")).toBe(false);
  });

  it("rewrites same-origin upstream redirects to the BFF catch-all prefix", async () => {
    vi.stubEnv("UPSTREAM_URL", "http://upstream.test");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(null, {
          status: 302,
          headers: {
            Location: "/api/v1/plugins/kubernetes/clusters/prod/velaux/?next=1",
          },
        }),
      ),
    );

    const res = await proxyUpstreamStreamRequest(
      createContext(),
      "/api/v1/plugins/kubernetes/clusters/prod/velaux",
      new Request("http://bff.test/api/bff/v1/plugins/kubernetes/clusters/prod/velaux"),
    );

    expect(res.status).toBe(302);
    expect(res.headers.get("Location")).toBe(
      "/api/bff/v1/plugins/kubernetes/clusters/prod/velaux/?next=1",
    );
  });

  it("rejects upstream redirects to an external origin", async () => {
    vi.stubEnv("UPSTREAM_URL", "http://upstream.test");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(null, {
          status: 302,
          headers: {
            Location: "https://evil.test/steal",
          },
        }),
      ),
    );

    await expect(
      proxyUpstreamStreamRequest(
        createContext(),
        "/api/v1/plugins/kubernetes/clusters/prod/velaux",
        new Request("http://bff.test/api/bff/v1/plugins/kubernetes/clusters/prod/velaux"),
      ),
    ).rejects.toMatchObject({ status: 502, code: "UPSTREAM_REDIRECT_INVALID" });
  });

  it("maps upstream transport failures to a fixed 502 error", async () => {
    vi.stubEnv("UPSTREAM_URL", "http://secret-upstream.test");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("connect http://secret")));

    await expect(
      proxyUpstreamStreamRequest(
        createContext(),
        "/api/v1/plugins/kubernetes/pods",
        new Request("http://bff.test/api/bff/v1/plugins/kubernetes/pods"),
      ),
    ).rejects.toMatchObject({
      status: 502,
      code: "UPSTREAM_UNAVAILABLE",
      message: "upstream unavailable",
    });
  });
});
