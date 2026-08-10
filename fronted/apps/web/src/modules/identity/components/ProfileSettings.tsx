"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { identity } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { MeVM } from "@sparkops/contracts/identity";

interface ProfileSettingsProps {
  session: MeVM;
}

export function ProfileSettings({ session }: ProfileSettingsProps) {
  const router = useRouter();
  const [displayName, setDisplayName] = useState(session.display_name);
  const [profileLoading, setProfileLoading] = useState(false);
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileSuccess, setProfileSuccess] = useState<string | null>(null);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [passwordSuccess, setPasswordSuccess] = useState<string | null>(null);

  async function onSaveProfile(e: React.FormEvent) {
    e.preventDefault();
    setProfileLoading(true);
    setProfileError(null);
    setProfileSuccess(null);
    try {
      await identity.updateProfile({ display_name: displayName });
      setProfileSuccess("资料已更新");
      router.refresh();
    } catch (err) {
      setProfileError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setProfileLoading(false);
    }
  }

  async function onChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setPasswordLoading(true);
    setPasswordError(null);
    setPasswordSuccess(null);
    try {
      await identity.changePassword({
        current_password: currentPassword,
        new_password: newPassword,
      });
      setCurrentPassword("");
      setNewPassword("");
      setPasswordSuccess("密码已更新");
    } catch (err) {
      setPasswordError(err instanceof BffError ? err.message : "修改失败");
    } finally {
      setPasswordLoading(false);
    }
  }

  return (
    <div className="stack-lg max-w-xl">
      <section className="section stack-md">
        <div>
          <h2 className="section-title">个人资料</h2>
          <p className="text-muted mt-1">修改显示名称</p>
        </div>
        <form className="stack-md" onSubmit={onSaveProfile}>
          <div>
            <label className="label" htmlFor="profile-email">
              邮箱
            </label>
            <input id="profile-email" className="input" value={session.email} disabled />
          </div>
          <div>
            <label className="label" htmlFor="profile-display-name">
              显示名称
            </label>
            <input
              id="profile-display-name"
              className="input"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
            />
          </div>
          {profileError ? <p className="error">{profileError}</p> : null}
          {profileSuccess ? <p className="text-sm text-green-600">{profileSuccess}</p> : null}
          <button className="btn" type="submit" disabled={profileLoading}>
            {profileLoading ? "保存中..." : "保存资料"}
          </button>
        </form>
      </section>

      <section className="section stack-md">
        <div>
          <h2 className="section-title">修改密码</h2>
          <p className="text-muted mt-1">新密码至少 8 位</p>
        </div>
        <form className="stack-md" onSubmit={onChangePassword}>
          <div>
            <label className="label" htmlFor="current-password">
              当前密码
            </label>
            <input
              id="current-password"
              className="input"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
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
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              minLength={8}
              required
            />
          </div>
          {passwordError ? <p className="error">{passwordError}</p> : null}
          {passwordSuccess ? <p className="text-sm text-green-600">{passwordSuccess}</p> : null}
          <button className="btn" type="submit" disabled={passwordLoading}>
            {passwordLoading ? "更新中..." : "更新密码"}
          </button>
        </form>
      </section>
    </div>
  );
}
