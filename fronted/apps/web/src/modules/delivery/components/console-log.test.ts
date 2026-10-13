import { describe, expect, it } from "vitest";
import { countLogLines, sanitizeConsoleLog } from "./console-log";

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

describe("countLogLines", () => {
  it("ignores a trailing newline", () => {
    expect(countLogLines("a\nb\n")).toBe(2);
    expect(countLogLines("")).toBe(0);
  });
});
