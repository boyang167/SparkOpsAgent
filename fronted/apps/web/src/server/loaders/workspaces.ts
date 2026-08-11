import { cache } from "react";
import { listWorkspaces } from "@sparkops/bff-modules/tenant";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";
import { loadBffContext } from "./context";

export const loadWorkspaces = cache(async (): Promise<WorkspaceVM[]> => {
  try {
    const ctx = await loadBffContext();
    return await listWorkspaces(ctx);
  } catch {
    return [];
  }
});
