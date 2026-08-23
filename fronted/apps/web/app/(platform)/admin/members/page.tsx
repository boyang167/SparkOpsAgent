import { AdminMembersPanel } from "@/modules/governance/components/AdminMembersPanel";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function AdminMembersPage() {
  const { workspaceId, roles, members } = await loadGovernanceContext();

  return <AdminMembersPanel workspaceId={workspaceId} members={members} roles={roles} />;
}
