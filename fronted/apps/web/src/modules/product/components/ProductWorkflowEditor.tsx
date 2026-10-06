"use client";

import { useMemo, useState } from "react";
import type { EnvironmentVM, ProductProjectVM, WorkflowAdvancedVM, WorkflowNotificationVM, WorkflowTriggerVM, WorkflowVariableVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Drawer } from "@/components/ui/Drawer";
import { Modal } from "@/components/ui/Modal";
import { WorkflowJobPickerModal } from "./WorkflowJobPickerModal";
import { ApprovalJobConfigDrawer } from "./ApprovalJobConfigDrawer";
import { BuildJobConfigDrawer } from "./BuildJobConfigDrawer";
import { CreateVersionJobConfigDrawer } from "./CreateVersionJobConfigDrawer";
import { DeployJobConfigDrawer } from "./DeployJobConfigDrawer";
import { BuildNotifyList } from "./BuildNotifyList";
import {
  emptyStage,
  jobTypeLabel,
  prepareNewJob,
  upstreamBuildJobs,
  upstreamVersionSourceJobs,
  type EditorJob,
  type EditorStage,
} from "./workflow-editor-utils";
import { fromNotifyItems, toNotifyItems } from "./workflow-notify";
import { WorkflowEditorFlowCanvas } from "./WorkflowEditorFlowCanvas";
import { WorkflowAdvancedPanel, WorkflowTriggersPanel, WorkflowVariablesPanel } from "./WorkflowSidePanels";

interface ProductWorkflowEditorProps {
  workspaceId: string;
  productId: string;
  workflowId: string;
  name: string;
  workflowKey: string;
  description: string;
  status: string;
  stages: EditorStage[];
  notifications: WorkflowNotificationVM[];
  variables: WorkflowVariableVM[];
  triggers: WorkflowTriggerVM[];
  advanced: WorkflowAdvancedVM;
  projects: ProductProjectVM[];
  environments: EnvironmentVM[];
  busy?: boolean;
  error?: string | null;
  onNameChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onStagesChange: (stages: EditorStage[]) => void;
  onNotificationsChange: (notifications: WorkflowNotificationVM[]) => void;
  onVariablesChange: (variables: WorkflowVariableVM[]) => void;
  onTriggersChange: (triggers: WorkflowTriggerVM[]) => void;
  onAdvancedChange: (advanced: WorkflowAdvancedVM) => void;
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
  productId,
  workflowId,
  name,
  workflowKey,
  description,
  status,
  stages,
  notifications,
  variables,
  triggers,
  advanced,
  projects,
  environments,
  busy,
  error,
  onNameChange,
  onDescriptionChange,
  onStagesChange,
  onNotificationsChange,
  onVariablesChange,
  onTriggersChange,
  onAdvancedChange,
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

  const sourceBuildJobs = useMemo(() => {
    if (!selection || selection.kind !== "job") return [];
    if (selectedJob?.type !== "sparkops-deploy" && selectedJob?.type !== "sparkops-create-version") return [];
    return upstreamBuildJobs(stages, selection.stageIndex, selection.jobIndex);
  }, [selection, selectedJob, stages]);

  const sourceVersionJobs = useMemo(() => {
    if (!selection || selection.kind !== "job") return [];
    if (
      selectedJob?.type !== "sparkops-deploy" &&
      selectedJob?.type !== "sparkops-deploy-version"
    ) {
      return [];
    }
    return upstreamVersionSourceJobs(stages, selection.stageIndex, selection.jobIndex);
  }, [selection, selectedJob, stages]);

  const yamlContent = useMemo(
    () => JSON.stringify({ name, key: workflowKey, description, status, stages, notifications, variables, triggers, advanced }, null, 2),
    [name, workflowKey, description, status, stages, notifications, variables, triggers, advanced],
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
    const jobIndex = stages[stageIndex]?.jobs.length ?? 0;
    const job = prepareNewJob(type, stages, stageIndex, jobIndex);
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
        notifications?: WorkflowNotificationVM[];
        variables?: WorkflowVariableVM[];
        triggers?: WorkflowTriggerVM[];
        advanced?: WorkflowAdvancedVM;
      };
      if (parsed.name) onNameChange(parsed.name);
      if (typeof parsed.description === "string") onDescriptionChange(parsed.description);
      if (Array.isArray(parsed.stages) && parsed.stages.length > 0) {
        onStagesChange(parsed.stages);
      }
      if (Array.isArray(parsed.notifications)) {
        onNotificationsChange(parsed.notifications);
      }
      if (Array.isArray(parsed.variables)) onVariablesChange(parsed.variables);
      if (Array.isArray(parsed.triggers)) onTriggersChange(parsed.triggers);
      if (parsed.advanced) onAdvancedChange(parsed.advanced);
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
          <WorkflowEditorFlowCanvas
            stages={stages}
            selection={selection}
            onStagesChange={onStagesChange}
            onSelectionChange={setSelection}
            onInsertStage={insertStage}
            onOpenExecMode={openExecMode}
            onOpenEditStage={openEditStage}
            onRemoveStage={removeStage}
            onAddJob={(stageIndex) => setJobPickerStage(stageIndex)}
            onRemoveJob={removeJob}
            onUpdateStageName={(stageIndex, name) => updateStage(stageIndex, { name })}
          />

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

      <DeployJobConfigDrawer
        open={
          selection?.kind === "job" &&
          (selectedJob?.type === "sparkops-deploy" || selectedJob?.type === "sparkops-deploy-version")
        }
        workspaceId={workspaceId}
        jobName={selectedJob?.name ?? ""}
        jobType={selectedJob?.type}
        spec={selectedJob?.spec}
        sourceJobs={sourceBuildJobs}
        sourceVersionJobs={sourceVersionJobs}
        environments={environments}
        projects={projects}
        onClose={() => setSelection(null)}
        onConfirm={(nextName, nextSpec) => {
          if (!selection || selection.kind !== "job") return;
          updateJob(selection.stageIndex, selection.jobIndex, {
            name: nextName,
            type: "sparkops-deploy",
            spec: nextSpec,
          });
          setSelection(null);
        }}
      />

      <CreateVersionJobConfigDrawer
        open={selection?.kind === "job" && selectedJob?.type === "sparkops-create-version"}
        jobName={selectedJob?.name ?? ""}
        spec={selectedJob?.spec}
        sourceJobs={sourceBuildJobs}
        onClose={() => setSelection(null)}
        onConfirm={(nextName, nextSpec) => {
          if (!selection || selection.kind !== "job") return;
          updateJob(selection.stageIndex, selection.jobIndex, { name: nextName, spec: nextSpec });
          setSelection(null);
        }}
      />

      <ApprovalJobConfigDrawer
        open={selection?.kind === "job" && selectedJob?.type === "sparkops-approval"}
        workspaceId={workspaceId}
        jobName={selectedJob?.name ?? ""}
        spec={selectedJob?.spec}
        onClose={() => setSelection(null)}
        onConfirm={(nextName, nextSpec) => {
          if (!selection || selection.kind !== "job") return;
          updateJob(selection.stageIndex, selection.jobIndex, { name: nextName, spec: nextSpec });
          setSelection(null);
        }}
      />

      <Drawer
        open={
          selection?.kind === "job" &&
          selectedJob?.type !== "sparkops-build" &&
          selectedJob?.type !== "sparkops-deploy" &&
          selectedJob?.type !== "sparkops-approval"
        }
        onClose={() => setSelection(null)}
        title={selectedJob ? `任务 · ${selectedJob.name}` : "属性"}
        description={selectedJob ? jobTypeLabel(selectedJob.type) : "配置任务参数"}
      >
        {selection?.kind === "job" &&
        selectedJob &&
        selectedJob.type !== "sparkops-build" &&
        selectedJob.type !== "sparkops-deploy" &&
        selectedJob.type !== "sparkops-approval" ? (
          <div className="stack-sm">
            <label className="field">
              <span className="text-xs text-muted">任务名称</span>
              <input
                className="input"
                value={selectedJob.name}
                onChange={(e) => updateJob(selection.stageIndex, selection.jobIndex, { name: e.target.value })}
              />
            </label>

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
        description={
          sidePanel === "variables"
            ? "自定义变量与系统变量"
            : sidePanel === "triggers"
              ? "Git、Webhook 与定时触发"
              : sidePanel === "notify"
                ? "工作流运行时按事件发送通知"
                : "并发与超时"
        }
        panelClassName={sidePanel === "advanced" ? undefined : "drawer-panel-wide"}
      >
        {sidePanel === "variables" ? <WorkflowVariablesPanel variables={variables} onChange={onVariablesChange} /> : null}
        {sidePanel === "triggers" ? (
          <WorkflowTriggersPanel
            productId={productId}
            workflowId={workflowId}
            triggers={triggers}
            projects={projects}
            onChange={onTriggersChange}
          />
        ) : null}
        {sidePanel === "notify" ? (
          <BuildNotifyList
            items={toNotifyItems(notifications)}
            onChange={(items) => onNotificationsChange(fromNotifyItems(items))}
          />
        ) : null}
        {sidePanel === "advanced" ? <WorkflowAdvancedPanel advanced={advanced} onChange={onAdvancedChange} /> : null}
      </Drawer>
    </div>
  );
}
