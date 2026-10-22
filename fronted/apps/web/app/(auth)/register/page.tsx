import Link from "next/link";
import { RegisterForm } from "@/modules/identity/components/RegisterForm";

export default function RegisterPage() {
  return (
    <div className="auth-card">
      <header className="auth-card-header">
        <p className="auth-card-kicker">SparkOps</p>
        <h1 className="auth-card-title">创建账号</h1>
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
