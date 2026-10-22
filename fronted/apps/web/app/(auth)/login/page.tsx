import Link from "next/link";
import { LoginForm } from "@/modules/identity/components/LoginForm";

export default function LoginPage() {
  return (
    <div className="auth-card">
      <header className="auth-card-header">
        <p className="auth-card-kicker">SparkOps</p>
        <h1 className="auth-card-title">登录</h1>
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
