import { afterEach, describe, expect, it, vi } from "vitest";
import { BffError } from "@sparkops/contracts";
import { bffFetch } from "./client";

function stubFetch(impl: () => Promise<Response>) {
  vi.stubGlobal("fetch", vi.fn(impl));
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("bffFetch", () => {
  it("turns a dropped response stream into a BffError", async () => {
    stubFetch(async () => ({
      ok: true,
      status: 200,
      json: () => Promise.reject(new TypeError("Error in input stream")),
    }) as unknown as Response);

    const error = await bffFetch("/workspaces").catch((err: unknown) => err);
    expect(error).toBeInstanceOf(BffError);
    expect((error as BffError).code).toBe("BFF_NETWORK_ERROR");
  });

  it("turns a failed request into a BffError", async () => {
    stubFetch(() => Promise.reject(new TypeError("Failed to fetch")));

    const error = await bffFetch("/workspaces").catch((err: unknown) => err);
    expect(error).toBeInstanceOf(BffError);
    expect((error as BffError).code).toBe("BFF_NETWORK_ERROR");
  });

  it("keeps caller-initiated aborts as AbortError", async () => {
    stubFetch(() => Promise.reject(new DOMException("aborted", "AbortError")));

    const error = await bffFetch("/workspaces").catch((err: unknown) => err);
    expect(error).toBeInstanceOf(DOMException);
    expect((error as DOMException).name).toBe("AbortError");
  });
});
