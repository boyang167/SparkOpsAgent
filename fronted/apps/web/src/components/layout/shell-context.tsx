"use client";

import { createContext, useContext, useEffect, useState } from "react";
import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";
import { platform } from "@sparkops/web-sdk";
import { DEFAULT_ENTITLEMENTS } from "@/components/layout/entitlements";
import {
  fetchWorkspaceEntitlementsCached,
  invalidateClientEntitlementsCache,
} from "@/components/layout/entitlements-cache";

export interface ShellContextValue {
  workspaceId?: string;
  canManageAdmin: boolean;
  isSuperAdmin: boolean;
  entitlements: WorkspaceEntitlementsVM;
  entitlementsLoading: boolean;
}

const ShellContext = createContext<ShellContextValue>({
  canManageAdmin: false,
  isSuperAdmin: false,
  entitlements: DEFAULT_ENTITLEMENTS,
  entitlementsLoading: false,
});

export function ShellProvider({
  workspaceId,
  canManageAdmin,
  isSuperAdmin,
  children,
}: {
  workspaceId?: string;
  canManageAdmin: boolean;
  isSuperAdmin: boolean;
  children: React.ReactNode;
}) {
  const [entitlements, setEntitlements] = useState<WorkspaceEntitlementsVM>(DEFAULT_ENTITLEMENTS);
  const [entitlementsLoading, setEntitlementsLoading] = useState(false);

  useEffect(() => {
    if (!workspaceId) {
      setEntitlements(DEFAULT_ENTITLEMENTS);
      return;
    }

    let cancelled = false;
    setEntitlementsLoading(true);
    const wsId = workspaceId;
    void fetchWorkspaceEntitlementsCached(wsId, () => platform.getWorkspaceEntitlements(wsId))
      .then((data) => {
        if (!cancelled) setEntitlements(data);
      })
      .catch(() => {
        if (!cancelled) setEntitlements(DEFAULT_ENTITLEMENTS);
      })
      .finally(() => {
        if (!cancelled) setEntitlementsLoading(false);
      });

    function onEntitlementsUpdated() {
      void fetchWorkspaceEntitlementsCached(
        wsId,
        () => platform.getWorkspaceEntitlements(wsId),
        { fresh: true },
      )
        .then((data) => setEntitlements(data))
        .catch(() => setEntitlements(DEFAULT_ENTITLEMENTS));
    }

    window.addEventListener("sparkops:entitlements-updated", onEntitlementsUpdated);
    return () => {
      cancelled = true;
      window.removeEventListener("sparkops:entitlements-updated", onEntitlementsUpdated);
    };
  }, [workspaceId]);

  return (
    <ShellContext.Provider
      value={{
        workspaceId,
        canManageAdmin,
        isSuperAdmin,
        entitlements,
        entitlementsLoading,
      }}
    >
      {children}
    </ShellContext.Provider>
  );
}

export function useShellContext() {
  return useContext(ShellContext);
}

export function notifyEntitlementsUpdated() {
  invalidateClientEntitlementsCache();
  window.dispatchEvent(new Event("sparkops:entitlements-updated"));
}
