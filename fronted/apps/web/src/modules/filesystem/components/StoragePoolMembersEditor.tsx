import type { StorageProviderVM } from "@sparkops/contracts/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import type { StoragePoolDraftMember } from "./storage-pool-panel-view";

const POOL_MEMBER_MODES = ["PRIMARY", "REPLICA", "ARCHIVE", "DRAINING"] as const;

export type StoragePoolMembersEditorProps = {
  activeProviders: StorageProviderVM[];
  members: StoragePoolDraftMember[];
  providerNameById: Map<string, string>;
  onChange: (members: StoragePoolDraftMember[]) => void;
  onClearError: () => void;
};

export function StoragePoolMembersEditor({
  activeProviders,
  members,
  providerNameById,
  onChange,
  onClearError,
}: StoragePoolMembersEditorProps) {
  return (
    <div className="stack-sm">
      <div>
        <div className="text-sm font-medium">Provider 成员</div>
        <p className="text-xs text-muted">至少配置一个 PRIMARY；同一 Provider 只能添加一次。</p>
      </div>
      {activeProviders.length === 0 ? (
        <InlineAlert tone="warning">暂无可用 Provider，请先注册并启用 Provider。</InlineAlert>
      ) : (
        <>
          {members.map((member, index) => (
            <div
              key={`${member.provider_id}-${index}`}
              className="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,0.75fr)_7rem_auto]"
            >
              <label className="stack-xs">
                <span className="text-xs text-muted">Provider</span>
                <select
                  className="input"
                  value={member.provider_id}
                  onChange={(e) => {
                    onClearError();
                    onChange(
                      members.map((item, itemIndex) =>
                        itemIndex === index ? { ...item, provider_id: e.target.value } : item,
                      ),
                    );
                  }}
                >
                  {activeProviders.map((provider) => (
                    <option key={provider.id} value={provider.id}>
                      {provider.name}（{provider.provider_type}）
                    </option>
                  ))}
                </select>
              </label>
              <label className="stack-xs">
                <span className="text-xs text-muted">角色</span>
                <select
                  className="input"
                  value={member.mode}
                  onChange={(e) => {
                    onClearError();
                    onChange(
                      members.map((item, itemIndex) =>
                        itemIndex === index
                          ? { ...item, mode: e.target.value as StoragePoolDraftMember["mode"] }
                          : item,
                      ),
                    );
                  }}
                >
                  {POOL_MEMBER_MODES.map((mode) => (
                    <option key={mode} value={mode}>
                      {mode}
                    </option>
                  ))}
                </select>
              </label>
              <label className="stack-xs">
                <span className="text-xs text-muted">优先级</span>
                <input
                  type="number"
                  className="input"
                  value={member.priority}
                  onChange={(e) => {
                    onClearError();
                    onChange(
                      members.map((item, itemIndex) =>
                        itemIndex === index ? { ...item, priority: Number(e.target.value) } : item,
                      ),
                    );
                  }}
                />
              </label>
              <div className="flex items-end">
                <button
                  type="button"
                  className="btn btn-ghost"
                  aria-label={`移除 ${providerNameById.get(member.provider_id) ?? "Provider"} 成员`}
                  onClick={() => {
                    onClearError();
                    onChange(members.filter((_, itemIndex) => itemIndex !== index));
                  }}
                >
                  移除
                </button>
              </div>
            </div>
          ))}
          <div>
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              disabled={members.length >= activeProviders.length}
              onClick={() => {
                const usedProviderIds = new Set(members.map((item) => item.provider_id));
                const provider = activeProviders.find((item) => !usedProviderIds.has(item.id));
                if (!provider) return;
                onClearError();
                onChange([...members, { provider_id: provider.id, mode: "REPLICA", priority: 0 }]);
              }}
            >
              添加成员
            </button>
          </div>
        </>
      )}
    </div>
  );
}