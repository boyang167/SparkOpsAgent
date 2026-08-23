"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { MeVM } from "@sparkops/contracts/identity";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { ConsoleSwitcher } from "@/components/layout/ConsoleSwitcher";
import { AdminContextHint } from "@/components/layout/AdminContextHint";
import { ContextBar } from "@/components/layout/ContextBar";
import { RouteVisitRecorder } from "@/components/layout/RouteVisitRecorder";
import { ShellProvider } from "@/components/layout/shell-context";
import { PlatformShell } from "@/components/layout/PlatformShell";
import { TopBarTools } from "@/components/layout/TopBarTools";
import type { ConsoleKind } from "@/components/layout/nav-tree";
import { LogoutButton } from "@/modules/identity/components/LogoutButton";
import { resolveConsoleKind } from "@/components/layout/nav-tree";

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
            {session && workspaces.length === 0 ? (
              <Link href="/settings/tenant" className="text-sm text-[var(--accent)] hover:underline px-1">
                创建组织与工作区
              </Link>
            ) : null}
          </div>
        </aside>

        <div className="app-main">
          <header className="topbar">
            <div className="topbar-leading">
              <ConsoleSwitcher isSuperAdmin={isSuperAdmin} canManageAdmin={canManageAdmin} />
              {session && workspaces.length > 0 ? (
                consoleKind === "business" ? (
                  <ContextBar
                    organizations={organizations}
                    workspaces={workspaces}
                    currentWorkspaceId={workspaceId}
                    projects={projects}
                    currentProjectId={currentProjectId}
                  />
                ) : (
                  <AdminContextHint
                    consoleKind={consoleKind}
                    workspaces={workspaces}
                    currentWorkspaceId={workspaceId}
                  />
                )
              ) : (
                <span className="topbar-hint hidden sm:inline">{topbarHint ?? TOPBAR_HINTS[consoleKind]}</span>
              )}
            </div>

            <div className="topbar-actions">
              <TopBarTools />

              {session ? <span className="topbar-divider" aria-hidden /> : null}

              {session ? (
                <>
                  <Link
                    href="/notifications"
                    className="topbar-tool-btn"
                    title="消息中心"
                    aria-label="消息中心"
                  >
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
                      <path d="M18 8a6 6 0 10-12 0c0 7-3 9-3 9h18s-3-2-3-9" strokeLinecap="round" />
                      <path d="M13.7 21a2 2 0 01-3.4 0" strokeLinecap="round" />
                    </svg>
                  </Link>
                  <Link href="/settings/profile" className="user-pill hover:opacity-90 transition-opacity">
                    <span className="user-avatar">{userInitial(session.email)}</span>
                    <div className="min-w-0 hidden sm:block">
                      <p className="text-sm font-medium truncate">{session.display_name}</p>
                      <p className="text-xs text-muted truncate">{session.email}</p>
                    </div>
                  </Link>
                </>
              ) : null}

              {session ? <LogoutButton /> : null}
            </div>
          </header>

          <main className="page-content">{children}</main>
        </div>
        </div>
      </ShellProvider>
    </PlatformShell>
  );
}
