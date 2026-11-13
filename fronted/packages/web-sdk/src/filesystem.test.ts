import { afterEach, describe, expect, it, vi } from "vitest";
import type { TransferSessionVM } from "@sparkops/contracts/filesystem";
import { uploadTransfer } from "./filesystem";

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

  it("uploads only missing parts when resuming", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const fileFixture = new Blob([..."abcdefghijklmnop".split("")]);
    const uploaded = await uploadTransfer(fileFixture, {
      session: sessionWithCompletedParts([1, 2]),
      partSize: 5,
      workspaceId: "00000000-0000-0000-0000-000000000001",
    });
    expect(uploaded.requestedParts).toEqual([3, 4]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("https://storage.example/parts/3");
    expect(fetchMock.mock.calls[1]?.[0]).toBe("https://storage.example/parts/4");
  });
});
