export type SSEEvent = {
  event: string;
  data: unknown;
};

type BoundedOptions = {
  maxLines: number;
  maxBytes: number;
};

export function createSSEParser(onEvent: (event: SSEEvent) => void) {
  let buffer = "";

  function push(chunk: string) {
    buffer += chunk.replace(/\r\n/g, "\n");
    let boundary = buffer.indexOf("\n\n");
    while (boundary >= 0) {
      const frame = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      emitFrame(frame, onEvent);
      boundary = buffer.indexOf("\n\n");
    }
  }

  function flush() {
    if (!buffer) return;
    emitFrame(buffer, onEvent);
    buffer = "";
  }

  return { push, flush };
}

export function formatSSEPayload(value: unknown): string {
  if (typeof value === "string") return value;
  return JSON.stringify(value);
}

export function appendBoundedLines(current: string[], next: string, options: BoundedOptions): string[] {
  const lines = [...current, ...next.split("\n")].filter((line) => line.length > 0);
  let bounded = lines.slice(-options.maxLines);
  while (byteLength(bounded.join("\n")) > options.maxBytes && bounded.length > 0) {
    if (bounded.length > 1) {
      bounded = bounded.slice(1);
      continue;
    }
    bounded[0] = trimToLastBytes(bounded[0], options.maxBytes);
  }
  return bounded;
}

function emitFrame(frame: string, onEvent: (event: SSEEvent) => void) {
  let event = "message";
  const data: string[] = [];
  for (const line of frame.split("\n")) {
    if (line.startsWith("event:")) {
      event = line.slice("event:".length).trim() || "message";
    } else if (line.startsWith("data:")) {
      data.push(line.slice("data:".length).trimStart());
    }
  }
  if (data.length === 0) return;
  onEvent({ event, data: decodeSSEData(data.join("\n")) });
}

function decodeSSEData(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}

function byteLength(value: string): number {
  return new TextEncoder().encode(value).length;
}

function trimToLastBytes(value: string, maxBytes: number): string {
  let current = value;
  while (byteLength(current) > maxBytes && current.length > 0) {
    current = current.slice(1);
  }
  return current;
}
