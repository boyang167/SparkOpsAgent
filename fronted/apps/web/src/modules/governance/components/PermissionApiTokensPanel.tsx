"use client";

import { useCallback, useEffect, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { APITokenVM, RoleTreeVM, WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

interface PermissionApiTokensPanelProps {
  workspaceId: string;
  members: WorkspaceMemberVM[];
  roles: RoleTreeVM[];
}

function flattenRoles(nodes: RoleTreeVM[]): RoleTreeVM[] {
  return nodes.flatMap((node) => [node, ...(node.children ? flattenRoles(node.children) : [])]);
}

export function PermissionApiTokensPanel({ workspaceId, members, roles }: PermissionApiTokensPanelProps) {
  const [tokens, setTokens] = useState<APITokenVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [memberId, setMemberId] = useState("");
  const [roleId, setRoleId] = useState("");
  const [remark, setRemark] = useState("");
  const [expireDays, setExpireDays] = useState(30);
  const [issuedToken, setIssuedToken] = useState<string | null>(null);
  const flatRoles = flattenRoles(roles);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setTokens(await governance.listApiTokens(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!memberId) return;
    setError(null);
    try {
      const result = await governance.createApiToken(workspaceId, {
        member_id: memberId,
        role_id: roleId || undefined,
        remark,
        expire_days: expireDays,
      });
      setIssuedToken(result.token);
      setCreateOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "签发失败");
    }
  }

  async function onRevoke(id: string) {
    if (!window.confirm("确定作废该 Token？")) return;
    try {
      await governance.revokeApiToken(workspaceId, id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "作废失败");
    }
  }

  return (
    <div className="section stack-md">
      <div className="flex justify-between gap-3">
        <div>
          <h2 className="section-title">API Token</h2>
          <p className="text-muted mt-1">为成员签发长期 JWT（仅创建时展示完整 Token）</p>
        </div>
        <button type="button" className="btn" onClick={() => setCreateOpen(true)}>
          签发 Token
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {issuedToken ? (
        <InlineAlert tone="success">
          <p className="font-medium mb-1">Token 已签发（请立即复制，不会再次显示）</p>
          <code className="text-xs break-all">{issuedToken}</code>
        </InlineAlert>
      ) : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : tokens.length === 0 ? (
        <div className="empty-state">暂无 Token</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>前缀</th>
                <th>成员</th>
                <th>角色</th>
                <th>状态</th>
                <th>过期</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {tokens.map((token) => (
                <tr key={token.id}>
                  <td className="font-mono">{token.token_prefix}…</td>
                  <td className="font-mono text-xs">{token.member_id.slice(0, 8)}…</td>
                  <td className="text-xs">{token.role_id ? token.role_id.slice(0, 8) + "…" : "-"}</td>
                  <td>{token.status}</td>
                  <td>{token.expires_at ? new Date(token.expires_at).toLocaleString() : "永久"}</td>
                  <td>
                    {token.status === "active" ? (
                      <button type="button" className="text-xs text-red-600 hover:underline" onClick={() => void onRevoke(token.id)}>
                        作废
                      </button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="签发 API Token">
        <form onSubmit={onCreate} className="stack-md">
          <label className="stack-xs">
            <span className="label">成员</span>
            <select className="input" value={memberId} onChange={(e) => setMemberId(e.target.value)} required>
              <option value="">选择成员</option>
              {members.map((member) => (
                <option key={member.member_id} value={member.member_id}>
                  {member.display_name} ({member.email})
                </option>
              ))}
            </select>
          </label>
          <label className="stack-xs">
            <span className="label">绑定角色（可选）</span>
            <select className="input" value={roleId} onChange={(e) => setRoleId(e.target.value)}>
              <option value="">不绑定角色</option>
              {flatRoles.map((role) => (
                <option key={role.id} value={role.id}>
                  {role.name}
                </option>
              ))}
            </select>
          </label>
          <label className="stack-xs">
            <span className="label">有效期（天）</span>
            <select className="input" value={expireDays} onChange={(e) => setExpireDays(Number(e.target.value))}>
              <option value={1}>1 天</option>
              <option value={7}>7 天</option>
              <option value={30}>30 天</option>
              <option value={90}>90 天</option>
              <option value={0}>永久</option>
            </select>
          </label>
          <input className="input" placeholder="备注" value={remark} onChange={(e) => setRemark(e.target.value)} />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setCreateOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn">
              签发
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
