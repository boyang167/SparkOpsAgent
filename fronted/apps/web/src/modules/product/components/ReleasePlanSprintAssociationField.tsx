"use client";

import { useMemo, useState } from "react";
import type { ReleasePlanSprintOption } from "./ReleasePlanForm";
import { ReleasePlanLinkSprintModal } from "./ReleasePlanLinkSprintModal";

interface ReleasePlanSprintAssociationFieldProps {
  workspaceId: string;
  productId?: string;
  sprintId: string;
  sprintOptions: ReleasePlanSprintOption[];
  sprintLocked?: boolean;
  onChange: (sprintId: string) => void;
  onSprintOptionsChange?: (options: ReleasePlanSprintOption[]) => void;
  compact?: boolean;
  embedded?: boolean;
  /** @deprecated use embedded */
  actionOnly?: boolean;
  /** @deprecated use embedded */
  bodyOnly?: boolean;
}

export function ReleasePlanSprintAssociationField({
  workspaceId,
  productId,
  sprintId,
  sprintOptions,
  sprintLocked,
  onChange,
  onSprintOptionsChange,
  compact,
  embedded,
  actionOnly,
  bodyOnly,
}: ReleasePlanSprintAssociationFieldProps) {
  const isEmbedded = embedded || compact;
  const [modalOpen, setModalOpen] = useState(false);
  const [extraOptions, setExtraOptions] = useState<ReleasePlanSprintOption[]>([]);

  const mergedOptions = useMemo(() => {
    const map = new Map<string, ReleasePlanSprintOption>();
    for (const item of [...extraOptions, ...sprintOptions]) {
      map.set(item.id, item);
    }
    return [...map.values()];
  }, [extraOptions, sprintOptions]);

  const selected = useMemo(
    () => mergedOptions.find((item) => item.id === sprintId),
    [mergedOptions, sprintId],
  );

  const jiraLabel = useMemo(() => {
    const ref = selected?.external;
    if (!ref || ref.provider !== "jira") return null;
    const parts = [ref.project_key, ref.sprint_id ? `Sprint ${ref.sprint_id}` : null].filter(Boolean);
    return parts.length > 0 ? parts.join(" · ") : null;
  }, [selected?.external]);

  function handleSprintCreated(option: ReleasePlanSprintOption) {
    setExtraOptions((prev) => {
      if (prev.some((item) => item.id === option.id)) return prev;
      const next = [option, ...prev];
      onSprintOptionsChange?.(next);
      return next;
    });
  }

  const linkButton = sprintLocked ? null : (
    <button
      type="button"
      className="btn-secondary text-sm release-plan-sprint-link-btn"
      onClick={() => setModalOpen(true)}
    >
      关联冲刺
    </button>
  );

  const modal = (
    <ReleasePlanLinkSprintModal
      open={modalOpen}
      workspaceId={workspaceId}
      productId={productId}
      sprintOptions={mergedOptions}
      selectedSprintId={sprintId}
      onClose={() => setModalOpen(false)}
      onConfirm={onChange}
      onSprintCreated={handleSprintCreated}
    />
  );

  if (actionOnly) {
    return (
      <>
        {linkButton}
        {modal}
      </>
    );
  }

  if (bodyOnly) {
    return selected ? (
      <div className="release-plan-sprint-assoc-card release-plan-sprint-assoc-card-embedded">
        <div className="stack-xs">
          <p className="text-sm font-medium">{selected.name}</p>
          {jiraLabel ? <p className="text-xs text-muted-foreground">Jira：{jiraLabel}</p> : null}
        </div>
      </div>
    ) : null;
  }

  if (sprintLocked) {
    return (
      <section className={isEmbedded ? "release-plan-sprint-assoc-compact" : "release-plan-form-section"}>
        {!isEmbedded ? (
          <div className="release-plan-sprint-assoc-bar">
            <h3 className="release-plan-form-section-title">需求关联</h3>
          </div>
        ) : null}
        <div className="release-plan-form-section-body">
          <p className="text-sm text-muted-foreground">
            {sprintId ? `当前迭代：${selected?.name ?? sprintId}` : "当前视图已绑定迭代"}
          </p>
          {jiraLabel ? <p className="text-xs text-muted-foreground">Jira：{jiraLabel}</p> : null}
        </div>
      </section>
    );
  }

  if (isEmbedded) {
    return (
      <>
        {selected ? (
          <div className="release-plan-sprint-assoc-card release-plan-sprint-assoc-card-embedded">
            <div className="stack-xs">
              <p className="text-sm font-medium">{selected.name}</p>
              {jiraLabel ? <p className="text-xs text-muted-foreground">Jira：{jiraLabel}</p> : null}
            </div>
            <div className="release-plan-sprint-assoc-actions">
              <button type="button" className="btn-ghost text-xs" onClick={() => setModalOpen(true)}>
                更换
              </button>
              <button type="button" className="btn-ghost text-xs text-red-600" onClick={() => onChange("")}>
                取消关联
              </button>
            </div>
          </div>
        ) : null}
        {modal}
      </>
    );
  }

  return (
    <>
      <section className="release-plan-form-section">
        <div className="release-plan-sprint-assoc-bar">
          <h3 className="release-plan-form-section-title">需求关联</h3>
          {linkButton}
        </div>
        <div className="release-plan-form-section-body">
          {selected ? (
            <div className="release-plan-sprint-assoc-card">
              <div className="stack-xs">
                <p className="text-sm font-medium">{selected.name}</p>
                {jiraLabel ? <p className="text-xs text-muted-foreground">Jira：{jiraLabel}</p> : null}
              </div>
              <div className="release-plan-sprint-assoc-actions">
                <button type="button" className="btn-ghost text-xs" onClick={() => setModalOpen(true)}>
                  更换
                </button>
                <button type="button" className="btn-ghost text-xs text-red-600" onClick={() => onChange("")}>
                  取消关联
                </button>
              </div>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">关联迭代后可从工作项生成检查项。</p>
          )}
        </div>
      </section>
      {modal}
    </>
  );
}

/** 仅渲染「关联冲刺」按钮与弹窗，供卡片标题栏使用 */
export function ReleasePlanSprintLinkAction(props: ReleasePlanSprintAssociationFieldProps) {
  return <ReleasePlanSprintAssociationField {...props} actionOnly />;
}
