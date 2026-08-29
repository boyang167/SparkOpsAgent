"use client";

import { useMemo } from "react";
import type { CreateProjectCommand } from "@sparkops/contracts/workspace";
import type { TemplateVM } from "@sparkops/contracts/product";
import type { SecretVM } from "@sparkops/contracts/secret";
import { FormRow } from "./project-form-shared";
import { hasSelectedProjectTemplate } from "./project-utils";

interface ProjectCreateStep3Props {
  form: CreateProjectCommand;
  updateForm: (patch: Partial<CreateProjectCommand>) => void;
  templates: TemplateVM[];
  secrets: SecretVM[];
  loadingTemplates: boolean;
  loadingSecrets: boolean;
}

function kindMatch(kind: string | undefined, expected: string) {
  return (kind ?? "").toLowerCase() === expected.toLowerCase();
}

export function ProjectCreateStep3({
  form,
  updateForm,
  templates,
  secrets,
  loadingTemplates,
  loadingSecrets,
}: ProjectCreateStep3Props) {
  const resources = form.resources ?? {};

  const serviceTemplates = useMemo(
    () => templates.filter((t) => kindMatch(t.kind, "Service")),
    [templates],
  );
  const deploymentTemplates = useMemo(
    () => templates.filter((t) => kindMatch(t.kind, "Deployment")),
    [templates],
  );
  const jenkinsfileTemplates = useMemo(
    () =>
      templates.filter(
        (t) => kindMatch(t.kind, "jenkinsFile") || kindMatch(t.kind, "Jenkinsfile"),
      ),
    [templates],
  );

  const registryType = resources.registry_type ?? "Harbor";
  const registrySecrets = useMemo(() => {
    if (registryType === "Docker_Registry") {
      return secrets.filter((s) => s.service_type === "Docker" || s.service_type === "Docker_Registry");
    }
    return secrets.filter((s) => s.service_type === "Harbor");
  }, [secrets, registryType]);

  const sonarSecrets = useMemo(
    () => secrets.filter((s) => s.service_type === "SonarQube"),
    [secrets],
  );
  const jenkinsSecrets = useMemo(
    () => secrets.filter((s) => s.service_type === "Jenkins"),
    [secrets],
  );
  const kubernetesSecrets = useMemo(
    () => secrets.filter((s) => s.service_type === "Kubernetes"),
    [secrets],
  );

  function updateResources(patch: Partial<NonNullable<CreateProjectCommand["resources"]>>) {
    const next = { ...resources, ...patch };
    updateForm({
      resources: next,
      template_id: hasSelectedProjectTemplate(next)
        ? next.deployment_template_id ||
          next.service_template_id ||
          next.jenkinsfile_template_id ||
          form.template_id
        : "",
    });
  }

  return (
    <div className="project-form-section is-horizontal is-ref project-step3">
      <div className="project-step3-intro">
        <h3>创建项目</h3>
        <p>请仔细核对每一项内容和参数</p>
      </div>

      <FormRow label="Service 模板:" required hint="选择 Kubernetes Service 配置模板">
        <select
          className="input"
          value={resources.service_template_id ?? ""}
          onChange={(e) => updateResources({ service_template_id: e.target.value })}
        >
          <option value="">{loadingTemplates ? "加载中…" : "请选择 Service 模板"}</option>
          {serviceTemplates.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="Deployment 模板:" required hint="选择 Kubernetes Deployment 配置模板">
        <select
          className="input"
          value={resources.deployment_template_id ?? ""}
          onChange={(e) => updateResources({ deployment_template_id: e.target.value })}
        >
          <option value="">{loadingTemplates ? "加载中…" : "请选择 Deployment 模板"}</option>
          {deploymentTemplates.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="Jenkinsfile 模板:" required hint="选择 Jenkins CI/CD 流水线配置模板">
        <select
          className="input"
          value={resources.jenkinsfile_template_id ?? ""}
          onChange={(e) => updateResources({ jenkinsfile_template_id: e.target.value })}
        >
          <option value="">{loadingTemplates ? "加载中…" : "请选择 Jenkinsfile 模板"}</option>
          {jenkinsfileTemplates.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="镜像仓库类型:">
        <div className="project-radio-row">
          {(["Harbor", "Docker_Registry"] as const).map((type) => (
            <label key={type} className="project-radio">
              <input
                type="radio"
                name="registry_type"
                checked={registryType === type}
                onChange={() =>
                  updateResources({
                    registry_type: type,
                    registry_secret_id: "",
                  })
                }
              />
              {type === "Docker_Registry" ? "Docker_Registry" : "Harbor"}
            </label>
          ))}
        </div>
      </FormRow>

      <FormRow label="镜像仓库秘钥:" hint="选择用于推送镜像的仓库认证秘钥">
        <select
          className="input"
          value={resources.registry_secret_id ?? ""}
          onChange={(e) => updateResources({ registry_secret_id: e.target.value })}
        >
          <option value="">
            {loadingSecrets ? "加载中…" : "请选择镜像仓库秘钥 (可选)"}
          </option>
          {registrySecrets.map((s) => (
            <option key={s.id} value={s.id}>
              {s.display_name?.trim() || s.name}
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="默认集群 ID:" hint="部署时将优先选择此集群；最终可选范围以工作空间授权为准">
        <input
          className="input"
          aria-label="默认集群 ID"
          value={form.default_cluster_id ?? ""}
          onChange={(e) => updateForm({ default_cluster_id: e.target.value })}
          placeholder="请输入已授权的 cluster_id（可选）"
        />
      </FormRow>

      <FormRow label="Jenkins 密钥:" hint="选择用于执行构建流水线的 Jenkins 密钥">
        <select
          className="input"
          aria-label="Jenkins 密钥"
          value={form.jenkins_secret_id ?? ""}
          onChange={(e) => updateForm({ jenkins_secret_id: e.target.value })}
        >
          <option value="">{loadingSecrets ? "加载中…" : "请选择 Jenkins 密钥"}</option>
          {jenkinsSecrets.map((item) => (
            <option key={item.id} value={item.id}>
              {item.display_name?.trim() || item.name}
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="Kubernetes 密钥:" hint="选择用于部署到集群的 Kubernetes 密钥">
        <select
          className="input"
          aria-label="Kubernetes 密钥"
          value={form.kubernetes_secret_id ?? ""}
          onChange={(e) => updateForm({ kubernetes_secret_id: e.target.value })}
        >
          <option value="">{loadingSecrets ? "加载中…" : "请选择 Kubernetes 密钥"}</option>
          {kubernetesSecrets.map((item) => (
            <option key={item.id} value={item.id}>
              {item.display_name?.trim() || item.name}
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="Sonarqube 秘钥:" hint="选择用于代码质量检查的 Sonarqube 认证秘钥">
        <select
          className="input"
          value={resources.sonarqube_secret_id ?? ""}
          onChange={(e) => updateResources({ sonarqube_secret_id: e.target.value })}
        >
          <option value="">
            {loadingSecrets ? "加载中…" : "请选择 Sonarqube 秘钥 (可选)"}
          </option>
          {sonarSecrets.map((s) => (
            <option key={s.id} value={s.id}>
              {s.display_name?.trim() || s.name}
            </option>
          ))}
        </select>
      </FormRow>

      <FormRow label="是否开启代码静态扫描:" align="center">
        <div className="project-scan-switch-row">
          <button
            type="button"
            className={`project-switch${resources.enable_static_scan ? " is-on" : ""}`}
            aria-pressed={Boolean(resources.enable_static_scan)}
            onClick={() =>
              updateResources({ enable_static_scan: !resources.enable_static_scan })
            }
          >
            <span className="project-switch-thumb" />
          </button>
          <span className="project-scan-switch-copy">
            {resources.enable_static_scan ? "开启" : "关闭"}
            <span>开启后将在构建过程中进行代码静态扫描</span>
          </span>
        </div>
      </FormRow>

      <p className="project-step3-hint">
        <span className="project-hint-icon" aria-hidden>
          i
        </span>
        提示：至少选择一个模板，系统将根据选择的模板生成对应的配置文件
      </p>

      <div className="project-wizard-tips">
        <strong>提示</strong>
        <p>请根据项目实际需求，设置合理的参数</p>
      </div>
    </div>
  );
}
