"use client";

import type { MyMenuButtonItemVM, MyMenuButtonsVM } from "@sparkops/contracts/governance";
import { governance } from "@sparkops/web-sdk";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";

const CACHE_TTL_MS = 30_000;

type CacheEntry = {
  expiresAt: number;
  data: MyMenuButtonsVM;
};

const buttonCache = new Map<string, CacheEntry>();
const inflight = new Map<string, Promise<MyMenuButtonsVM>>();

function normalizePath(path: string): string {
  let value = path.trim();
  if (!value) return "";
  if (!value.startsWith("/")) value = `/${value}`;
  while (value.length > 1 && value.endsWith("/")) {
    value = value.slice(0, -1);
  }
  return value;
}

function cacheKey(workspaceId: string, path: string) {
  return `${workspaceId}:${path}`;
}

async function fetchMenuButtons(workspaceId: string, path: string): Promise<MyMenuButtonsVM> {
  const normalized = normalizePath(path);
  const key = cacheKey(workspaceId, normalized);
  const now = Date.now();
  const cached = buttonCache.get(key);
  if (cached && cached.expiresAt > now) {
    return cached.data;
  }

  const pending = inflight.get(key);
  if (pending) {
    return pending;
  }

  const request = governance
    .myMenuButtons(workspaceId, normalized)
    .then((data) => {
      buttonCache.set(key, { data, expiresAt: Date.now() + CACHE_TTL_MS });
      inflight.delete(key);
      return data;
    })
    .catch((err) => {
      inflight.delete(key);
      throw err;
    });

  inflight.set(key, request);
  return request;
}

export interface UseMenuButtonResult {
  loading: boolean;
  error: string | null;
  configured: boolean;
  menuId?: string;
  menuPath: string;
  buttons: MyMenuButtonItemVM[];
  can: (code: string) => boolean;
  canAny: (codes: string[]) => boolean;
  canAll: (codes: string[]) => boolean;
  refresh: () => void;
}

/**
 * 按菜单路由解析当前用户可用的按钮权限。
 *
 * @param workspaceId 当前工作区 ID（页面层传入）
 * @param menuPath 菜单 path，默认取当前 pathname（支持详情页前缀匹配）
 */
export function useMenuButton(workspaceId: string, menuPath?: string): UseMenuButtonResult {
  const pathname = usePathname();
  const resolvedPath = normalizePath(menuPath ?? pathname ?? "");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<MyMenuButtonsVM | null>(null);
  const [refreshCounter, setRefreshCounter] = useState(0);

  const refresh = useCallback(() => {
    buttonCache.delete(cacheKey(workspaceId, resolvedPath));
    setRefreshCounter((value) => value + 1);
  }, [workspaceId, resolvedPath]);

  useEffect(() => {
    if (!workspaceId || !resolvedPath) {
      setLoading(false);
      setData(null);
      setError(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);

    void fetchMenuButtons(workspaceId, resolvedPath)
      .then((result) => {
        if (cancelled) return;
        setData(result);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setData(null);
        setError(err instanceof Error ? err.message : "加载按钮权限失败");
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [workspaceId, resolvedPath, refreshCounter]);

  const allowedCodes = useMemo(() => {
    return new Set((data?.items ?? []).map((item) => item.code));
  }, [data?.items]);

  const configured = data?.configured ?? false;

  const can = useCallback(
    (code: string) => {
      if (loading) return false;
      if (!configured) return true;
      return allowedCodes.has(code);
    },
    [allowedCodes, configured, loading],
  );

  const canAny = useCallback(
    (codes: string[]) => codes.some((code) => can(code)),
    [can],
  );

  const canAll = useCallback(
    (codes: string[]) => codes.every((code) => can(code)),
    [can],
  );

  return {
    loading,
    error,
    configured,
    menuId: data?.menu_id,
    menuPath: data?.path ?? resolvedPath,
    buttons: data?.items ?? [],
    can,
    canAny,
    canAll,
    refresh,
  };
}
