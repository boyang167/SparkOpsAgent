import { PageHeader } from "@/components/ui/PageHeader";
import { PlatformMembersPanel } from "@/modules/identity/components/PlatformMembersPanel";
import { requirePlatformConsoleAccess } from "@/server/loaders/console-access";

export default async function PlatformAccountsPage() {
  const { session } = await requirePlatformConsoleAccess();
  return (
    <div className="stack-lg">
      <PageHeader eyebrow="平台运营" title="平台账号" description="管理全平台用户账号、状态与超级管理员授权。" />
      <PlatformMembersPanel currentMemberId={session.id} />
    </div>
  );
}
