import Link from "next/link";
import { RegisterForm } from "@/modules/identity/components/RegisterForm";

export default function RegisterPage() {
  return (
    <div className="card p-8">
      <div className="mb-8">
        <p className="eyebrow">SparkOps Platform</p>
        <h1 className="page-title mt-2">注册</h1>
        <p className="text-muted mt-2">创建账号后即可创建 Org 与 Workspace。</p>
      </div>
      <RegisterForm />
      <p className="text-muted mt-8 text-center text-sm">
        已有账号？{" "}
        <Link href="/login" className="link-primary">
          登录
        </Link>
      </p>
    </div>
  );
}
