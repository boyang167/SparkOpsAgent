"use client";

import { useState } from "react";

export function LogoutButton() {
  const [loading, setLoading] = useState(false);

  function onLogout() {
    setLoading(true);
    // 浏览器整页跳转，确保 HttpOnly Cookie 被服务端 Set-Cookie 可靠清除
    window.location.href = "/api/bff/v1/auth/logout";
  }

  return (
    <button className="btn-ghost" type="button" onClick={onLogout} disabled={loading}>
      {loading ? "退出中…" : "退出"}
    </button>
  );
}
