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
    <section className="storage-pool-members" aria-labelledby="storage-pool-members-title">
      <header className="storage-pool-members-header">
        <h3 id="storage-pool-members-title" className="storage-pool-members-title">
          Provider 成员
        </h3>
        <p className="storage-pool-members-hint">至少配置一个 PRIMARY；同一 Provider 只能添加一次。</p>
      </header>
      {activeProviders.length === 0 ? (
        <InlineAlert tone="warning">暂无可用 Provider，请先注册并启用 Provider。</InlineAlert>
      ) : (
        <>
          <div className="storage-pool-members-table">
            <div className="storage-pool-members-row storage-pool-members-row-head" aria-hidden>
              <span>Provider</span>
              <span>角色</span>
              <span>优先级</span>
              <span />
            </div>
            {members.map((member, index) => (
              <div key={`${member.provider_id}-${index}`} className="storage-pool-members-row">
                <select
                  className="input"
                  aria-label="Provider"
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
                      {provider.name}
                    </option>
                  ))}
                </select>
                <select
                  className="input"
                  aria-label="角色"
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
                <input
                  type="number"
                  className="input storage-pool-members-priority"
                  aria-label="优先级"
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
                <button
                  type="button"
                  className="storage-pool-members-remove"
                  aria-label={`移除 ${providerNameById.get(member.provider_id) ?? "Provider"} 成员`}
                  onClick={() => {
                    onClearError();
                    onChange(members.filter((_, itemIndex) => itemIndex !== index));
                  }}
                >
                  移除
                </button>
              </div>
            ))}
          </div>
          <button
            type="button"
            className="btn btn-secondary btn-sm storage-pool-members-add"
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
        </>
      )}
    </section>
  );
}