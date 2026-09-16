import type { ProductVM } from "@sparkops/contracts/product";

export const PRODUCT_STATUS_OPTIONS = [
  { value: "active", label: "启用" },
  { value: "inactive", label: "停用" },
] as const;

const PRODUCT_COLORS = ["#4f46e5", "#0891b2", "#059669", "#d97706", "#db2777", "#7c3aed"];

export function productDisplayName(product: ProductVM): string {
  return product.display_name?.trim() || product.name;
}

export function productStatusLabel(status?: string): string {
  const normalized = (status ?? "active").toLowerCase();
  if (normalized === "inactive") return "停用";
  if (normalized === "active") return "启用";
  return status ?? "启用";
}

export function productStatusTagClass(status?: string): string {
  const normalized = (status ?? "active").toLowerCase();
  if (normalized === "inactive") return "project-tag project-tag-gray";
  return "project-tag project-tag-green";
}

export function productAccentColor(name: string): string {
  let hash = 0;
  for (const char of name) {
    hash = (hash + char.charCodeAt(0)) % PRODUCT_COLORS.length;
  }
  return PRODUCT_COLORS[hash] ?? PRODUCT_COLORS[0];
}

export function labelOf<T extends { value: string; label: string }>(
  options: readonly T[],
  value?: string,
) {
  return options.find((option) => option.value === value)?.label ?? value ?? "-";
}

export { formatDateTime } from "@/modules/workspace/components/project/project-utils";
