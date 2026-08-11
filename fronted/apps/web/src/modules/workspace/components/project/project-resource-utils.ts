export const CPU_MARKS = [
  { label: "0.01核", millicores: 10 },
  { label: "2核", millicores: 2000 },
  { label: "4核", millicores: 4000 },
  { label: "8核", millicores: 8000 },
  { label: "16核", millicores: 16000 },
  { label: "32核", millicores: 32000 },
] as const;

export const MEMORY_MARKS = [
  { label: "1Mi", mebibytes: 1 },
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
    return Number.isFinite(n) ? Math.max(10, Math.round(n)) : 500;
  }
  const n = Number.parseFloat(value);
  return Number.isFinite(n) ? Math.max(10, Math.round(n * 1000)) : 500;
}

export function formatCpuMillicores(millicores: number): string {
  const clamped = Math.max(10, Math.min(32000, millicores));
  if (clamped < 1000) return `${clamped}m`;
  if (clamped % 1000 === 0) return String(clamped / 1000);
  return `${clamped}m`;
}

export function parseMemoryToMi(raw: string): number {
  const value = raw.trim();
  if (!value) return 512;
  const lower = value.toLowerCase();
  if (lower.endsWith("gi")) {
    const n = Number.parseFloat(lower.slice(0, -2));
    return Number.isFinite(n) ? Math.max(1, Math.round(n * 1024)) : 512;
  }
  if (lower.endsWith("mi")) {
    const n = Number.parseFloat(lower.slice(0, -2));
    return Number.isFinite(n) ? Math.max(1, Math.round(n)) : 512;
  }
  const n = Number.parseFloat(lower);
  return Number.isFinite(n) ? Math.max(1, Math.round(n)) : 512;
}

export function formatMemoryMi(mebibytes: number): string {
  const clamped = Math.max(1, Math.min(32768, mebibytes));
  if (clamped >= 1024 && clamped % 1024 === 0) return `${clamped / 1024}Gi`;
  return `${clamped}Mi`;
}

export const DEFAULT_GIT_BRANCHES = ["main", "master", "develop", "dev", "release"];

export const DOCKERFILE_SUGGESTIONS = ["./Dockerfile", "./docker/Dockerfile", "./build/Dockerfile"];
