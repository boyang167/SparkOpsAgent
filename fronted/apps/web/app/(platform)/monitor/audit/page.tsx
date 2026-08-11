import { redirect } from "next/navigation";

/** 旧路径兼容：操作日志已迁移至 /monitor/operation */
export default function MonitorAuditRedirectPage() {
  redirect("/monitor/operation");
}
