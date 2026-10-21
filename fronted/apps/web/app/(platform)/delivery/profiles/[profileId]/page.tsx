import { redirect } from "next/navigation";
import Link from "next/link";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProfileEditor } from "@/modules/delivery/cd/components/ProfileEditor/ProfileEditor";
import { loadSession } from "@/server/loaders/session";

interface PageProps {
  params: Promise<{ profileId: string }>;
}

export default async function ProfileEditorRoutePage({ params }: PageProps) {
  const session = await loadSession();
  if (!session) redirect("/login");
  const { profileId } = await params;

  return (
    <div className="stack-lg">
      <PageHeader eyebrow="Profile 编辑器" title="环境差异" description="L2 环境参数表单与 YAML（Mock）" />
      <Link href="/delivery/profiles" className="link-back">
        返回 Profile 列表
      </Link>
      <ProfileEditor profileId={profileId} />
    </div>
  );
}
