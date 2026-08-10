import { listWorkspaces } from "@sparkops/bff-modules/tenant";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";
import { loadBffContext } from "./context";

export async function loadWorkspaces(): Promise<WorkspaceVM[]> {
  try {
    const ctx = await loadBffContext();
    return await listWorkspaces(ctx);
  } catch {
    return [];
  }
}
