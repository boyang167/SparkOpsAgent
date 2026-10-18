"use client";

import { useEffect, useMemo, useState } from "react";
import type { EnvironmentSyncProjectSpec } from "./environment-sync-job-spec";
import { environmentSyncResourceKey } from "./environment-sync-resource-overrides";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";

export interface EnvironmentSyncSourceResourceVM {
  kind: string;
  name: string;
  yaml?: string;
  managed_pull_secret?: boolean;
  note?: string;
}

interface EnvironmentSyncResourceEditorModalProps {
  open: boolean;
  projectName: string;
  projectSpec: EnvironmentSyncProjectSpec;
  resources: EnvironmentSyncSourceResourceVM[];
  loading: boolean;
  error: string | null;
  readOnly?: boolean;
  onClose: () => void;
  onSave: (overrides: Record<string, string>) => void;
}

export function EnvironmentSyncResourceEditorModal({
  open,
  projectName,
  projectSpec,
  resources,
  loading,
  error,
  readOnly = false,
  onClose,
  onSave,
}: EnvironmentSyncResourceEditorModalProps) {
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [draftOverrides, setDraftOverrides] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!open) return;
    setDraftOverrides({ ...projectSpec.resourceOverrides });
    const first = resources.find((item) => !item.managed_pull_secret);
    setSelectedKey(first ? environmentSyncResourceKey(first.kind, first.name) : null);
  }, [open, projectSpec.resourceOverrides, resources]);

  const selected = useMemo(
    () => resources.find((item) => environmentSyncResourceKey(item.kind, item.name) === selectedKey) ?? null,
    [resources, selectedKey],
  );

  const selectedKeyValue = selected ? environmentSyncResourceKey(selected.kind, selected.name) : "";
  const editorValue = selected
    ? draftOverrides[selectedKeyValue] ?? selected.yaml ?? ""
    : "";

  function saveCurrentYaml(value: string) {
    if (!selected || selected.managed_pull_secret) return;
    const key = environmentSyncResourceKey(selected.kind, selected.name);
    setDraftOverrides((current) => ({ ...current, [key]: value }));
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={projectName}
      description="编辑将同步到目标环境的 Kubernetes 配置，保存后随任务一起发布。"
      size="xl"
      priority
      panelClassName="pwf-env-sync-editor-modal"
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" onClick={onClose}>
            关闭
          </button>
          {!readOnly ? (
            <button
              type="button"
              className="btn-primary"
              disabled={loading}
              onClick={() => onSave(draftOverrides)}
            >
              保存修改
            </button>
          ) : null}
        </div>
      }
    >
      <div className="pwf-env-sync-editor">
        {loading ? <p className="pwf-env-sync-editor-status">正在加载源环境配置…</p> : null}
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {!loading && !error && resources.length === 0 ? (
          <p className="pwf-env-sync-editor-status">当前选项下没有可同步的配置资源</p>
        ) : null}
        {!loading && resources.length > 0 ? (
          <div className="pwf-env-sync-editor-body">
            <ul className="pwf-env-sync-editor-list">
              {resources.map((item) => {
                const key = environmentSyncResourceKey(item.kind, item.name);
                const active = key === selectedKey;
                const edited = Boolean(draftOverrides[key]?.trim());
                return (
                  <li key={key}>
                    <button
                      type="button"
                      className={`pwf-env-sync-editor-item${active ? " pwf-env-sync-editor-item--active" : ""}`}
                      onClick={() => setSelectedKey(key)}
                    >
                      <span className="pwf-env-sync-editor-item-kind">{item.kind}</span>
                      <span className="pwf-env-sync-editor-item-name">{item.name}</span>
                      {edited ? <span className="pwf-env-sync-editor-edited">已修改</span> : null}
                    </button>
                  </li>
                );
              })}
            </ul>
            <div className="pwf-env-sync-editor-pane">
              {selected ? (
                <>
                  <div className="pwf-env-sync-editor-pane-head">
                    <span className="pwf-env-sync-editor-pane-title">
                      {selected.kind}/{selected.name}
                    </span>
                    {selected.note ? <p className="pwf-env-sync-editor-pane-note">{selected.note}</p> : null}
                  </div>
                  {selected.managed_pull_secret ? (
                    <p className="pwf-env-sync-editor-status">
                      镜像拉取 Secret 由平台在目标环境自动 ensure，不支持编辑密文。
                    </p>
                  ) : (
                    <textarea
                      className="pwf-env-sync-editor-yaml"
                      value={editorValue}
                      readOnly={readOnly}
                      spellCheck={false}
                      onChange={(event) => saveCurrentYaml(event.target.value)}
                    />
                  )}
                </>
              ) : (
                <p className="pwf-env-sync-editor-status">选择左侧资源查看 YAML</p>
              )}
            </div>
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
