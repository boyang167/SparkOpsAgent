export function slugify(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function isValidSlug(slug: string): boolean {
  return /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug);
}

export function slugPreview(slug: string, kind: "org" | "workspace"): string {
  if (!slug) return "—";
  return kind === "org" ? `/orgs/${slug}` : `/workspaces/${slug}`;
}
