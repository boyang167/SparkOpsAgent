// Jenkins embeds hidden ConsoleNote annotations as ESC[8mha:////<base64>ESC[0m.
// Some transports drop the ESC byte, leaving "[8mha:////...[0m" in plain text.
const CONSOLE_NOTE_PATTERN = /\x1b?\[8mha:\/{4}[A-Za-z0-9+/=]*\x1b?\[0m/g;
const ANSI_PATTERN = /\x1b\[[0-9;?]*[A-Za-z]/g;

export function sanitizeConsoleLog(text: string): string {
  if (!text) return "";
  return text.replace(CONSOLE_NOTE_PATTERN, "").replace(ANSI_PATTERN, "").replace(/\r\n?/g, "\n");
}

export function countLogLines(text: string): number {
  if (!text) return 0;
  const trimmed = text.endsWith("\n") ? text.slice(0, -1) : text;
  return trimmed.split("\n").length;
}
