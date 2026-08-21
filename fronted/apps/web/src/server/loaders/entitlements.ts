import { cache } from "react";
import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";
import { getWorkspaceEntitlementsCached } from "@sparkops/bff-modules/platform";
import { DEFAULT_ENTITLEMENTS } from "@/components/layout/entitlements";
import { loadBffContext } from "./context";

export const loadWorkspaceEntitlements = cache(
  async (workspaceId: string): Promise<WorkspaceEntitlementsVM> => {
    try {
      const ctx = await loadBffContext(workspaceId);
      return await getWorkspaceEntitlementsCached(ctx, workspaceId);
    } catch {
      return DEFAULT_ENTITLEMENTS;
    }
  },
);
