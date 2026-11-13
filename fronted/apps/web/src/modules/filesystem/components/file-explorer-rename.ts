/** Rename by node ID so file identity stays stable (path is display-only). */
export async function renameNodeKeepingIdentity(
  nodeId: string,
  nextName: string,
  rename: (nodeId: string, name: string) => Promise<void>,
): Promise<void> {
  const trimmed = nextName.trim();
  if (!trimmed) {
    throw new Error("name required");
  }
  await rename(nodeId, trimmed);
}
