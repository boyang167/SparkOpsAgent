"use client";

import type { ReactNode } from "react";
import { useMenuButton } from "@/hooks/useMenuButton";

export interface MenuButtonGuardProps {
  workspaceId: string;
  /** 按钮编码，与菜单管理里配置的 code 一致 */
  code: string;
  /** 可选：显式指定菜单 path；默认使用当前 pathname */
  menuPath?: string;
  /** 无权限或加载中时渲染的内容，默认 null */
  fallback?: ReactNode;
  /** 加载中是否隐藏子节点，默认 true */
  hideWhileLoading?: boolean;
  children: ReactNode;
}

export function MenuButtonGuard({
  workspaceId,
  code,
  menuPath,
  fallback = null,
  hideWhileLoading = true,
  children,
}: MenuButtonGuardProps) {
  const { can, loading } = useMenuButton(workspaceId, menuPath);

  if (loading && hideWhileLoading) {
    return null;
  }
  if (!can(code)) {
    return <>{fallback}</>;
  }
  return <>{children}</>;
}
