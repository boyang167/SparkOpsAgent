import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { BuildDetailPanel } from "@/modules/delivery/components/BuildDetailPanel";
import { loadBuild } from "@/server/loaders/delivery";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";

interface BuildDetailPageProps {
  params: Promise<{ buildId: string }>;
}

export default async function BuildDetailPage({ params }: BuildDetailPageProps) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { buildId } = await params;
  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ?? workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  let build;
  try {
    build = await loadBuild(workspaceId, buildId);
  } catch {
    notFound();
  }

  return (
    <div className="stack-lg">
      <Link href="/delivery/builds" className="link-back">
        ← 返回构建列表
      </Link>

      <PageHeader
        eyebrow="Build"
        title={`构建 #${build.build_number}`}
        description={`${build.branch} · ${build.jenkins_job ?? "jenkins"}`}
      />

      <BuildDetailPanel workspaceId={workspaceId} initialBuild={build} />
    </div>
  );
}
