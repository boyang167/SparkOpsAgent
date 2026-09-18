"use client";

import { useState } from "react";
import {
  configReasonLabel,
  isConfigResourceSelected,
  type ConfigResourceItem,
  type ConfigResourceRef,
} from "./deploy-job-spec";

export interface ConfigResourceGroup {
  projectId: string;
  name: string;
  items: ConfigResourceItem[];
}

interface DeployConfigResourceListProps {
  loading: boolean;
  error: string | null;
  groups: ConfigResourceGroup[];
  selected: ConfigResourceRef[];
  onToggle: (item: ConfigResourceItem, selected: boolean) => void;
}

export function DeployConfigResourceList({
  loading,
  error,
  groups,
  selected,
  onToggle,
}: DeployConfigResourceListProps) {
  const [openKey, setOpenKey] = useState<string | null>(null);
  const selectedCount = groups.reduce(
    (count, group) => count + group.items.filter((item) => isConfigResourceSelected(selected, item)).length,
    0,
  );

  return (
    <div className="pwf-config-sync">
      <div className="pwf-config-sync-head">
        <span>将同步的配置资源</span>
        <span className="text-xs text-muted">已选 {selectedCount} 项</span>
      </div>
      {loading ? <p className="text-xs text-muted">正在读取环境中的 ConfigMap 和 Secret…</p> : null}
      {error ? <p className="text-xs text-muted">{error}</p> : null}
      {!loading && groups.length === 0 ? <p className="text-xs text-muted">上游构建还没有项目组件</p> : null}
      {groups.map((group) => (
        <section key={group.projectId} className="pwf-config-sync-group">
          <h4>{group.name}</h4>
          {group.items.length === 0 ? <p className="text-xs text-muted">该项目没有可同步的配置资源</p> : null}
          <ul>
            {group.items.map((item) => {
              const key = `${item.projectId}/${item.kind}/${item.name}`;
              const checked = isConfigResourceSelected(selected, item);
              const open = openKey === key;
              return (
                <li key={key}>
                  <label className="pwf-config-sync-row">
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={(event) => onToggle(item, event.target.checked)}
                    />
                    <span className="badge badge-neutral">{item.kind}</span>
                    <button
                      type="button"
                      className="pwf-config-sync-name"
                      aria-expanded={open}
                      onClick={(event) => {
                        event.preventDefault();
                        setOpenKey(open ? null : key);
                      }}
                    >
                      {item.name}
                    </button>
                    <span className="text-xs text-muted">{configReasonLabel(item.reason)}</span>
                  </label>
                  {item.note ? <p className="pwf-config-sync-note text-xs text-muted">{item.note}</p> : null}
                  {open ? <pre className="pwf-config-sync-yaml">{item.yaml || "没有可预览的 YAML"}</pre> : null}
                </li>
              );
            })}
          </ul>
        </section>
      ))}
    </div>
  );
}
