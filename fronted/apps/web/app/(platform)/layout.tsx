import Link from "next/link";

import { PlatformNav } from "@/components/layout/PlatformNav";
import { LogoutButton } from "@/modules/identity/components/LogoutButton";
import { WorkspaceSwitcher } from "@/modules/tenant/components/WorkspaceSwitcher";
import { loadSession } from "@/server/loaders/session";
import { loadResolvedWorkspaceId } from "@/server/loaders/workspace-context";



function userInitial(email: string) {

  return email.charAt(0).toUpperCase();

}



export default async function PlatformLayout({ children }: { children: React.ReactNode }) {

  const session = await loadSession();

  let workspaces: Awaited<ReturnType<typeof loadResolvedWorkspaceId>>["workspaces"] = [];
  let workspaceId: string | undefined;

  if (session) {
    try {
      const resolved = await loadResolvedWorkspaceId();
      workspaces = resolved.workspaces;
      workspaceId = resolved.workspaceId;
    } catch {
      // ignore
    }
  }



  return (

    <div className="app-shell">

      <aside className="sidebar">

        <div className="sidebar-brand">

          <Link href="/dashboard" className="sidebar-brand-mark">

            <span className="sidebar-logo">S</span>

            SparkOps

          </Link>

        </div>

        <nav className="sidebar-nav">

          <PlatformNav workspaceId={workspaceId} />

        </nav>

        <div className="sidebar-footer">

          {session && workspaces.length > 0 ? (

            <div className="space-y-2">

              <p className="text-xs font-medium text-[var(--muted)] px-1">Workspace</p>

              <WorkspaceSwitcher workspaces={workspaces} currentWorkspaceId={workspaceId} />

            </div>

          ) : session ? (

            <Link href="/settings/tenant" className="text-sm text-[var(--accent)] hover:underline px-1">

              创建组织与工作区

            </Link>

          ) : null}

        </div>

      </aside>



      <div className="app-main">

        <header className="topbar">

          <div className="text-sm text-muted hidden sm:block">Agent 运营平台</div>

          <div className="flex items-center gap-3">
            {session ? (
              <Link href="/settings/profile" className="user-pill hover:opacity-90 transition-opacity">
                <span className="user-avatar">{userInitial(session.email)}</span>
                <div className="min-w-0 hidden sm:block">
                  <p className="text-sm font-medium truncate">{session.display_name}</p>
                  <p className="text-xs text-muted truncate">{session.email}</p>
                </div>
              </Link>
            ) : null}

            {session ? <LogoutButton /> : null}

          </div>

        </header>

        <main className="page-content">{children}</main>

      </div>

    </div>

  );

}

