// Jenkins embeds hidden ConsoleNote annotations as ESC[8mha:////<base64>ESC[0m.
// Some transports drop the ESC byte, leaving "[8mha:////...[0m" in plain text.
const CONSOLE_NOTE_PATTERN = /\x1b?\[8mha:\/{4}[A-Za-z0-9+/=]*\x1b?\[0m/g;
const ANSI_PATTERN = /\x1b\[[0-9;?]*[A-Za-z]/g;
const NOTE_START = "[8mha:";
const MAX_PENDING = 64 * 1024;

export function sanitizeConsoleLog(text: string): string {
  if (!text) return "";
  return text.replace(CONSOLE_NOTE_PATTERN, "").replace(ANSI_PATTERN, "").replace(/\r\n?/g, "\n");
}

/** Index from which `text` may hold an unfinished note / escape sequence that the next chunk completes. */
function unsafeTailStart(text: string): number {
  const noteStart = text.lastIndexOf(NOTE_START);
  if (noteStart !== -1 && text.indexOf("[0m", noteStart + NOTE_START.length) === -1) {
    return noteStart > 0 && text[noteStart - 1] === "\x1b" ? noteStart - 1 : noteStart;
  }
  const escStart = text.lastIndexOf("\x1b");
  if (escStart !== -1 && !/^\x1b\[[0-9;?]*[A-Za-z]/.test(text.slice(escStart))) {
    return escStart;
  }
  for (let len = Math.min(NOTE_START.length - 1, text.length); len > 0; len--) {
    if (NOTE_START.startsWith(text.slice(text.length - len))) return text.length - len;
  }
  if (text.endsWith("\r")) return text.length - 1;
  return text.length;
}

/** Sanitizes a streamed log chunk by chunk, holding back partial sequences split across chunks. */
export function createConsoleLogSanitizer() {
  let pending = "";
  return {
    push(chunk: string): string {
      const text = pending + chunk;
      let cut = unsafeTailStart(text);
      if (text.length - cut > MAX_PENDING) cut = text.length;
      pending = text.slice(cut);
      return sanitizeConsoleLog(text.slice(0, cut));
    },
    flush(): string {
      const rest = sanitizeConsoleLog(pending);
      pending = "";
      return rest;
    },
  };
}

export function countNewlines(text: string): number {
  let count = 0;
  for (let i = text.indexOf("\n"); i !== -1; i = text.indexOf("\n", i + 1)) count++;
  return count;
}

export function countLogLines(text: string): number {
  if (!text) return 0;
  return countNewlines(text) + (text.endsWith("\n") ? 0 : 1);
}

/** Returns the last `maxLines` lines of `text` and how many lines were dropped before them. */
export function tailLines(text: string, maxLines: number): { text: string; hidden: number } {
  const total = countLogLines(text);
  if (total <= maxLines) return { text, hidden: 0 };
  let index = text.endsWith("\n") ? text.length - 1 : text.length;
  for (let seen = 0; seen < maxLines; seen++) {
    index = text.lastIndexOf("\n", index - 1);
    if (index === -1) return { text, hidden: 0 };
  }
  return { text: text.slice(index + 1), hidden: total - maxLines };
}
