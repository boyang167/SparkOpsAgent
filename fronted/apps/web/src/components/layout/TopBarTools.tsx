"use client";

import { useEffect, useState } from "react";
import { CommandSearchModal } from "./CommandSearchModal";
import { SystemConfigDrawer } from "./SystemConfigDrawer";
import { useShellContext } from "./shell-context";
import { useThemeConfig, useResolvedThemeMode } from "./theme/ThemeProvider";

export function TopBarTools() {
  const { refreshPage, refreshing } = useShellContext();
  const { toggleDarkLight } = useThemeConfig();
  const resolvedMode = useResolvedThemeMode();
  const [searchOpen, setSearchOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setSearchOpen(true);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  const isDark = resolvedMode === "dark";

  return (
    <>
      <div className="topbar-tools">
        <button
          type="button"
          className="topbar-tool-btn"
          aria-label="搜索"
          title="命令面板 (Ctrl+K)"
          onClick={() => setSearchOpen(true)}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <circle cx="11" cy="11" r="7" />
            <path d="M20 20l-3-3" strokeLinecap="round" />
          </svg>
        </button>

        <button
          type="button"
          className="topbar-tool-btn"
          aria-label="界面偏好"
          title="界面偏好"
          onClick={() => setSettingsOpen(true)}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <path d="M4 7h10M18 7h2M4 17h4M12 17h8" strokeLinecap="round" />
            <circle cx="16" cy="7" r="2" />
            <circle cx="10" cy="17" r="2" />
          </svg>
        </button>

        <button
          type="button"
          className={`topbar-tool-btn${refreshing ? " topbar-tool-btn-spinning" : ""}`}
          aria-label="刷新"
          aria-busy={refreshing}
          title="刷新页面"
          onClick={refreshPage}
          disabled={refreshing}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <path d="M21 12a9 9 0 1 1-2.64-6.36" strokeLinecap="round" />
            <path d="M21 3v6h-6" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>

        <button
          type="button"
          className="topbar-tool-btn"
          aria-label={isDark ? "切换浅色" : "切换深色"}
          title={isDark ? "切换浅色" : "切换深色"}
          onClick={toggleDarkLight}
        >
          {isDark ? (
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
              <circle cx="12" cy="12" r="4" />
              <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" strokeLinecap="round" />
            </svg>
          ) : (
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
              <path d="M21 14.5A8.5 8.5 0 1 1 9.5 3a6.5 6.5 0 0 0 11.5 11.5z" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          )}
        </button>
      </div>

      <CommandSearchModal open={searchOpen} onClose={() => setSearchOpen(false)} />
      <SystemConfigDrawer open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </>
  );
}
