import { AdminModuleShell } from "@/components/layout/AdminModuleShell";
import { requireAdminConsoleAccess } from "@/server/loaders/console-access";

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  await requireAdminConsoleAccess();
  return <AdminModuleShell>{children}</AdminModuleShell>;
}
