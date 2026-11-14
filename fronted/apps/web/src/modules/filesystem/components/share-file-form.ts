import type {
  CreateGovernedGrantCommand,
  GovernedPermission,
  PrincipalType,
} from "@sparkops/contracts/filesystem";

export type ShareAudienceOption = {
  principalType: PrincipalType;
  principalId: string;
  label: string;
  memberCount?: number;
  approvalRequired?: boolean;
  disabledReason?: string;
};

export type ShareFormState = {
  actions: GovernedPermission[];
  inheritToChildren: boolean;
  expiresAt: string;
  effect: "ALLOW" | "DENY";
};

const actions: GovernedPermission[] = [
  "READ",
  "DOWNLOAD",
  "WRITE",
  "DELETE",
  "SHARE",
  "ADMIN",
];

export function createInitialShareForm(nodeType: "FILE" | "DIRECTORY"): ShareFormState {
  return {
    actions: ["READ", "DOWNLOAD"],
    inheritToChildren: nodeType === "DIRECTORY",
    expiresAt: "",
    effect: "ALLOW",
  };
}

export function allowedActions(principalType: PrincipalType): GovernedPermission[] {
  return principalType === "ORGANIZATION"
    ? actions.filter((action) => !["DELETE", "SHARE", "ADMIN"].includes(action))
    : [...actions];
}

export function impactSummary(option: ShareAudienceOption): string {
  if (option.principalType === "ORGANIZATION") {
    return "将共享给当前租户（所有具备租户成员资格的用户）";
  }
  if (option.memberCount == null) return "影响人数不可用";
  return option.principalType === "USER"
    ? `将影响${option.label}（${option.memberCount} 位成员）`
    : `将影响${option.label}的 ${option.memberCount} 位成员`;
}

export function submitLabel(approvalRequired: boolean): "立即共享" | "提交审批" {
  return approvalRequired ? "提交审批" : "立即共享";
}

export function canonicalSharePayload(input: CreateGovernedGrantCommand): string {
  return JSON.stringify({
    principal_type: input.principal_type,
    principal_ids: [...input.principal_ids].sort(),
    actions: [...input.actions].sort(),
    effect: input.effect ?? "ALLOW",
    inherit_to_children: input.inherit_to_children ?? false,
    expires_at: input.expires_at ?? null,
    reshare: input.reshare ?? false,
    source_grant_ids: [...(input.source_grant_ids ?? [])].sort(),
  });
}
