import { describe, expect, it } from "vitest";
import { bucketRecordFrames, coalesceRecordFrames, ndjsonToAsciicastV2, parseRecordFrames, parseRecordObjectPath } from "./records-view";

describe("records view", () => {
  it("parses object path", () => {
    expect(parseRecordObjectPath("namespaces/default/pods/web-0/containers/app")).toEqual({
      namespace: "default",
      pod: "web-0",
      container: "app",
    });
  });

  it("parses ndjson frames", () => {
    expect(parseRecordFrames('{"time":0,"stream":"stdout","data":"ok"}\n')).toHaveLength(1);
  });

  it("coalesces consecutive stdout chunks", () => {
    const merged = coalesceRecordFrames([
      { time: 0, stream: "stdout", data: "hel" },
      { time: 0, stream: "stdout", data: "lo" },
      { time: 0.3, stream: "stdout", data: "!" },
    ]);
    expect(merged).toEqual([
      { time: 0, stream: "stdout", data: "hello" },
      { time: 0.3, stream: "stdout", data: "!" },
    ]);
  });

  it("buckets frames to fixed fps timeline", () => {
    const buckets = bucketRecordFrames([
      { time: 0, stream: "stdout", data: "a" },
      { time: 0.05, stream: "stdout", data: "b" },
      { time: 0.25, stream: "stdout", data: "c" },
    ], 5);
    expect(buckets).toEqual([
      { time: 0, stream: "stdout", data: "ab" },
      { time: 0.2, stream: "stdout", data: "c" },
    ]);
  });

  it("converts ndjson to asciicast", () => {
    const cast = ndjsonToAsciicastV2('{"time":0,"stream":"stdout","data":"ok"}\n');
    expect(cast).toContain('"version":2');
    expect(cast).toContain('"ok"');
  });
});
