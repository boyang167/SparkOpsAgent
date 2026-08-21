"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { tenant } from "@sparkops/web-sdk";
import type { CommandSearchItem } from "@/components/layout/command-search-index";
import {
  buildStaticCommandItems,
  rankCommandItems,
} from "@/components/layout/command-search-index";
import { buildDynamicCommandItems } from "@/components/layout/command-search-dynamic";
import { recentVisitsToSearchItems } from "@/components/layout/command-search-recent";
import { readWorkspaceIdFromCookie } from "@/components/layout/workspace-cookie";
import { isPathEntitled } from "@/components/layout/entitlements";
import { useShellContext } from "@/components/layout/shell-context";

interface UseCommandSearchOptions {
  open: boolean;
  workspaceId?: string;
  canManageAdmin: boolean;
  isSuperAdmin: boolean;
}

export function useCommandSearch({
  open,
  workspaceId,
  canManageAdmin,
  isSuperAdmin,
}: UseCommandSearchOptions) {
  const { entitlements } = useShellContext();
  const [keyword, setKeyword] = useState("");
  const [dynamicItems, setDynamicItems] = useState<CommandSearchItem[]>([]);
  const [loadingDynamic, setLoadingDynamic] = useState(false);

  const staticItems = useMemo(
    () => buildStaticCommandItems({ canManageAdmin, isSuperAdmin, entitlements }),
    [canManageAdmin, isSuperAdmin, entitlements],
  );

  const loadDynamicItems = useCallback(async () => {
    setLoadingDynamic(true);
    try {
      let wsId = workspaceId ?? readWorkspaceIdFromCookie();
      if (!wsId) {
        const workspaces = await tenant.listWorkspaces();
        wsId = workspaces[0]?.id;
      }
      if (!wsId) {
        setDynamicItems([]);
        return;
      }

      const items = await buildDynamicCommandItems({
        workspaceId: wsId,
        isSuperAdmin,
        entitlements,
      });
      setDynamicItems(items);
    } finally {
      setLoadingDynamic(false);
    }
  }, [workspaceId, isSuperAdmin, entitlements]);

  useEffect(() => {
    if (!open) return;
    setKeyword("");
    void loadDynamicItems();
  }, [open, loadDynamicItems]);

  const allItems = useMemo(() => {
    const recent = recentVisitsToSearchItems().filter((item) =>
      isPathEntitled(item.href, entitlements),
    );
    return [...recent, ...staticItems, ...dynamicItems];
  }, [staticItems, dynamicItems, entitlements]);

  const results = useMemo(
    () => rankCommandItems(allItems, keyword, keyword.trim() ? 24 : 16),
    [allItems, keyword],
  );

  return {
    keyword,
    setKeyword,
    results,
    loadingDynamic,
  };
}
