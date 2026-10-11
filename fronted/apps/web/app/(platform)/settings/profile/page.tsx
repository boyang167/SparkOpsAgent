import { redirect } from "next/navigation";
import { ProfileSettings } from "@/modules/identity/components/ProfileSettings";
import { loadConsoleAccess } from "@/server/loaders/console-access";

export default async function ProfileSettingsPage() {
  const { session, isSuperAdmin, canManageAdmin } = await loadConsoleAccess();
  if (!session) {
    redirect("/login");
  }

  return (
    <div className="stack-lg">
      <ProfileSettings
        session={session}
        isSuperAdmin={isSuperAdmin}
        canManageAdmin={canManageAdmin}
      />
    </div>
  );
}
