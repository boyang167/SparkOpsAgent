import { truncateClusterId } from "./cluster-view";

export type RecordItem = {
  id: string;
  clusterId: string;
  kind: string;
  startedAt: string;
  endedAt?: string;
  objectPath?: string;
};

export type ParsedRecordTarget = {
  namespace: string;
  pod: string;
  container: string;
};

export type RecordFrame = {
  time: number;
  stream: string;
  data: string;
};

export const RECORDS_TABLE_COL_SPAN = 8;

export function resolveClusterLabel(clusterId: string, namesById: ReadonlyMap<string, string>): { label: string; title: string } {
  const id = clusterId.trim();
  if (!id) return { label: "-", title: "" };
  const name = namesById.get(id);
  if (name) return { label: name, title: id };
  return { label: truncateClusterId(id), title: id };
}

export function parseRecordObjectPath(objectPath?: string): ParsedRecordTarget {
  const empty = { namespace: "-", pod: "-", container: "-" };
  if (!objectPath?.trim()) return empty;
  const parts = objectPath.split("/").filter(Boolean);
  let namespace = "-";
  let pod = "-";
  let container = "-";
  for (let index = 0; index < parts.length; index += 1) {
    if (parts[index] === "namespaces" && parts[index + 1]) namespace = parts[index + 1];
    if (parts[index] === "pods" && parts[index + 1]) pod = parts[index + 1];
    if (parts[index] === "containers" && parts[index + 1]) container = parts[index + 1];
  }
  return { namespace, pod, container };
}

export function formatRecordTime(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

export function shortRecordId(id: string): string {
  if (id.length <= 8) return id;
  return id.slice(0, 8);
}

export function parseRecordFrames(text: string): RecordFrame[] {
  const frames: RecordFrame[] = [];
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    try {
      const frame = JSON.parse(trimmed) as RecordFrame;
      if (typeof frame.time === "number" && typeof frame.data === "string") {
        frames.push(frame);
      }
    } catch {
      // ignore malformed lines
    }
  }
  return frames;
}

const OUTPUT_STREAMS = new Set(["stdout", "stderr"]);
const DEFAULT_COALESCE_GAP_SEC = 0.2;
const DEFAULT_COALESCE_MAX_CHARS = 262144;

/** Merge consecutive output chunks; ignores stream alternation within the same window. */
export function coalesceRecordFrames(
  frames: RecordFrame[],
  maxGapSec = DEFAULT_COALESCE_GAP_SEC,
  maxChars = DEFAULT_COALESCE_MAX_CHARS,
): RecordFrame[] {
  if (frames.length === 0) return [];
  const merged: RecordFrame[] = [];
  let buffer: RecordFrame | null = null;

  const flush = () => {
    if (!buffer) return;
    merged.push(buffer);
    buffer = null;
  };

  for (const frame of frames) {
    if (frame.stream === "truncated") continue;
    if (!OUTPUT_STREAMS.has(frame.stream)) {
      flush();
      merged.push({ ...frame });
      continue;
    }
    if (
      buffer &&
      OUTPUT_STREAMS.has(buffer.stream) &&
      (frame.time === buffer.time || frame.time - buffer.time <= maxGapSec) &&
      buffer.data.length + frame.data.length <= maxChars
    ) {
      buffer.data += frame.data;
      continue;
    }
    flush();
    buffer = { time: frame.time, stream: "stdout", data: frame.data };
  }
  flush();
  return merged;
}

/** Bucket output into fixed FPS timeline for smooth replay. */
export function bucketRecordFrames(frames: RecordFrame[], fps = 5): RecordFrame[] {
  if (frames.length === 0) return [];
  const interval = 1 / fps;
  const buckets = new Map<number, string>();
  for (const frame of frames) {
    if (!OUTPUT_STREAMS.has(frame.stream)) continue;
    const bucketTime = Math.floor(frame.time / interval) * interval;
    buckets.set(bucketTime, (buckets.get(bucketTime) ?? "") + frame.data);
  }
  return Array.from(buckets.entries())
    .sort((a, b) => a[0] - b[0])
    .map(([time, data]) => ({ time, stream: "stdout", data }));
}

export function inferTerminalSize(frames: RecordFrame[]): { width: number; height: number } {
  const resizePattern = /\x1b\[8;(\d+);(\d+)t/g;
  let width = 120;
  let height = 30;
  for (const frame of frames) {
    resizePattern.lastIndex = 0;
    let match = resizePattern.exec(frame.data);
    while (match) {
      height = Number(match[1]);
      width = Number(match[2]);
      match = resizePattern.exec(frame.data);
    }
  }
  return {
    width: Number.isFinite(width) && width >= 40 ? width : 120,
    height: Number.isFinite(height) && height >= 8 ? height : 30,
  };
}

export function ndjsonToAsciicastV2(text: string, width = 120, height = 30): string {
  const frames = coalesceRecordFrames(parseRecordFrames(text));
  const size = frames.length ? inferTerminalSize(frames) : { width, height };
  const events: string[] = [];
  let lastTime = -1;
  for (const frame of frames) {
    if (!OUTPUT_STREAMS.has(frame.stream)) continue;
    const time = frame.time <= lastTime ? lastTime + 0.001 : frame.time;
    lastTime = time;
    events.push(JSON.stringify([time, "o", frame.data]));
  }
  return `${JSON.stringify({ version: 2, width: size.width, height: size.height })}\n${events.join("\n")}`;
}
