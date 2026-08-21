export function extractProjectId(pathname: string): string | null {
  const match = pathname.match(/^\/projects\/([^/]+)/);
  if (!match) return null;
  const projectId = match[1];
  if (projectId === "create") return null;
  return projectId;
}

export function isProjectRoute(pathname: string): boolean {
  return extractProjectId(pathname) !== null;
}
