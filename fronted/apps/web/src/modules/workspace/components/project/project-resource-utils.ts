export const CPU_MARKS = [
  { label: "0.1核", millicores: 100 },
  { label: "0.5核", millicores: 500 },
  { label: "4核", millicores: 4000 },
  { label: "8核", millicores: 8000 },
  { label: "16核", millicores: 16000 },
  { label: "32核", millicores: 32000 },
] as const;

export const EXPAND_CPU_MARKS = [
  { label: "200m", millicores: 200 },
  { label: "500m", millicores: 500 },
  { label: "1", millicores: 1000 },
  { label: "2", millicores: 2000 },
  { label: "4", millicores: 4000 },
  { label: "8", millicores: 8000 },
] as const;

export const EXPAND_MEMORY_MARKS = [
  { label: "64Mi", mebibytes: 64 },
  { label: "128Mi", mebibytes: 128 },
  { label: "256Mi", mebibytes: 256 },
  { label: "512Mi", mebibytes: 512 },
  { label: "1Gi", mebibytes: 1024 },
  { label: "2Gi", mebibytes: 2048 },
  { label: "4Gi", mebibytes: 4096 },
  { label: "6Gi", mebibytes: 6144 },
  { label: "8Gi", mebibytes: 8192 },
  { label: "12Gi", mebibytes: 12288 },
  { label: "16Gi", mebibytes: 16384 },
] as const;

export const MEMORY_MARKS = [
  { label: "128Mi", mebibytes: 128 },
  { label: "512Mi", mebibytes: 512 },
  { label: "2Gi", mebibytes: 2048 },
  { label: "4Gi", mebibytes: 4096 },
  { label: "8Gi", mebibytes: 8192 },
  { label: "16Gi", mebibytes: 16384 },
  { label: "32Gi", mebibytes: 32768 },
] as const;

export function parseCpuToMillicores(raw: string): number {
  const value = raw.trim().toLowerCase().replace(/核$/, "");
  if (!value) return 500;
  if (value.endsWith("m")) {
    const n = Number.parseFloat(value.slice(0, -1));
    return Number.isFinite(n) ? Math.max(100, Math.round(n)) : 500;
  }
  const n = Number.parseFloat(value);
  return Number.isFinite(n) ? Math.max(100, Math.round(n * 1000)) : 500;
}

export function formatCpuMillicores(millicores: number): string {
  const clamped = Math.max(100, Math.min(32000, millicores));
  if (clamped < 1000) return `${clamped}m`;
  if (clamped % 1000 === 0) return String(clamped / 1000);
  return `${clamped}m`;
}

export function parseMemoryToMi(raw: string, fallback = 512): number {
  const value = raw.trim();
  if (!value) return fallback;
  const lower = value.toLowerCase();
  if (lower.endsWith("gi")) {
    const n = Number.parseFloat(lower.slice(0, -2));
    return Number.isFinite(n) ? Math.max(64, Math.round(n * 1024)) : fallback;
  }
  if (lower.endsWith("mi")) {
    const n = Number.parseFloat(lower.slice(0, -2));
    return Number.isFinite(n) ? Math.max(64, Math.round(n)) : fallback;
  }
  const n = Number.parseFloat(lower);
  return Number.isFinite(n) ? Math.max(64, Math.round(n)) : fallback;
}

export function formatMemoryMi(mebibytes: number, minMi = 128): string {
  const clamped = Math.max(minMi, Math.min(32768, mebibytes));
  if (clamped >= 1024 && clamped % 1024 === 0) return `${clamped / 1024}Gi`;
  return `${clamped}Mi`;
}

/** Snap a continuous value onto the nearest mark for smoother dual-slider UX. */
export function snapToNearestMark(value: number, markValues: readonly number[]): number {
  if (markValues.length === 0) return value;
  let best = markValues[0]!;
  let bestDist = Math.abs(value - best);
  for (const mark of markValues) {
    const dist = Math.abs(value - mark);
    if (dist < bestDist) {
      best = mark;
      bestDist = dist;
    }
  }
  return best;
}

export const DEFAULT_GIT_BRANCHES = ["main", "master", "develop", "dev", "release", "feature/xxx"];

export const DOCKERFILE_SUGGESTIONS = ["./Dockerfile", "./docker/Dockerfile", "./build/Dockerfile"];
