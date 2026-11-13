import Link from "next/link";
import { PageHeader } from "@/components/ui/PageHeader";
import { FileAuditPanel } from "@/modules/filesystem/components/FileAuditPanel";
import { LifecyclePolicyPanel } from "@/modules/filesystem/components/LifecyclePolicyPanel";
import { StorageBindingPanel } from "@/modules/filesystem/components/StorageBindingPanel";
import { StoragePoolPanel } from "@/modules/filesystem/components/StoragePoolPanel";
import { loadPlatformPageContext } from "@/server/loaders/platform-page";

const DEFAULT_PERMISSIONS = ["storage:read", "lifecycle:read"] as const;

export default async function FilesystemAdminPage() {
  const { workspaceId } = await loadPlatformPageContext();
  const permissions = [...DEFAULT_PERMISSIONS];

  return (
    <div className="stack-xl">
      <PageHeader
        eyebrow="Filesystem"
        title="存储与生命周期管理"
        description="管理 Provider、存储池、作用域绑定、生命周期策略与文件审计。"
        actions={
          <Link href="/files" className="btn btn-ghost">
            返回文件浏览
          </Link>
        }
      />
      <StoragePoolPanel workspaceId={workspaceId} permissions={permissions} />
      <StorageBindingPanel workspaceId={workspaceId} permissions={permissions} />
      <LifecyclePolicyPanel workspaceId={workspaceId} permissions={permissions} />
      <FileAuditPanel workspaceId={workspaceId} />
    </div>
  );
}
