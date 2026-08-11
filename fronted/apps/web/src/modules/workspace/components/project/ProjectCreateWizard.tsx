"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { product, secret, tenant, workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { CreateProjectCommand, ProjectVM } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { TemplateVM } from "@sparkops/contracts/product";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { CharCounter, FormRow } from "./project-form-shared";
import { ProjectCreateStep2 } from "./ProjectCreateStep2";
import {
  EMPTY_PROJECT_FORM,
  validateProjectName,
} from "./project-utils";

interface ProjectCreateWizardProps {
  workspaceId: string;
  open?: boolean;
  onClose: () => void;
  onSuccess?: (project: ProjectVM) => void;
}

function workspaceOptionLabel(ws: WorkspaceVM) {
  return ws.name !== ws.slug ? `${ws.name} (${ws.slug})` : ws.name;
}

const STEPS = ["填写项目基本信息", "填写项目配置信息", "选择项目模板"] as const;

export function ProjectCreateWizard({ workspaceId, open, onClose, onSuccess }: ProjectCreateWizardProps) {
  const router = useRouter();
  const [step, setStep] = useState(1);
  const [form, setForm] = useState<CreateProjectCommand>(EMPTY_PROJECT_FORM);
  const [workspaces, setWorkspaces] = useState<WorkspaceVM[]>([]);
  const [targetWorkspaceId, setTargetWorkspaceId] = useState(workspaceId);
  const [workspaceMode, setWorkspaceMode] = useState<"select" | "manual">("select");
  const [loadingWorkspaces, setLoadingWorkspaces] = useState(false);
  const [gitServiceType, setGitServiceType] = useState<"GitLab" | "GitHub">("GitLab");
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [templates, setTemplates] = useState<TemplateVM[]>([]);
  const [loadingSecrets, setLoadingSecrets] = useState(false);
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [displayNameManual, setDisplayNameManual] = useState(false);
  const [draftProject, setDraftProject] = useState<{ id: string; workspaceId: string } | null>(null);

  const gitSecrets = useMemo(
    () => secrets.filter((s) => s.service_type === gitServiceType),
    [secrets, gitServiceType],
  );

  useEffect(() => {
    if (open === false) return;
    setStep(1);
    setForm(EMPTY_PROJECT_FORM);
    setTargetWorkspaceId(workspaceId);
    setWorkspaceMode("select");
    setDisplayNameManual(false);
    setDraftProject(null);
    setError(null);
    void loadWorkspaces();
  }, [open, workspaceId]);

  useEffect(() => {
    const wsId = workspaceMode === "select" ? targetWorkspaceId : workspaceId;
    if (!wsId) return;
    void (async () => {
      setLoadingTemplates(true);
      try {
        setTemplates(await product.listTemplates(wsId));
      } catch {
        setTemplates([]);
      } finally {
        setLoadingTemplates(false);
      }
    })();
  }, [targetWorkspaceId, workspaceId, workspaceMode]);

  const activeWorkspaceId = workspaceMode === "select" ? targetWorkspaceId : workspaceId;

  async function loadWorkspaces() {
    setLoadingWorkspaces(true);
    try {
      const items = await tenant.listWorkspaces();
      setWorkspaces(items);
      const selected = items.find((ws) => ws.id === workspaceId) ?? items[0];
      if (selected) {
        setTargetWorkspaceId(selected.id);
        setForm((prev) => ({ ...prev, namespace: selected.slug }));
      }
    } catch {
      setWorkspaces([]);
    } finally {
      setLoadingWorkspaces(false);
    }
  }

  async function loadSecrets() {
    if (!activeWorkspaceId) return;
    setLoadingSecrets(true);
    try {
      setSecrets(await secret.listSecrets(activeWorkspaceId));
    } catch {
      setSecrets([]);
    } finally {
      setLoadingSecrets(false);
    }
  }

  function handleWorkspaceChange(nextWorkspaceId: string) {
    const selected = workspaces.find((ws) => ws.id === nextWorkspaceId);
    setTargetWorkspaceId(nextWorkspaceId);
    setForm((prev) => ({
      ...prev,
      namespace: selected?.slug ?? "",
      secret_id: "",
      template_id: "",
    }));
    setSecrets([]);
  }

  function toggleWorkspaceMode() {
    setWorkspaceMode((mode) => {
      const next = mode === "select" ? "manual" : "select";
      if (next === "select") {
        const selected = workspaces.find((ws) => ws.id === targetWorkspaceId);
        setForm((prev) => ({
          ...prev,
          namespace: selected?.slug ?? prev.namespace,
          secret_id: "",
          template_id: "",
        }));
      } else {
        setForm((prev) => ({ ...prev, secret_id: "", template_id: "" }));
      }
      setSecrets([]);
      return next;
    });
  }

  function updateForm(patch: Partial<CreateProjectCommand>) {
    setForm((prev) => ({ ...prev, ...patch }));
  }

  function handleNameChange(name: string) {
    setForm((prev) => ({
      ...prev,
      name,
      ...(displayNameManual ? {} : { display_name: name }),
    }));
  }

  function handleDisplayNameChange(display_name: string) {
    setDisplayNameManual(true);
    updateForm({ display_name });
  }

  function validateStep(current: number) {
    if (current === 1) {
      if (workspaceMode === "select") {
        if (!targetWorkspaceId) return "请选择业务空间";
      } else if (!form.namespace?.trim()) {
        return "请填写业务空间（命名空间）";
      }
      if (!validateProjectName(form.name)) return "项目英文名格式不正确（小写字母、数字、连字符）";
      if (!form.display_name?.trim()) return "请填写显示名称";
    }
    if (current === 2) {
      if (!form.language) return "请选择开发语言";
      if (!form.git_url?.trim()) return "请填写 Git 仓库地址";
      if (!form.git_ref_value?.trim()) return "请填写 Git 引用值";
    }
    return null;
  }

  function resolveWorkspaceContext() {
    const selectedWorkspace = workspaces.find((ws) => ws.id === targetWorkspaceId);
    const createInWorkspaceId = workspaceMode === "select" ? targetWorkspaceId : workspaceId;
    const namespace =
      workspaceMode === "select"
        ? selectedWorkspace?.slug ?? form.namespace?.trim()
        : form.namespace?.trim();
    return { createInWorkspaceId, namespace };
  }

  function buildProjectPayload() {
    const { namespace } = resolveWorkspaceContext();
    return {
      ...form,
      namespace,
      name: form.name.trim(),
      display_name: form.display_name?.trim(),
      description: form.description?.trim() ?? "",
      git_url: form.git_url?.trim(),
      git_ref_value: form.git_ref_value?.trim(),
      dockerfile_path: form.dockerfile_path?.trim() || "./Dockerfile",
      secret_id: form.secret_id || undefined,
      template_id: form.template_id || undefined,
    };
  }

  async function persistProject() {
    const payload = buildProjectPayload();
    const { createInWorkspaceId } = resolveWorkspaceContext();
    if (draftProject) {
      return workspace.updateProject(draftProject.workspaceId, draftProject.id, payload);
    }
    const project = await workspace.createProject(createInWorkspaceId, payload);
    setDraftProject({ id: project.id, workspaceId: createInWorkspaceId });
    return project;
  }

  async function handleNext() {
    const message = validateStep(step);
    if (message) {
      setError(message);
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await persistProject();
      setStep((s) => Math.min(3, s + 1));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败，请稍后重试");
    } finally {
      setSubmitting(false);
    }
  }

  function handlePrev() {
    setError(null);
    setStep((s) => Math.max(1, s - 1));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    for (let i = 1; i <= 2; i += 1) {
      const message = validateStep(i);
      if (message) {
        setError(message);
        setStep(i);
        return;
      }
    }
    setSubmitting(true);
    setError(null);
    try {
      const project = await persistProject();
      onSuccess?.(project);
      onClose();
      router.push(`/projects/${project.id}`);
      router.refresh();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建项目失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="project-wizard project-wizard-modal">
      <div className="project-wizard-hero">
        <h3>创建项目</h3>
        <p>请仔细核对每一项内容和参数。</p>
      </div>

      <div className="project-wizard-steps is-modal">
        {STEPS.map((title, index) => {
          const n = index + 1;
          const done = step > n;
          const active = step === n;
          return (
            <div key={title} className="project-wizard-step-wrap">
              <div className={`project-wizard-step${active ? " is-active" : ""}${done ? " is-done" : ""}`}>
                <div className="project-wizard-step-circle">{done ? "✓" : n}</div>
                <span className="project-wizard-step-label">{title}</span>
              </div>
              {n < STEPS.length ? <div className={`project-wizard-connector${done ? " is-done" : ""}`} /> : null}
            </div>
          );
        })}
      </div>

      <form className="project-wizard-form" onSubmit={handleSubmit}>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

        {step === 1 ? (
          <div className="project-form-section is-horizontal">
            <FormRow
              label="业务空间:"
              required
              hint={
                workspaceMode === "select"
                  ? loadingWorkspaces
                    ? "正在加载可访问的业务空间..."
                    : workspaces.length > 0
                      ? `共 ${workspaces.length} 个可访问业务空间，默认为当前工作区，可切换到手动输入`
                      : "暂无可访问的业务空间，可切换到手动输入"
                  : "手动输入命名空间名称，支持小写字母、数字和连字符(-)，不超过63个字符"
              }
            >
              <div className="project-namespace-group">
                {workspaceMode === "select" ? (
                  <select
                    className="input"
                    value={targetWorkspaceId}
                    disabled={loadingWorkspaces || workspaces.length === 0}
                    onFocus={() => {
                      if (workspaces.length === 0) void loadWorkspaces();
                    }}
                    onChange={(e) => handleWorkspaceChange(e.target.value)}
                  >
                    {loadingWorkspaces ? (
                      <option value={targetWorkspaceId}>正在加载业务空间...</option>
                    ) : workspaces.length === 0 ? (
                      <option value="">暂无业务空间</option>
                    ) : (
                      workspaces.map((ws) => (
                        <option key={ws.id} value={ws.id}>
                          {workspaceOptionLabel(ws)}
                        </option>
                      ))
                    )}
                  </select>
                ) : (
                  <input
                    className="input"
                    placeholder="请输入命名空间名称，如 default、production"
                    value={form.namespace ?? ""}
                    onChange={(e) => updateForm({ namespace: e.target.value })}
                    maxLength={63}
                  />
                )}
                <button
                  type="button"
                  className="project-icon-btn"
                  title={workspaceMode === "select" ? "切换到手动输入" : "切换到下拉选择"}
                  onClick={toggleWorkspaceMode}
                >
                  {workspaceMode === "select" ? "✎" : "☰"}
                </button>
              </div>
            </FormRow>

            <FormRow
              label="项目英文名:"
              required
              hint="仅支持英文小写字母数字 - 名称不能纯数字,不能有大写,不能有'_'"
            >
              <div className="project-input-counter-wrap">
                <input
                  className="input"
                  placeholder="请输入项目英文名"
                  value={form.name}
                  onChange={(e) => handleNameChange(e.target.value)}
                  maxLength={50}
                  required
                />
                <CharCounter value={form.name} max={50} />
              </div>
            </FormRow>

            <FormRow label="显示名称:" required hint="默认为项目英文名，可手动修改">
              <div className="project-input-counter-wrap">
                <input
                  className="input"
                  placeholder="请输入显示名称"
                  value={form.display_name ?? ""}
                  onChange={(e) => handleDisplayNameChange(e.target.value)}
                  maxLength={50}
                  required
                />
                <CharCounter value={form.display_name ?? ""} max={50} />
              </div>
            </FormRow>

            <FormRow label="项目描述:">
              <div className="project-textarea-wrap">
                <textarea
                  className="input min-h-24"
                  placeholder="请填写项目描述..."
                  value={form.description ?? ""}
                  onChange={(e) => updateForm({ description: e.target.value })}
                  maxLength={200}
                />
                <CharCounter value={form.description ?? ""} max={200} />
              </div>
            </FormRow>
          </div>
        ) : null}

        {step === 2 ? (
          <ProjectCreateStep2
            key={open === false ? "closed" : workspaceId}
            form={form}
            updateForm={updateForm}
            gitServiceType={gitServiceType}
            setGitServiceType={setGitServiceType}
            gitSecrets={gitSecrets}
            loadingSecrets={loadingSecrets}
            loadSecrets={() => void loadSecrets()}
          />
        ) : null}

        {step === 3 ? (
          <div className="project-form-section is-horizontal">
            <FormRow label="项目模板:" hint="可从模板库选择 Deployment / Service 等配置模板（可选）">
              <select
                className="input"
                value={form.template_id ?? ""}
                onChange={(e) => updateForm({ template_id: e.target.value })}
              >
                <option value="">{loadingTemplates ? "加载中…" : "不选择模板"}</option>
                {templates.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name} {t.kind ? `(${t.kind})` : ""}
                  </option>
                ))}
              </select>
            </FormRow>
            {form.template_id ? (
              <div className="project-template-preview">
                {templates
                  .filter((t) => t.id === form.template_id)
                  .map((t) => (
                    <div key={t.id}>
                      <strong>{t.name}</strong>
                      <p>{t.detail?.slice(0, 200) || "暂无模板说明"}</p>
                    </div>
                  ))}
              </div>
            ) : null}
          </div>
        ) : null}

        <div className="project-wizard-actions is-centered">
          {step > 1 ? (
            <button type="button" className="btn-secondary" onClick={handlePrev}>
              上一步
            </button>
          ) : null}
          {step < 3 ? (
            <button
              type="button"
              className="btn project-btn-primary"
              onClick={() => void handleNext()}
              disabled={submitting}
            >
              {submitting ? "保存中…" : "下一步"}
            </button>
          ) : (
            <button type="submit" className="btn project-btn-primary" disabled={submitting}>
              {submitting ? "创建中…" : "创建项目"}
            </button>
          )}
        </div>
      </form>

      <div className="project-wizard-tips">
        <strong>提示</strong>
        <p>请根据项目实际需求，设置合理的参数</p>
      </div>
    </div>
  );
}
