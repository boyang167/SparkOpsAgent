"use client";

import { useWorkspaceMembers } from "../hooks/useWorkspaceMembers";

interface ReleasePlanApproverFieldProps {
  workspaceId: string;
  approverIds: string[];
  onChange: (approverIds: string[]) => void;
}

export function ReleasePlanApproverField({ workspaceId, approverIds, onChange }: ReleasePlanApproverFieldProps) {
  const { members, loading, memberName } = useWorkspaceMembers(workspaceId);

  function toggle(memberId: string) {
    if (approverIds.includes(memberId)) {
      onChange(approverIds.filter((id) => id !== memberId));
      return;
    }
    onChange([...approverIds, memberId]);
  }

  if (loading) {
    return <p className="text-xs text-muted-foreground">加载成员…</p>;
  }

  if (members.length === 0) {
    return <p className="text-xs text-muted-foreground">暂无可用工作空间成员</p>;
  }

  return (
    <div className="release-plan-approver-field stack-sm">
      <div className="release-plan-approver-list" role="listbox" aria-label="审批人" aria-multiselectable="true">
        {members.map((member) => {
          const selected = approverIds.includes(member.member_id);
          const label = member.display_name?.trim() || member.email || member.member_id;
          return (
            <label
              key={member.member_id}
              className={`release-plan-approver-option${selected ? " is-selected" : ""}`}
            >
              <input type="checkbox" checked={selected} onChange={() => toggle(member.member_id)} />
              <span className="release-plan-approver-option-copy">
                <strong>{label}</strong>
                {member.email ? <span>{member.email}</span> : null}
              </span>
            </label>
          );
        })}
      </div>
      {approverIds.length > 0 ? (
        <div className="release-plan-approver-pills">
          {approverIds.map((id) => (
            <span key={id} className="release-plan-approver-pill">
              {memberName(id)}
            </span>
          ))}
        </div>
      ) : (
        <p className="field-hint text-xs text-muted-foreground">未指定时，任意有权限的成员均可审批。</p>
      )}
    </div>
  );
}
