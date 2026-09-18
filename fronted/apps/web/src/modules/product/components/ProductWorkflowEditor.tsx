"use client";

import { useMemo, useState } from "react";
import type { EnvironmentVM, ProductProjectVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Drawer } from "@/components/ui/Drawer";
import { Modal } from "@/components/ui/Modal";
import { WorkflowJobPickerModal } from "./WorkflowJobPickerModal";
import { BuildJobConfigDrawer } from "./BuildJobConfigDrawer";
import {
  defaultJob,
  emptyStage,
  jobTypeLabel,
  nextJobIndex,
  stageExecModeLabel,
  type EditorJob,
  type EditorStage,
} from "./workflow-editor-utils";

interface ProductWorkflowEditorProps {
  workspaceId: string;
  name: string;
  workflowKey: string;
  description: string;
  status: string;
  stages: EditorStage[];
  projects: ProductProjectVM[];
  environments: EnvironmentVM[];
  busy?: boolean;
  error?: string | null;
  onNameChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onStagesChange: (stages: EditorStage[]) => void;
  onSave: (activate: boolean) => void;
  onCancel?: () => void;
}

type Selection =
  | { kind: "stage"; stageIndex: number }
  | { kind: "job"; stageIndex: number; jobIndex: number }
  | null;

type SidePanel = "variables" | "triggers" | "notify" | "advanced" | null;
type ViewMode = "visual" | "yaml";

export function ProductWorkflowEditor({
  workspaceId,
  name,
  workflowKey,
  description,
  status,
  stages,
  projects,
  environments,
  busy,
  error,
  onNameChange,
  onDescriptionChange,
  onStagesChange,
  onSave,
  onCancel,
}: ProductWorkflowEditorProps) {
  const [selection, setSelection] = useState<Selection>(null);
  const [viewMode, setViewMode] = useState<ViewMode>("visual");
  const [editingTitle, setEditingTitle] = useState(false);
  const [sidePanel, setSidePanel] = useState<SidePanel>(null);
  const [jobPickerStage, setJobPickerStage] = useState<number | null>(null);
  const [execModeStage, setExecModeStage] = useState<number | null>(null);
  const [draftExecMode, setDraftExecMode] = useState<"auto" | "manual">("auto");
  const [editStage, setEditStage] = useState<number | null>(null);
  const [draftStageName, setDraftStageName] = useState("");
  const [draftParallel, setDraftParallel] = useState(false);
  const [yamlError, setYamlError] = useState<string | null>(null);
  const [yamlText, setYamlText] = useState("");

  const selectedJob = useMemo(() => {
    if (!selection || selection.kind !== "job") return null;
    return stages[selection.stageIndex]?.jobs[selection.jobIndex] ?? null;
  }, [selection, stages]);

  const yamlContent = useMemo(
    () => JSON.stringify({ name, key: workflowKey, description, status, stages }, null, 2),
    [name, workflowKey, description, status, stages],
  );

  function updateStage(stageIndex: number, patch: Partial<EditorStage>) {
    onStagesChange(stages.map((stage, i) => (i === stageIndex ? { ...stage, ...patch } : stage)));
  }

  function updateJob(stageIndex: number, jobIndex: number, patch: Partial<EditorJob>) {
    onStagesChange(
      stages.map((stage, si) => {
        if (si !== stageIndex) return stage;
        return {
          ...stage,
          jobs: stage.jobs.map((job, ji) => (ji === jobIndex ? { ...job, ...patch } : job)),
        };
      }),
    );
  }

  function updateJobSpec(stageIndex: number, jobIndex: number, specPatch: Record<string, unknown>) {
    const job = stages[stageIndex]?.jobs[jobIndex];
    if (!job) return;
    updateJob(stageIndex, jobIndex, { spec: { ...(job.spec ?? {}), ...specPatch } });
  }

  function insertStage(atIndex: number) {
    const next = [...stages];
    next.splice(atIndex, 0, emptyStage(stages.length + 1));
    onStagesChange(next);
  }

  function openExecMode(stageIndex: number) {
    setDraftExecMode(stages[stageIndex]?.exec_mode === "manual" ? "manual" : "auto");
    setExecModeStage(stageIndex);
  }

  function openEditStage(stageIndex: number) {
    setDraftStageName(stages[stageIndex]?.name ?? "");
    setDraftParallel(Boolean(stages[stageIndex]?.parallel));
    setEditStage(stageIndex);
  }

  function removeStage(stageIndex: number) {
    if (stages.length <= 1) return;
    onStagesChange(stages.filter((_, i) => i !== stageIndex));
    setSelection(null);
  }

  function addJob(stageIndex: number, type: string) {
    const job = defaultJob(type, nextJobIndex(stages));
    onStagesChange(
      stages.map((stage, i) => (i === stageIndex ? { ...stage, jobs: [...stage.jobs, job] } : stage)),
    );
    setSelection({ kind: "job", stageIndex, jobIndex: stages[stageIndex].jobs.length });
    setJobPickerStage(null);
  }

  function removeJob(stageIndex: number, jobIndex: number) {
    onStagesChange(
      stages.map((stage, si) =>
        si === stageIndex ? { ...stage, jobs: stage.jobs.filter((_, ji) => ji !== jobIndex) } : stage,
      ),
    );
    setSelection(null);
  }

  function switchToYaml() {
    setYamlText(yamlContent);
    setYamlError(null);
    setViewMode("yaml");
  }

  function applyYaml() {
    try {
      const parsed = JSON.parse(yamlText) as {
        name?: string;
        description?: string;
        stages?: EditorStage[];
      };
      if (parsed.name) onNameChange(parsed.name);
      if (typeof parsed.description === "string") onDescriptionChange(parsed.description);
      if (Array.isArray(parsed.stages) && parsed.stages.length > 0) {
        onStagesChange(parsed.stages);
      }
      setYamlError(null);
      setViewMode("visual");
    } catch {
      setYamlError("YAML/JSON 格式无效，请检查后重试");
    }
  }

  return (
    <div className="pwf-studio">
      <header className="pwf-studio-top">
        <div className="pwf-studio-identity">
          <div className="pwf-studio-title-row">
            {editingTitle ? (
              <input
                className="pwf-studio-title-input"
                value={name}
                autoFocus
                onChange={(e) => onNameChange(e.target.value)}
                onBlur={() => setEditingTitle(false)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") setEditingTitle(false);
                }}
              />
            ) : (
              <h1 className="pwf-studio-title">{name || "未命名工作流"}</h1>
            )}
            <button
              type="button"
              className="pwf-icon-btn"
              aria-label="编辑名称"
              onClick={() => setEditingTitle(true)}
            >
              ✎
            </button>
          </div>
          <p className="pwf-studio-sub">
            <span className="pwf-studio-key">{workflowKey || "-"}</span>
            <span className="pwf-studio-sep">·</span>
            <input
              className="pwf-studio-desc-input"
              value={description}
              onChange={(e) => onDescriptionChange(e.target.value)}
              placeholder="添加工作流描述"
            />
          </p>
        </div>

        <div className="pwf-view-toggle" role="tablist" aria-label="编辑模式">
          <button
            type="button"
            role="tab"
            aria-selected={viewMode === "visual"}
            className={viewMode === "visual" ? "is-active" : undefined}
            onClick={() => setViewMode("visual")}
          >
            界面化
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={viewMode === "yaml"}
            className={viewMode === "yaml" ? "is-active" : undefined}
            onClick={switchToYaml}
          >
            YAML
          </button>
        </div>

        <div className="pwf-studio-actions">
          <span className={`pwf-status-pill${status === "active" ? " is-active" : ""}`}>
            {status === "active" ? "已启用" : "草稿"}
          </span>
          <button type="button" className="btn btn-ghost text-sm" disabled title="后续支持">
            保存为模板
          </button>
          {onCancel ? (
            <button type="button" className="btn btn-ghost text-sm" onClick={onCancel} disabled={busy}>
              取消
            </button>
          ) : null}
          <button type="button" className="btn btn-secondary text-sm" onClick={() => onSave(false)} disabled={busy}>
            保存
          </button>
          <button type="button" className="btn btn-primary text-sm" onClick={() => onSave(true)} disabled={busy}>
            保存并启用
          </button>
        </div>
      </header>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {yamlError ? <InlineAlert tone="error">{yamlError}</InlineAlert> : null}

      {viewMode === "yaml" ? (
        <div className="pwf-yaml-panel stack-sm">
          <textarea
            className="pwf-yaml-editor"
            value={yamlText}
            onChange={(e) => setYamlText(e.target.value)}
            spellCheck={false}
          />
          <div className="pwf-yaml-actions">
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setViewMode("visual")}>
              返回界面化
            </button>
            <button type="button" className="btn btn-primary text-sm" onClick={applyYaml}>
              应用 YAML
            </button>
          </div>
        </div>
      ) : (
        <div className="pwf-studio-main">
          <div className="pwf-flow-canvas">
            <div className="pwf-flow-track">
              <div className="pwf-endpoint">
                <span className="pwf-endpoint-dot">开始</span>
              </div>

              {stages.map((stage, stageIndex) => (
                <div key={`stage-wrap-${stageIndex}`} className="pwf-flow-segment">
                  <div className="pwf-connector">
                    <span className="pwf-connector-line" />
                    <button
                      type="button"
                      className="pwf-connector-add"
                      title="在此插入阶段"
                      onClick={() => insertStage(stageIndex)}
                    >
                      +
                    </button>
                    <button
                      type="button"
                      className="pwf-connector-mode"
                      onClick={() => openExecMode(stageIndex)}
                    >
                      {stageExecModeLabel(stage.exec_mode)}
                    </button>
                    <span className="pwf-connector-line" />
                  </div>

                  <article
                    className={`pwf-stage-card${
                      selection?.stageIndex === stageIndex ? " is-active" : ""
                    }`}
                  >
                    <header className="pwf-stage-card-head">
                      <input
                        className="pwf-stage-name"
                        value={stage.name}
                        onChange={(e) => updateStage(stageIndex, { name: e.target.value })}
                      />
                      <button
                        type="button"
                        className="pwf-icon-btn"
                        title="编辑阶段"
                        onClick={() => openEditStage(stageIndex)}
                      >
                        ⚙
                      </button>
                    </header>

                    <div className="pwf-stage-jobs">
                      {stage.jobs.map((job, jobIndex) => (
                        <button
                          key={`${job.name}-${jobIndex}`}
                          type="button"
                          className={`pwf-task-row${
                            selection?.kind === "job" &&
                            selection.stageIndex === stageIndex &&
                            selection.jobIndex === jobIndex
                              ? " is-active"
                              : ""
                          }`}
                          onClick={() => setSelection({ kind: "job", stageIndex, jobIndex })}
                        >
                          <span className="pwf-task-dot" aria-hidden />
                          <span className="pwf-task-name">{job.name}</span>
                          <span className="pwf-task-type">{jobTypeLabel(job.type)}</span>
                          <span
                            className="pwf-task-remove"
                            role="button"
                            tabIndex={0}
                            onClick={(e) => {
                              e.stopPropagation();
                              removeJob(stageIndex, jobIndex);
                            }}
                            onKeyDown={(e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                e.stopPropagation();
                                removeJob(stageIndex, jobIndex);
                              }
                            }}
                          >
                            ×
                          </span>
                        </button>
                      ))}
                    </div>

                    <div className="pwf-stage-footer">
                      <button
                        type="button"
                        className="pwf-add-task"
                        onClick={() => setJobPickerStage(stageIndex)}
                      >
                        + 任务
                      </button>
                      {stages.length > 1 ? (
                        <button
                          type="button"
                          className="btn btn-ghost text-xs"
                          onClick={() => removeStage(stageIndex)}
                        >
                          删除阶段
                        </button>
                      ) : null}
                    </div>
                  </article>
                </div>
              ))}

              <div className="pwf-flow-segment">
                <div className="pwf-connector">
                  <span className="pwf-connector-line" />
                  <button
                    type="button"
                    className="pwf-connector-add is-stage"
                    title="添加阶段"
                    onClick={() => insertStage(stages.length)}
                  >
                    +
                  </button>
                  <span className="pwf-connector-label">+ 阶段</span>
                  <span className="pwf-connector-line" />
                </div>
                <div className="pwf-endpoint">
                  <span className="pwf-endpoint-dot">结束</span>
                </div>
              </div>
            </div>
          </div>

          <aside className="pwf-rail" aria-label="工作流配置">
            {(
              [
                { id: "variables", label: "变量" },
                { id: "triggers", label: "触发器" },
                { id: "notify", label: "通知" },
                { id: "advanced", label: "高级配置" },
              ] as const
            ).map((item) => (
              <button
                key={item.id}
                type="button"
                className={`pwf-rail-item${sidePanel === item.id ? " is-active" : ""}`}
                onClick={() => setSidePanel(sidePanel === item.id ? null : item.id)}
              >
                <span className="pwf-rail-icon" aria-hidden>
                  {item.id === "variables" ? "Var" : item.id === "triggers" ? "⏱" : item.id === "notify" ? "✉" : "⚙"}
                </span>
                <span>{item.label}</span>
              </button>
            ))}
          </aside>
        </div>
      )}

      <WorkflowJobPickerModal
        open={jobPickerStage !== null}
        onClose={() => setJobPickerStage(null)}
        onPick={(type) => {
          if (jobPickerStage === null) return;
          addJob(jobPickerStage, type);
        }}
      />

      <Modal
        open={editStage !== null}
        onClose={() => setEditStage(null)}
        title="编辑阶段"
        footer={
          <div className="modal-footer-actions">
            <button
              type="button"
              className="btn btn-primary text-sm"
              disabled={!draftStageName.trim()}
              onClick={() => {
                if (editStage === null || !draftStageName.trim()) return;
                updateStage(editStage, { name: draftStageName.trim(), parallel: draftParallel });
                setEditStage(null);
              }}
            >
              确定
            </button>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setEditStage(null)}>
              取消
            </button>
          </div>
        }
      >
        <div className="pwf-stage-edit">
          <div className="pwf-stage-edit-row">
            <span className="pwf-stage-edit-label">阶段名称</span>
            <div className="pwf-stage-edit-control">
              <span className="pwf-required" aria-hidden>
                *
              </span>
              <input
                className="input"
                value={draftStageName}
                onChange={(e) => setDraftStageName(e.target.value)}
                aria-label="阶段名称"
              />
            </div>
          </div>
          <div className="pwf-stage-edit-row">
            <span className="pwf-stage-edit-label">并发执行</span>
            <button
              type="button"
              className={`menu-switch${draftParallel ? " menu-switch-on" : ""}`}
              aria-pressed={draftParallel}
              aria-label="并发执行"
              onClick={() => setDraftParallel((value) => !value)}
            >
              <span className="menu-switch-thumb" />
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        open={execModeStage !== null}
        onClose={() => setExecModeStage(null)}
        title="阶段执行方式"
        footer={
          <>
            <button
              type="button"
              className="btn btn-primary text-sm"
              onClick={() => {
                if (execModeStage === null) return;
                updateStage(execModeStage, { exec_mode: draftExecMode });
                setExecModeStage(null);
              }}
            >
              确定
            </button>
            <button type="button" className="btn btn-secondary text-sm" onClick={() => setExecModeStage(null)}>
              取消
            </button>
          </>
        }
      >
        <div className="pwf-exec-mode">
          <span className="pwf-exec-mode-label">执行方式</span>
          <div className="pwf-exec-mode-options" role="radiogroup" aria-label="执行方式">
            <label className="pwf-radio">
              <input
                type="radio"
                name="stage-exec-mode"
                checked={draftExecMode === "auto"}
                onChange={() => setDraftExecMode("auto")}
              />
              自动执行
            </label>
            <label className="pwf-radio">
              <input
                type="radio"
                name="stage-exec-mode"
                checked={draftExecMode === "manual"}
                onChange={() => setDraftExecMode("manual")}
              />
              手动执行
            </label>
          </div>
        </div>
      </Modal>

      <BuildJobConfigDrawer
        open={selection?.kind === "job" && selectedJob?.type === "sparkops-build"}
        workspaceId={workspaceId}
        jobName={selectedJob?.name ?? ""}
        spec={selectedJob?.spec}
        projects={projects}
        onClose={() => setSelection(null)}
        onConfirm={(nextName, nextSpec) => {
          if (!selection || selection.kind !== "job") return;
          updateJob(selection.stageIndex, selection.jobIndex, { name: nextName, spec: nextSpec });
          setSelection(null);
        }}
      />

      <Drawer
        open={selection?.kind === "job" && selectedJob?.type !== "sparkops-build"}
        onClose={() => setSelection(null)}
        title={selectedJob ? `任务 · ${selectedJob.name}` : "属性"}
        description={selectedJob ? jobTypeLabel(selectedJob.type) : "配置任务参数"}
      >
        {selection?.kind === "job" && selectedJob && selectedJob.type !== "sparkops-build" ? (
          <div className="stack-sm">
            <label className="field">
              <span className="text-xs text-muted">任务名称</span>
              <input
                className="input"
                value={selectedJob.name}
                onChange={(e) => updateJob(selection.stageIndex, selection.jobIndex, { name: e.target.value })}
              />
            </label>

            {selectedJob.type === "sparkops-deploy" ? (
              <>
                <label className="field">
                  <span className="text-xs text-muted">环境</span>
                  <select
                    className="input"
                    value={String(selectedJob.spec?.environment_id ?? "")}
                    onChange={(e) =>
                      updateJobSpec(selection.stageIndex, selection.jobIndex, { environment_id: e.target.value })
                    }
                  >
                    <option value="">请选择环境</option>
                    {environments.map((env) => (
                      <option key={env.id} value={env.id}>
                        {env.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="field">
                  <span className="text-xs text-muted">镜像来源 Job</span>
                  <input
                    className="input"
                    value={String(selectedJob.spec?.job_name ?? "")}
                    onChange={(e) =>
                      updateJobSpec(selection.stageIndex, selection.jobIndex, {
                        job_name: e.target.value,
                        source: "fromjob",
                      })
                    }
                    placeholder="上游构建任务名"
                  />
                </label>
              </>
            ) : null}

            {selectedJob.type === "freestyle" ? (
              <label className="field">
                <span className="text-xs text-muted">脚本</span>
                <textarea
                  className="input"
                  rows={8}
                  value={String(selectedJob.spec?.script ?? "")}
                  onChange={(e) =>
                    updateJobSpec(selection.stageIndex, selection.jobIndex, { script: e.target.value })
                  }
                />
              </label>
            ) : null}

            {selectedJob.type === "notify" ? (
              <>
                <label className="field">
                  <span className="text-xs text-muted">渠道</span>
                  <input
                    className="input"
                    value={String(selectedJob.spec?.channel ?? "in-app")}
                    onChange={(e) =>
                      updateJobSpec(selection.stageIndex, selection.jobIndex, { channel: e.target.value })
                    }
                  />
                </label>
                <label className="field">
                  <span className="text-xs text-muted">消息</span>
                  <textarea
                    className="input"
                    rows={4}
                    value={String(selectedJob.spec?.message ?? "")}
                    onChange={(e) =>
                      updateJobSpec(selection.stageIndex, selection.jobIndex, { message: e.target.value })
                    }
                  />
                </label>
              </>
            ) : null}

            {selectedJob.type === "sparkops-approval" ? (
              <label className="field">
                <span className="text-xs text-muted">超时（分钟）</span>
                <input
                  className="input"
                  type="number"
                  min={1}
                  value={Number(selectedJob.spec?.timeout_minutes ?? 60)}
                  onChange={(e) =>
                    updateJobSpec(selection.stageIndex, selection.jobIndex, {
                      timeout_minutes: Number(e.target.value) || 60,
                    })
                  }
                />
              </label>
            ) : null}
          </div>
        ) : null}
      </Drawer>

      <Drawer
        open={Boolean(sidePanel)}
        onClose={() => setSidePanel(null)}
        title={
          sidePanel === "variables"
            ? "变量"
            : sidePanel === "triggers"
              ? "触发器"
              : sidePanel === "notify"
                ? "通知"
                : "高级配置"
        }
        description="全局工作流配置（部分能力后续开放）"
      >
        <div className="stack-sm text-sm text-muted">
          {sidePanel === "variables" ? (
            <p>可在任务 Spec 中使用运行时覆盖；全局变量面板即将接入。</p>
          ) : null}
          {sidePanel === "triggers" ? <p>支持手动执行；Webhook / Cron 触发后续版本提供。</p> : null}
          {sidePanel === "notify" ? (
            <p>可在编排中添加「通知」任务；工作流级通知策略后续开放。</p>
          ) : null}
          {sidePanel === "advanced" ? (
            <p>并发限制、调试断点等高级选项对齐 Zadig Workflow V4，后续迭代。</p>
          ) : null}
        </div>
      </Drawer>
    </div>
  );
}
