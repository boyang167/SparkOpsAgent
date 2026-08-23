import { redirect } from "next/navigation";
import { PlatformOrganizationsPanel } from "@/modules/tenant/components/PlatformOrganizationsPanel";
import { loadSession } from "@/server/loaders/session";

export default async function PlatformOrgsPage() {
  const session = await loadSession();
  if (!session) redirect("/login");
  if (!session.is_super_admin) redirect("/dashboard");

  return <PlatformOrganizationsPanel variant="settings" />;
}
