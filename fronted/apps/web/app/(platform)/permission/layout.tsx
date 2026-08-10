import { PermissionModuleShell } from "@/components/layout/PermissionModuleShell";

export default function PermissionLayout({ children }: { children: React.ReactNode }) {
  return <PermissionModuleShell>{children}</PermissionModuleShell>;
}
