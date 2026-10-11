"use client";

import {
  createContext,
  Fragment,
  useCallback,
  useContext,
  useEffect,
  useState,
  useTransition,
} from "react";
import { useRouter } from "next/navigation";
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
  refreshKey: number;
  refreshing: boolean;
  refreshPage: () => void;
}

const ShellContext = createContext<ShellContextValue>({
  canManageAdmin: false,
  isSuperAdmin: false,
  entitlements: DEFAULT_ENTITLEMENTS,
  entitlementsLoading: false,
  refreshKey: 0,
  refreshing: false,
  refreshPage: () => {},
});

const MIN_REFRESH_FEEDBACK_MS = 600;

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
  const router = useRouter();
  const [refreshKey, setRefreshKey] = useState(0);
  const [isPending, startTransition] = useTransition();
  const [feedbackActive, setFeedbackActive] = useState(false);

  const refreshPage = useCallback(() => {
    setFeedbackActive(true);
    window.setTimeout(() => setFeedbackActive(false), MIN_REFRESH_FEEDBACK_MS);
    startTransition(() => {
      router.refresh();
      setRefreshKey((key) => key + 1);
    });
    if (workspaceId) notifyEntitlementsUpdated();
  }, [router, workspaceId]);

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
        refreshKey,
        refreshing: isPending || feedbackActive,
        refreshPage,
      }}
    >
      {children}
    </ShellContext.Provider>
  );
}

export function useShellContext() {
  return useContext(ShellContext);
}

/** Remounts page content on refresh so client panels re-run their data fetching. */
export function RefreshBoundary({ children }: { children: React.ReactNode }) {
  const { refreshKey } = useShellContext();
  return <Fragment key={refreshKey}>{children}</Fragment>;
}

export function notifyEntitlementsUpdated() {
  invalidateClientEntitlementsCache();
  window.dispatchEvent(new Event("sparkops:entitlements-updated"));
}
