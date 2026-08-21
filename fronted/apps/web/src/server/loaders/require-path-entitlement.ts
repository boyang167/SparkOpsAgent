import { redirect } from "next/navigation";
import { isPathEntitled, pathEntitlement } from "@/components/layout/entitlements";
import { loadWorkspaceEntitlements } from "./entitlements";

const SKIP_PATH_PREFIXES = [
  "/dashboard",
  "/admin",
  "/platform",
  "/settings",
  "/notifications",
  "/permission",
  "/cluster",
  "/login",
  "/register",
  "/callback",
];

/** 服务端路由守卫：未授权模块重定向到提示页 */
export async function requirePathEntitlement(pathname: string, workspaceId?: string) {
  if (!workspaceId || !pathname.startsWith("/")) return;

  if (pathname.startsWith("/dashboard/module-unavailable")) return;
  if (SKIP_PATH_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`))) {
    return;
  }

  const moduleKey = pathEntitlement(pathname);
  if (!moduleKey) return;

  const entitlements = await loadWorkspaceEntitlements(workspaceId);
  if (isPathEntitled(pathname, entitlements)) return;

  const params = new URLSearchParams({ module: moduleKey, from: pathname });
  redirect(`/dashboard/module-unavailable?${params.toString()}`);
}
