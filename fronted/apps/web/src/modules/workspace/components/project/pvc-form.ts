const NAME_PATTERN = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;
const AMOUNT_PATTERN = /^[0-9]+(\.[0-9]+)?$/;

export const ACCESS_MODES = ["ReadWriteOnce", "ReadOnlyMany", "ReadWriteMany"] as const;
export const STORAGE_UNITS = ["Mi", "Gi", "Ti"] as const;

export type StorageUnit = (typeof STORAGE_UNITS)[number];
export type AccessMode = (typeof ACCESS_MODES)[number];

export function isValidPvcName(name: string): boolean {
  return NAME_PATTERN.test(name) && name.length <= 253;
}

export function isValidMountPath(path: string): boolean {
  return path.startsWith("/") && path.length > 1 && !path.includes(" ");
}

export function composeStorage(amount: string, unit: StorageUnit): { storage: string; error: string | null } {
  const value = amount.trim();
  if (!AMOUNT_PATTERN.test(value) || Number(value) <= 0) {
    return { storage: "", error: "容量需为正数，例如 1 或 512" };
  }
  return { storage: `${value}${unit}`, error: null };
}
