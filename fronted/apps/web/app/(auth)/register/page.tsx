import Link from "next/link";
import { RegisterForm } from "@/modules/identity/components/RegisterForm";

export default function RegisterPage() {
  return (
    <div className="auth-card">
      <header className="auth-card-header">
        <p className="auth-card-eyebrow">Get started</p>
        <h1 className="auth-card-title">创建账号</h1>
        <p className="auth-card-desc">注册后即可创建组织与工作区，开始使用 Agent 运营平台</p>
      </header>

      <RegisterForm />

      <footer className="auth-card-footer">
        <span className="text-muted text-sm">已有账号？</span>
        <Link href="/login" className="auth-card-link">
          登录
        </Link>
      </footer>
    </div>
  );
}
