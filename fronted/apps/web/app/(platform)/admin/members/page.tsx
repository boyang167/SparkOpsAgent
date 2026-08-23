import { InviteMemberForm } from "@/modules/governance/components/InviteMemberForm";
import { MemberBindings } from "@/modules/governance/components/MemberBindings";
import { loadGovernanceContext } from "@/server/loaders/governance-context";

export default async function AdminMembersPage() {
  const { workspaceId, roles, members } = await loadGovernanceContext();

  return (
    <section className="section stack-md">
      <InviteMemberForm workspaceId={workspaceId} />
      <MemberBindings workspaceId={workspaceId} members={members} roles={roles} />
    </section>
  );
}
