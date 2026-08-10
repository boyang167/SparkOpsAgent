import Link from "next/link";
import { LoginForm } from "@/modules/identity/components/LoginForm";

export default function LoginPage() {
  return (
    <div className="card p-8">
      <div className="mb-8">
        <p className="eyebrow">SparkOps Platform</p>
        <h1 className="page-title mt-2">登录</h1>
        <p className="text-muted mt-2">通过 BFF 安全接入，Cookie 会话管理。</p>
      </div>
      <LoginForm />
      <p className="text-muted mt-8 text-center text-sm">
        还没有账号？{" "}
        <Link href="/register" className="link-primary">
          注册
        </Link>
      </p>
    </div>
  );
}
