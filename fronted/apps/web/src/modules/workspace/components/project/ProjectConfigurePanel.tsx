"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { product, secret, workspace } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { CreateProjectCommand, ProjectVM } from "@sparkops/contracts/workspace";
import type { SecretVM } from "@sparkops/contracts/secret";
import type { TemplateVM } from "@sparkops/contracts/product";
import { Modal } from "@/components/ui/Modal";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { ProjectCreateStep2 } from "./ProjectCreateStep2";
import { ProjectCreateStep3 } from "./ProjectCreateStep3";
import {
  buildProjectUpdatePayload,
  EMPTY_PROJECT_FORM,
  hasSelectedProjectTemplate,
  resolvePrimaryTemplateId,
} from "./project-utils";
import { isProjectConfigComplete } from "./project-progress";

interface ProjectConfigurePanelProps {
  open: boolean;
  workspaceId: string;
  project: ProjectVM;
  onClose: () => void;
  onSaved: () => void;
  onProjectUpdated?: (project: ProjectVM) => void;
}

const CONFIG_STEPS = ["填写项目基本信息", "填写项目配置信息", "选择项目模板"] as const;

function projectToForm(project: ProjectVM): CreateProjectCommand {
  const resources = {
    ...EMPTY_PROJECT_FORM.resources,
    ...project.resources,
    ports: project.resources?.ports ?? [],
    command: project.resources?.command ?? "",
    args: project.resources?.args ?? "",
    project_config: project.resources?.project_config ?? "",
    service_template_id: project.resources?.service_template_id ?? "",
    deployment_template_id: project.resources?.deployment_template_id ?? "",
    jenkinsfile_template_id: project.resources?.jenkinsfile_template_id ?? "",
    registry_type: project.resources?.registry_type ?? "Harbor",
    registry_secret_id: project.resources?.registry_secret_id ?? "",
    sonarqube_secret_id: project.resources?.sonarqube_secret_id ?? "",
    enable_static_scan: project.resources?.enable_static_scan ?? false,
  };

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
    git_ref_value: (() => {
      const value = project.git_ref_value?.trim() ?? "";
      if (value === "main" && (project.git_ref_type ?? "branch") === "branch" && !isProjectConfigComplete(project)) {
        return "";
      }
      return value;
    })(),
    dockerfile_path: project.dockerfile_path || "./Dockerfile",
    template_id: project.template_id || resolvePrimaryTemplateId(resources),
    resources,
  };
}

export function ProjectConfigurePanel({
  open,
  workspaceId,
  project,
  onClose,
  onSaved,
  onProjectUpdated,
}: ProjectConfigurePanelProps) {
  const [step, setStep] = useState(2);
  const [form, setForm] = useState<CreateProjectCommand>(() => projectToForm(project));
  const openedRef = useRef(false);
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
    if (!open) {
      openedRef.current = false;
      return;
    }
    if (project.review_status === "pending") return;
    if (openedRef.current) return;
    openedRef.current = true;
    setStep(2);
    setForm(projectToForm(project));
    setError(null);
    void (async () => {
      setLoadingTemplates(true);
      setLoadingSecrets(true);
      try {
        const [templateList, secretList] = await Promise.all([
          product.listTemplates(workspaceId),
          secret.listSecrets(workspaceId),
        ]);
        setTemplates(templateList);
        setSecrets(secretList);
      } catch {
        setTemplates([]);
        setSecrets([]);
      } finally {
        setLoadingTemplates(false);
        setLoadingSecrets(false);
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

  function validateStep3() {
    if (!hasSelectedProjectTemplate(form.resources)) {
      return "请至少选择一个模板（Service / Deployment / Jenkinsfile）";
    }
    return null;
  }

  async function persistProject(preserveTemplateIds = false) {
    const payload = buildProjectUpdatePayload(form, project, { preserveTemplateIds });
    return workspace.updateProject(workspaceId, project.id, payload);
  }

  function notifyProjectUpdated(updated: ProjectVM) {
    onProjectUpdated?.(updated);
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
        const updated = await persistProject(true);
        notifyProjectUpdated(updated);
        setStep(3);
      } catch (err) {
        setError(err instanceof BffError ? err.message : "保存配置失败");
      } finally {
        setSubmitting(false);
      }
      return;
    }

    const message = validateStep3();
    if (message) {
      setError(message);
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      const updated = await persistProject();
      notifyProjectUpdated(updated);
      onSaved();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSubmitting(false);
    }
  }

  if (project.review_status === "pending") {
    return (
      <Modal open={open} onClose={onClose} title="项目审核中">
        <InlineAlert tone="error">项目审核中，无法修改配置。请等待审核通过或驳回后再操作。</InlineAlert>
      </Modal>
    );
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="继续配置项目"
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
              workspaceId={workspaceId}
              form={form}
              updateForm={updateForm}
              gitServiceType={gitServiceType}
              setGitServiceType={setGitServiceType}
              gitSecrets={gitSecrets}
              loadingSecrets={loadingSecrets}
              loadSecrets={() => void loadSecrets()}
            />
          ) : (
            <ProjectCreateStep3
              form={form}
              updateForm={updateForm}
              templates={templates}
              secrets={secrets}
              loadingTemplates={loadingTemplates}
              loadingSecrets={loadingSecrets}
            />
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
