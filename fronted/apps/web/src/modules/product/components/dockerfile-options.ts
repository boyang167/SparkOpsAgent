export function isDockerfileName(name: string): boolean {
  const base = name.trim().split("/").pop() ?? "";
  return /^dockerfile(\.|$)/i.test(base);
}

export function toRepoRelativePath(path: string): string {
  const normalized = path.replace(/\\/g, "/").replace(/^\.\//, "");
  return normalized ? `./${normalized}` : "./Dockerfile";
}
