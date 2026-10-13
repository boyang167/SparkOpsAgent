import { describe, expect, it } from "vitest";
import { countLogLines, createConsoleLogSanitizer, sanitizeConsoleLog, tailLines } from "./console-log";

describe("sanitizeConsoleLog", () => {
  it("removes Jenkins console notes with and without ESC bytes", () => {
    const withEsc = "Started by user \x1b[8mha:////4HHu3rOh+AAAA==\x1b[0madmin\n";
    const plain = "Started by user [8mha:////4HHu3rOh+AAAA==[0madmin\n";
    expect(sanitizeConsoleLog(withEsc)).toBe("Started by user admin\n");
    expect(sanitizeConsoleLog(plain)).toBe("Started by user admin\n");
  });

  it("strips ANSI colors and normalizes line endings", () => {
    expect(sanitizeConsoleLog("\x1b[32mok\x1b[0m\r\nnext")).toBe("ok\nnext");
  });
});

describe("createConsoleLogSanitizer", () => {
  it("handles notes and escapes split across chunks", () => {
    const sanitizer = createConsoleLogSanitizer();
    const chunks = ["Started by user \x1b[8mha:////4HH", "u3r==\x1b[0", "madmin\n\x1b[3", "2mok\x1b[0m\r", "\nend"];
    const out = chunks.map((chunk) => sanitizer.push(chunk)).join("") + sanitizer.flush();
    expect(out).toBe("Started by user admin\nok\nend");
  });

  it("holds a partial note prefix at the end of a chunk", () => {
    const sanitizer = createConsoleLogSanitizer();
    expect(sanitizer.push("line [8m")).toBe("line ");
    expect(sanitizer.push("ha:////AA==[0mrest")).toBe("rest");
  });
});

describe("line helpers", () => {
  it("counts lines ignoring a trailing newline", () => {
    expect(countLogLines("a\nb\n")).toBe(2);
    expect(countLogLines("a\nb")).toBe(2);
    expect(countLogLines("")).toBe(0);
  });

  it("keeps only the last lines", () => {
    expect(tailLines("1\n2\n3\n4\n", 2)).toEqual({ text: "3\n4\n", hidden: 2 });
    expect(tailLines("1\n2", 5)).toEqual({ text: "1\n2", hidden: 0 });
  });
});
