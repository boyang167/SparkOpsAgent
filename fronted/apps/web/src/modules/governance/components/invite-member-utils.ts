import { BffError } from "@sparkops/contracts";

const INVITE_ERROR_MESSAGES: Record<string, string> = {
  GOVERNANCE_MEMBER_EXISTS: "该用户已在当前工作区，无需重复邀请",
  GOVERNANCE_MEMBER_NOT_FOUND: "未找到该邮箱对应的注册用户，请先让对方完成注册",
  TENANT_ORG_MEMBER_EXISTS: "该用户已在当前组织，无需重复邀请",
  TENANT_MEMBER_NOT_FOUND: "未找到该邮箱对应的注册用户，请先让对方完成注册",
  AUTH_FORBIDDEN: "你没有邀请成员的权限",
  GOVERNANCE_PERMISSION_DENIED: "你没有邀请成员的权限",
};

export function formatInviteError(err: unknown, fallback = "邀请失败，请稍后重试"): string {
  if (err instanceof BffError) {
    return INVITE_ERROR_MESSAGES[err.code] ?? err.message ?? fallback;
  }
  return fallback;
}
