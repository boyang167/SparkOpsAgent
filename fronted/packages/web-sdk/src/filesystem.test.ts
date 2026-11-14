import { afterEach, describe, expect, it, vi } from "vitest";
import type { TransferSessionVM } from "@sparkops/contracts/filesystem";
import { listFileNodes, moveFileNode, uploadTransfer } from "./filesystem";

function sessionWithCompletedParts(partNumbers: number[]): TransferSessionVM {
  const partSize = 5;
  const totalParts = 4;
  const parts = Array.from({ length: totalParts }, (_, index) => {
    const partNumber = index + 1;
    const completed = partNumbers.includes(partNumber);
    return {
      part_number: partNumber,
      offset: (partNumber - 1) * partSize,
      size: partSize,
      etag: completed ? `etag-${partNumber}` : "",
      upload_url: completed ? undefined : `https://storage.example/parts/${partNumber}`,
    };
  });
  return {
    session_id: "00000000-0000-0000-0000-000000000099",
    workspace_id: "00000000-0000-0000-0000-000000000001",
    namespace_id: "00000000-0000-0000-0000-000000000002",
    status: "UPLOADING",
    protocol: "MULTIPART",
    target_name: "payload.bin",
    expected_size: totalParts * partSize,
    part_size: partSize,
    total_parts: totalParts,
    expires_at: new Date(Date.now() + 3_600_000).toISOString(),
    parts,
  };
}

describe("filesystem transfer sdk", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uploads a direct transfer through the gateway capability contract", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    const session: TransferSessionVM = {
      ...sessionWithCompletedParts([]),
      protocol: "DIRECT",
      upload_url: "https://gateway.example/v1/transfers/session/content",
      capability_token: "upload-capability",
      parts: [],
      total_parts: 1,
    };
    const progress = vi.fn();
    const fileFixture = new Blob(["payload"]);

    const uploaded = await uploadTransfer(fileFixture, {
      session,
      workspaceId: session.workspace_id,
      fetchImpl: fetchMock,
      onProgress: progress,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      session.upload_url,
      expect.objectContaining({
        method: "PUT",
        body: fileFixture,
        headers: { Authorization: "Bearer upload-capability" },
      }),
    );
    expect(progress).toHaveBeenLastCalledWith({
      uploadedBytes: fileFixture.size,
      totalBytes: fileFixture.size,
      completedParts: 1,
      totalParts: 1,
    });
    expect(uploaded.requestedParts).toEqual([1]);
  });

  it("uploads only missing parts when resuming", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const fileFixture = new Blob([..."abcdefghijklmnop".split("")]);
    const controller = new AbortController();
    const uploaded = await uploadTransfer(fileFixture, {
      session: sessionWithCompletedParts([1, 2]),
      partSize: 5,
      workspaceId: "00000000-0000-0000-0000-000000000001",
      signal: controller.signal,
    });
    expect(uploaded.requestedParts).toEqual([3, 4]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("https://storage.example/parts/3");
    expect(fetchMock.mock.calls[1]?.[0]).toBe("https://storage.example/parts/4");
    expect(fetchMock.mock.calls.every(([, init]) => init?.signal === controller.signal)).toBe(true);
  });

  it("stops multipart upload on a non-successful PUT without reporting progress", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("failed", { status: 503 }));
    const progress = vi.fn();

    await expect(
      uploadTransfer(new Blob([..."abcdefghijklmnop".split("")]), {
        session: sessionWithCompletedParts([1, 2]),
        partSize: 5,
        workspaceId: "00000000-0000-0000-0000-000000000001",
        fetchImpl: fetchMock,
        onProgress: progress,
      }),
    ).rejects.toThrow("transfer upload failed: 503");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(progress).not.toHaveBeenCalled();
  });
});

describe("filesystem node sdk", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("encodes a directory type filter after the parent id", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 0,
          message: "ok",
          data: { items: [], next_cursor: "" },
          trace_id: "trace-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await listFileNodes("workspace-1", {
      parentId: "parent-1",
      nodeType: "DIRECTORY",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("parent_id=parent-1&type=DIRECTORY"),
      expect.anything(),
    );
  });

  it("moves a node with the backend request fields and an idempotency key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({ code: 0, message: "ok", data: null, trace_id: "trace-1" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await moveFileNode("workspace-1", "node-1", "target-1", "报告.md", {
      idempotencyKey: "move-key-1",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/bff/v1/workspaces/workspace-1/files/node-1/move",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ parent_id: "target-1", name: "报告.md" }),
        headers: expect.any(Headers),
      }),
    );
    const options = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(options.headers).get("Idempotency-Key")).toBe("move-key-1");
  });

  it("generates non-empty distinct idempotency keys for concurrent moves", async () => {
    vi.spyOn(Date, "now").mockReturnValue(1_700_000_000_000);
    const fetchMock = vi.fn().mockImplementation(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({ code: 0, message: "ok", data: null, trace_id: "trace-1" }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await Promise.all([
      moveFileNode("workspace-1", "node-1", "target-1", "A.txt"),
      moveFileNode("workspace-1", "node-2", "target-1", "B.txt"),
    ]);

    const keys = fetchMock.mock.calls.map(([, init]) =>
      new Headers((init as RequestInit).headers).get("Idempotency-Key"),
    );
    expect(keys[0]).toBeTruthy();
    expect(keys[1]).toBeTruthy();
    expect(keys[0]).not.toBe(keys[1]);
  });
});
