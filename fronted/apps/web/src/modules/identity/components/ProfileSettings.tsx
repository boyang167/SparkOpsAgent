"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { identity } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { MeVM } from "@sparkops/contracts/identity";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { buildConsoleOptions, resolveConsoleKind } from "@/components/layout/nav-tree";

interface ProfileSettingsProps {
  session: MeVM;
  isSuperAdmin: boolean;
  canManageAdmin: boolean;
}

function initialsOf(name: string, email: string) {
  const source = name.trim() || email.trim();
  return source ? source.charAt(0).toUpperCase() : "?";
}

export function ProfileSettings({
  session,
  isSuperAdmin,
  canManageAdmin,
}: ProfileSettingsProps) {
  const router = useRouter();
  const pathname = usePathname();
  const currentConsole = resolveConsoleKind(pathname);
  const consoleOptions = buildConsoleOptions(isSuperAdmin, canManageAdmin);

  const [displayName, setDisplayName] = useState(session.display_name);
  const [profileLoading, setProfileLoading] = useState(false);
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileSuccess, setProfileSuccess] = useState<string | null>(null);

  const [passwordOpen, setPasswordOpen] = useState(false);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [passwordSuccess, setPasswordSuccess] = useState<string | null>(null);

  const [logoutLoading, setLogoutLoading] = useState(false);

  const nameChanged =
    displayName.trim() !== "" && displayName.trim() !== session.display_name.trim();

  async function onSaveProfile(e: React.FormEvent) {
    e.preventDefault();
    setProfileLoading(true);
    setProfileError(null);
    setProfileSuccess(null);
    try {
      await identity.updateProfile({ display_name: displayName.trim() });
      setProfileSuccess("显示名称已更新");
      router.refresh();
    } catch (err) {
      setProfileError(err instanceof BffError ? err.message : "保存失败，请稍后重试");
    } finally {
      setProfileLoading(false);
    }
  }

  function openPasswordModal() {
    setCurrentPassword("");
    setNewPassword("");
    setPasswordError(null);
    setPasswordSuccess(null);
    setPasswordOpen(true);
  }

  async function onChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setPasswordLoading(true);
    setPasswordError(null);
    try {
      await identity.changePassword({
        current_password: currentPassword,
        new_password: newPassword,
      });
      setPasswordOpen(false);
      setPasswordSuccess("密码已更新");
    } catch (err) {
      setPasswordError(err instanceof BffError ? err.message : "修改失败，请检查当前密码是否正确");
    } finally {
      setPasswordLoading(false);
    }
  }

  function onLogout() {
    setLogoutLoading(true);
    // 浏览器整页跳转，确保 HttpOnly Cookie 被服务端 Set-Cookie 可靠清除
    window.location.href = "/api/bff/v1/auth/logout";
  }

  return (
    <div className="prefs">
      <header className="prefs-hero">
        <div className="prefs-avatar" aria-hidden="true">
          {initialsOf(session.display_name, session.email)}
        </div>
        <h1 className="prefs-name">{session.display_name}</h1>
        <p className="prefs-email">{session.email}</p>
      </header>

      <section className="prefs-group">
        <h2 className="prefs-group-title">账号</h2>
        <div className="prefs-list">
          <form className="prefs-row" onSubmit={onSaveProfile}>
            <label className="prefs-row-label" htmlFor="profile-display-name">
              显示名称
            </label>
            <input
              id="profile-display-name"
              className="prefs-row-input"
              value={displayName}
              onChange={(e) => {
                setDisplayName(e.target.value);
                setProfileSuccess(null);
              }}
              placeholder="输入显示名称"
              required
            />
            {nameChanged ? (
              <button
                className="btn btn-primary prefs-row-action"
                type="submit"
                disabled={profileLoading}
              >
                {profileLoading ? "保存中…" : "保存"}
              </button>
            ) : null}
          </form>
          <div className="prefs-row">
            <span className="prefs-row-label">邮箱</span>
            <span className="prefs-row-value">{session.email}</span>
          </div>
          <div className="prefs-row">
            <span className="prefs-row-label">角色</span>
            <span className="prefs-row-value">
              {session.is_super_admin ? "超级管理员" : "普通成员"}
            </span>
          </div>
        </div>
        {profileError ? <InlineAlert tone="error">{profileError}</InlineAlert> : null}
        {profileSuccess ? <InlineAlert tone="success">{profileSuccess}</InlineAlert> : null}
      </section>

      {consoleOptions.length > 1 ? (
        <section className="prefs-group">
          <h2 className="prefs-group-title">控制台</h2>
          <div className="prefs-list">
            {consoleOptions.map((option) => {
              const active = option.kind === currentConsole;
              return (
                <Link
                  key={option.kind}
                  href={option.href}
                  className="prefs-row prefs-row-link"
                  aria-current={active ? "page" : undefined}
                >
                  <span className="prefs-row-title">{option.label}</span>
                  {active ? (
                    <svg
                      className="prefs-row-icon prefs-row-check"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2.25"
                      aria-label="当前"
                    >
                      <path d="M5 12.5l4.5 4.5L19 7.5" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  ) : (
                    <svg
                      className="prefs-row-icon"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      aria-hidden
                    >
                      <path d="M9 18l6-6-6-6" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  )}
                </Link>
              );
            })}
          </div>
        </section>
      ) : null}

      <section className="prefs-group">
        <h2 className="prefs-group-title">安全</h2>
        <div className="prefs-list">
          <div className="prefs-row">
            <span className="prefs-row-title">登录密码</span>
            <button
              type="button"
              className="btn-secondary prefs-row-action"
              onClick={openPasswordModal}
            >
              修改
            </button>
          </div>
        </div>
        {passwordSuccess ? <InlineAlert tone="success">{passwordSuccess}</InlineAlert> : null}
      </section>

      <section className="prefs-group">
        <div className="prefs-list">
          <button
            type="button"
            className="prefs-row prefs-logout"
            onClick={onLogout}
            disabled={logoutLoading}
          >
            {logoutLoading ? "退出中…" : "退出登录"}
          </button>
        </div>
      </section>

      <Modal
        open={passwordOpen}
        onClose={() => setPasswordOpen(false)}
        title="修改密码"
        description="新密码至少 8 位，建议包含字母与数字"
        size="sm"
      >
        <form className="stack-md" onSubmit={onChangePassword}>
          <div>
            <label className="label" htmlFor="current-password">
              当前密码
            </label>
            <input
              id="current-password"
              className="input"
              type="password"
              autoComplete="current-password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
              autoFocus
            />
          </div>
          <div>
            <label className="label" htmlFor="new-password">
              新密码
            </label>
            <input
              id="new-password"
              className="input"
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              minLength={8}
              required
            />
          </div>
          {passwordError ? <InlineAlert tone="error">{passwordError}</InlineAlert> : null}
          <div className="prefs-modal-actions">
            <button
              type="button"
              className="btn-secondary"
              onClick={() => setPasswordOpen(false)}
            >
              取消
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={passwordLoading || !currentPassword || newPassword.length < 8}
            >
              {passwordLoading ? "更新中…" : "更新密码"}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
