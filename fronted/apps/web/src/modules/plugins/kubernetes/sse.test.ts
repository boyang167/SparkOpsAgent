import { describe, expect, it } from "vitest";
import { appendBoundedLines, createSSEParser, formatSSEPayload } from "./sse";

describe("kubernetes SSE parser", () => {
  it("parses events split across chunks and decodes JSON strings", () => {
    const events: Array<{ event: string; data: unknown }> = [];
    const parser = createSSEParser((event) => events.push(event));

    parser.push('event: chunk\ndata: "hel');
    expect(events).toEqual([]);

    parser.push('lo"\n\n');
    expect(events).toEqual([{ event: "chunk", data: "hello" }]);
  });

  it("handles multiline data fields", () => {
    const events: Array<{ event: string; data: unknown }> = [];
    const parser = createSSEParser((event) => events.push(event));

    parser.push("event: output\ndata: first\ndata: second\n\n");

    expect(events).toEqual([{ event: "output", data: "first\nsecond" }]);
  });

  it("formats JSON payloads and bounds displayed output", () => {
    expect(formatSSEPayload({ ok: true })).toBe('{"ok":true}');
    expect(appendBoundedLines(["a", "b"], "c\nd", { maxLines: 3, maxBytes: 10 })).toEqual(["b", "c", "d"]);
    expect(appendBoundedLines([], "123456789", { maxLines: 10, maxBytes: 5 })).toEqual(["56789"]);
  });
});
