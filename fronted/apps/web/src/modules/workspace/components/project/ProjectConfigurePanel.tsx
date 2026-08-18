"use client";

import { useEffect, useMemo, useState } from "react";
import { product, secret, workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { CreateProjectCommand, ProjectVM } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { TemplateVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { FormRow } from "./project-form-shared";
import { ProjectCreateStep2 } from "./ProjectCreateStep2";
import { EMPTY_PROJECT_FORM } from "./project-utils";

interface ProjectConfigurePanelProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  onClose: () => void;
  onSaved: () => void;
}

const CONFIG_STEPS = ["填写项目基本信息", "填写项目配置信息", "选择项目模板"] as const;

function projectToForm(project: ProjectVM): CreateProjectCommand {
  return {
    ...EMPTY_PROJECT_FORM,
    name: project.name,
    display_name: project.display_name ?? project.name,
    description: project.description ?? "",
    namespace: project.namespace ?? "",
    language: project.language || "go",
    git_url: project.git_url ?? "",
    secret_id: project.secret_id ?? "",
    git_ref_type: project.git_ref_type ?? "branch",
    git_ref_value: project.git_ref_value ?? "main",
    dockerfile_path: project.dockerfile_path || "./Dockerfile",
    template_id: project.template_id ?? "",
    resources: {
      ...EMPTY_PROJECT_FORM.resources,
      ...project.resources,
      ports: project.resources?.ports ?? [],
      command: project.resources?.command ?? "",
      args: project.resources?.args ?? "",
      project_config: project.resources?.project_config ?? "",
    },
  };
}

export function ProjectConfigurePanel({
  open,
  workspaceId,
  project,
  onClose,
  onSaved,
}: ProjectConfigurePanelProps) {
  const [step, setStep] = useState(2);
  const [form, setForm] = useState<CreateProjectCommand>(() => projectToForm(project));
  const [gitServiceType, setGitServiceType] = useState<"GitLab" | "GitHub">("GitLab");
  const [secrets, setSecrets] = useState<SecretVM[]>([]);
  const [templates, setTemplates] = useState<TemplateVM[]>([]);
  const [loadingSecrets, setLoadingSecrets] = useState(false);
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const gitSecrets = useMemo(
    () => secrets.filter((s) => s.service_type === gitServiceType),
    [secrets, gitServiceType],
  );

  useEffect(() => {
    if (!open) return;
    setStep(2);
    setForm(projectToForm(project));
    setError(null);
    void (async () => {
      setLoadingTemplates(true);
      try {
        setTemplates(await product.listTemplates(workspaceId));
      } catch {
        setTemplates([]);
      } finally {
        setLoadingTemplates(false);
      }
    })();
  }, [open, project, workspaceId]);

  function updateForm(patch: Partial<CreateProjectCommand>) {
    setForm((prev) => ({ ...prev, ...patch }));
  }

  async function loadSecrets() {
    setLoadingSecrets(true);
    try {
      setSecrets(await secret.listSecrets(workspaceId));
    } catch {
      setSecrets([]);
    } finally {
      setLoadingSecrets(false);
    }
  }

  function validateStep2() {
    if (!form.language) return "请选择开发语言";
    if (!form.git_url?.trim()) return "请填写 Git 仓库地址";
    if (!form.git_ref_value?.trim()) return "请填写 Git 引用值";
    return null;
  }

  async function persistProject() {
    await workspace.updateProject(workspaceId, project.id, {
      name: project.name,
      display_name: project.display_name ?? project.name,
      description: project.description ?? "",
      namespace: project.namespace,
      language: form.language,
      git_url: form.git_url?.trim(),
      secret_id: form.secret_id || undefined,
      git_ref_type: form.git_ref_type,
      git_ref_value: form.git_ref_value?.trim(),
      dockerfile_path: form.dockerfile_path?.trim() || "./Dockerfile",
      template_id: form.template_id || undefined,
      resources: form.resources,
    });
  }

  function handlePrev() {
    setError(null);
    if (step === 2) {
      onClose();
      return;
    }
    setStep(2);
  }

  async function handleNext() {
    if (step === 2) {
      const message = validateStep2();
      if (message) {
        setError(message);
        return;
      }
      setSubmitting(true);
      setError(null);
      try {
        await persistProject();
        setStep(3);
      } catch (err) {
        setError(err instanceof BffError ? err.message : "保存配置失败");
      } finally {
        setSubmitting(false);
      }
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      await persistProject();
      onSaved();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="继续配置"
      description="请仔细核对每一项内容和参数。"
      size="xl"
      panelClassName="project-create-modal"
    >
      <div className="project-configure-panel">
        <div className="project-wizard-steps is-modal is-ref">
          {CONFIG_STEPS.map((title, index) => {
            const n = index + 1;
            const done = step > n;
            const active = step === n;
            return (
              <div key={title} className="project-wizard-step-wrap">
                <div
                  className={`project-wizard-step${active ? " is-active" : ""}${done ? " is-done" : ""}`}
                >
                  <div className="project-wizard-step-circle">{done ? "✓" : n}</div>
                  <span className="project-wizard-step-label">{title}</span>
                </div>
                {index < CONFIG_STEPS.length - 1 ? (
                  <div className={`project-wizard-connector${done ? " is-done" : ""}`} />
                ) : null}
              </div>
            );
          })}
        </div>

        <div className="project-configure-scroll">
          {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

          {step === 2 ? (
            <ProjectCreateStep2
              form={form}
              updateForm={updateForm}
              gitServiceType={gitServiceType}
              setGitServiceType={setGitServiceType}
              gitSecrets={gitSecrets}
              loadingSecrets={loadingSecrets}
              loadSecrets={() => void loadSecrets()}
            />
          ) : (
            <div className="project-form-section is-horizontal is-ref">
              <FormRow
                label="项目模板:"
                hint="可从模板库选择 Deployment / Service 等配置模板（可选）"
              >
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
          )}
        </div>

        <div className="project-configure-footer">
          <button type="button" className="btn-secondary" onClick={handlePrev} disabled={submitting}>
            上一步
          </button>
          <button
            type="button"
            className="btn project-btn-primary"
            onClick={() => void handleNext()}
            disabled={submitting}
          >
            {submitting ? "保存中…" : step === 2 ? "下一步" : "完成"}
          </button>
        </div>
      </div>
    </Modal>
  );
}
