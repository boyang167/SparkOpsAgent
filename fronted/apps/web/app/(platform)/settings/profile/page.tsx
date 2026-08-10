import { redirect } from "next/navigation";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProfileSettings } from "@/modules/identity/components/ProfileSettings";
import { loadSession } from "@/server/loaders/session";

export default async function ProfileSettingsPage() {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Settings"
        title="个人设置"
        description="管理显示名称与登录密码"
      />
      <ProfileSettings session={session} />
    </div>
  );
}
