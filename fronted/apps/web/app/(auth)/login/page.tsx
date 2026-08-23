import Link from "next/link";
import { LoginForm } from "@/modules/identity/components/LoginForm";

export default function LoginPage() {
  return (
    <div className="auth-card">
      <header className="auth-card-header">
        <p className="auth-card-eyebrow">Welcome back</p>
        <h1 className="auth-card-title">登录 SparkOps</h1>
        <p className="auth-card-desc">使用企业账号或邮箱密码进入控制台</p>
      </header>

      <LoginForm />

      <footer className="auth-card-footer">
        <span className="text-muted text-sm">还没有账号？</span>
        <Link href="/register" className="auth-card-link">
          创建账号
        </Link>
      </footer>
    </div>
  );
}
