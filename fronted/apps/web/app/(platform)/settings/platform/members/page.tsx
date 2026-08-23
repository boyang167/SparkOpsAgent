import { redirect } from "next/navigation";
import { PlatformMembersPanel } from "@/modules/identity/components/PlatformMembersPanel";
import { loadSession } from "@/server/loaders/session";

export default async function PlatformMembersPage() {
  const session = await loadSession();
  if (!session) redirect("/login");
  if (!session.is_super_admin) redirect("/dashboard");

  return <PlatformMembersPanel currentMemberId={session.id} variant="settings" />;
}
