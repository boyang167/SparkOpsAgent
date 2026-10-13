"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { MeVM } from "@sparkops/contracts/identity";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { RouteVisitRecorder } from "@/components/layout/RouteVisitRecorder";
import { SidebarWorkspaceSwitcher } from "@/components/layout/SidebarWorkspaceSwitcher";
import { RefreshBoundary, ShellProvider } from "@/components/layout/shell-context";
import { PlatformShell } from "@/components/layout/PlatformShell";
import { TopBarTools } from "@/components/layout/TopBarTools";
import type { ConsoleKind } from "@/components/layout/nav-tree";
import { resolveConsoleKind } from "@/components/layout/nav-tree";
import { NotificationBell } from "@/modules/notification/components/NotificationBell";

interface ProjectOption {
  id: string;
  name: string;
}

interface AppShellProps {
  children: React.ReactNode;
  nav: React.ReactNode;
  session: MeVM | null;
  organizations: OrganizationVM[];
  workspaces: WorkspaceVM[];
  workspaceId?: string;
  projects?: ProjectOption[];
  currentProjectId?: string;
  isSuperAdmin: boolean;
  canManageAdmin: boolean;
  topbarHint?: string;
}

function userInitial(email: string) {
  return email.charAt(0).toUpperCase();
}

const TOPBAR_HINTS: Record<ConsoleKind, string> = {
  business: "Agent 运营平台",
  admin: "管理控制台",
  platform: "平台运营中心",
};

export function AppShell({
  children,
  nav,
  session,
  organizations,
  workspaces,
  workspaceId,
  projects,
  currentProjectId,
  isSuperAdmin,
  canManageAdmin,
  topbarHint,
}: AppShellProps) {
  const pathname = usePathname();
  const consoleKind = resolveConsoleKind(pathname);
  const shellClass =
    consoleKind === "platform"
      ? "app-shell app-shell-platform"
      : consoleKind === "admin"
        ? "app-shell app-shell-admin"
        : "app-shell";

  return (
    <PlatformShell>
      <ShellProvider
        workspaceId={workspaceId}
        canManageAdmin={canManageAdmin}
        isSuperAdmin={isSuperAdmin}
      >
        <RouteVisitRecorder />
        <div className={shellClass}>
        <aside className="sidebar">
          <div className="sidebar-brand">
            <Link href="/dashboard/workplace" className="sidebar-brand-mark">
              <span className="sidebar-logo">S</span>
              SparkOps
            </Link>
          </div>

          <nav className="sidebar-nav">{nav}</nav>

          <div className="sidebar-footer">
            {session && workspaces.length > 0 ? (
              <SidebarWorkspaceSwitcher
                organizations={organizations}
                workspaces={workspaces}
                currentWorkspaceId={workspaceId}
              />
            ) : session ? (
              <Link href="/settings/tenant" className="text-sm text-[var(--accent)] hover:underline px-1">
                创建组织与工作区
              </Link>
            ) : null}
          </div>
        </aside>

        <div className="app-main">
          <header className="topbar">
            <div className="topbar-leading" />

            <div className="topbar-actions">
              <TopBarTools />

              {session ? (
                <>
                  <span className="topbar-divider" aria-hidden />
                  <NotificationBell workspaceId={workspaceId} />
                  <Link
                    href="/settings/profile"
                    className="user-pill hover:opacity-90 transition-opacity"
                    title={`${session.display_name} · ${session.email}`}
                    aria-label={session.display_name}
                  >
                    <span className="user-avatar">{userInitial(session.email)}</span>
                  </Link>
                </>
              ) : null}
            </div>
          </header>

          <main className="page-content">
            <div className="page-canvas">
              <RefreshBoundary>{children}</RefreshBoundary>
            </div>
          </main>
        </div>
        </div>
      </ShellProvider>
    </PlatformShell>
  );
}
