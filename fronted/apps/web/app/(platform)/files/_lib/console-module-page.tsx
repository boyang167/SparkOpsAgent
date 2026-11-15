import type { FileNamespaceVM } from "@sparkops/contracts/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import type { GovernanceSection } from "@/modules/filesystem/components/GovernanceWorkspace";
import type { WorkbenchTab } from "@/modules/filesystem/components/workbench-state";
import { FilesystemWorkbench } from "@/modules/filesystem/components/FilesystemWorkbench";
import {
  resolveFilesystemAuditMode,
} from "@/modules/filesystem/components/workbench-permissions";
import { loadFilesystemConsoleContext } from "./console-context";

const CONSOLE_PLACEHOLDER_NAMESPACE: FileNamespaceVM = {
  id: "00000000-0000-4000-8000-000000000010",
  workspace_id: "00000000-0000-4000-8000-000000000011",
  root_node_id: null,
  status: "ACTIVE",
};

export async function renderFilesystemConsoleModule(
  lockedTab: WorkbenchTab,
  options?: { governanceSection?: GovernanceSection },
) {
  const ctx = await loadFilesystemConsoleContext();
  if (!ctx.ok) {
    return ctx.element;
  }
  const { workspaceId, organizationId, permissions, members, memberDirectoryAvailable } =
    ctx.value;

  if (lockedTab === "audit" && !resolveFilesystemAuditMode(permissions)) {
    return <InlineAlert tone="error">当前角色无权查看文件审计。</InlineAlert>;
  }
  if (lockedTab === "approvals" && !permissions.includes("org:file:admin")) {
    return <InlineAlert tone="error">当前角色无权处理共享审批。</InlineAlert>;
  }
  if (lockedTab === "governance" && !permissions.includes("storage:read")) {
    return <InlineAlert tone="error">当前角色无权访问存储治理。</InlineAlert>;
  }

  const namespace: FileNamespaceVM = {
    ...CONSOLE_PLACEHOLDER_NAMESPACE,
    workspace_id: workspaceId,
  };

  return (
    <FilesystemWorkbench
      workspaceId={workspaceId}
      namespace={namespace}
      initialNodes={[]}
      initialState={{ tab: lockedTab, view: "list", query: "" }}
      permissions={permissions}
      organizationId={organizationId}
      members={members}
      memberDirectoryAvailable={memberDirectoryAvailable}
      lockedTab={lockedTab}
      hideTabList
      governanceSection={options?.governanceSection}
    />
  );
}