import { PlatformMembersPanel } from "@/modules/identity/components/PlatformMembersPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformAccountsPage() {
  const { session } = await requirePlatformConsoleAccess();
  return (
    <div className="platform-page">
      <PlatformMembersPanel currentMemberId={session.id} variant="platform" />
    </div>
  );
}
